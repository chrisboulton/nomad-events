package nomad

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/nomad/api"
)

type Event struct {
	Topic     string      `json:"Topic"`
	Type      string      `json:"Type"`
	Key       string      `json:"Key"`
	Namespace string      `json:"Namespace"`
	Index     uint64      `json:"Index"`
	Payload   interface{} `json:"Payload"`
	Diff      interface{} `json:"Diff,omitempty"`
}

type EventStream struct {
	client       *api.Client
	lastIndex    uint64
	retryBackoff time.Duration
	maxRetries   int
}

func NewEventStream(address, token string) (*EventStream, error) {
	config := api.DefaultConfig()
	config.Address = address
	if token != "" {
		config.SecretID = token
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Nomad client: %w", err)
	}

	return &EventStream{
		client:       client,
		retryBackoff: time.Second,
		maxRetries:   10,
	}, nil
}

func (es *EventStream) Stream(ctx context.Context, eventChan chan<- Event) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := es.streamWithRetry(ctx, eventChan); err != nil {
			log.Printf("Stream ended with error: %v", err)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(es.retryBackoff):
				es.exponentialBackoff()
			}
		}
	}
}

func (es *EventStream) streamWithRetry(ctx context.Context, eventChan chan<- Event) error {
	retries := 0
	for retries < es.maxRetries {
		err := es.connectAndStream(ctx, eventChan)
		if err == nil {
			es.retryBackoff = time.Second
			return nil
		}

		retries++
		if retries >= es.maxRetries {
			return fmt.Errorf("max retries exceeded: %w", err)
		}

		log.Printf("Connection failed (attempt %d/%d): %v", retries, es.maxRetries, err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(es.retryBackoff):
			es.exponentialBackoff()
		}
	}
	return fmt.Errorf("max retries exceeded")
}

func (es *EventStream) connectAndStream(ctx context.Context, eventChan chan<- Event) error {
	topics := map[api.Topic][]string{
		"*": {"*"},
	}

	events, err := es.client.EventStream().Stream(ctx, topics, es.lastIndex, &api.QueryOptions{})
	if err != nil {
		return fmt.Errorf("failed to start event stream: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case eventWrapper := <-events:
			if eventWrapper == nil {
				continue
			}

			for _, event := range eventWrapper.Events {
				nomadEvent := Event{
					Topic:     string(event.Topic),
					Type:      event.Type,
					Key:       event.Key,
					Namespace: "",
					Index:     event.Index,
					Payload:   event.Payload,
				}

				// Fetch job diff for JobRegistered events
				if string(event.Topic) == "Job" && event.Type == "JobRegistered" {
					if jobData, ok := event.Payload["Job"].(map[string]interface{}); ok {
						if jobID, ok := jobData["ID"].(string); ok {
							// Only fetch diff if job version > 1 (has previous version to compare)
							if version, ok := jobData["Version"].(float64); ok && version > 1 {
								diff, err := es.fetchJobDiff(jobID)
								if err != nil {
									log.Printf("Failed to fetch job diff for %s: %v", jobID, err)
									// Continue without diff - don't fail the entire event
								} else {
									nomadEvent.Diff = diff
								}
							}
							// For version 1 jobs, nomadEvent.Diff remains nil (no previous version to compare)
						}
					}
				}

				es.lastIndex = event.Index

				select {
				case eventChan <- nomadEvent:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}
}

func (es *EventStream) exponentialBackoff() {
	es.retryBackoff = es.retryBackoff * 2
	if es.retryBackoff > time.Minute {
		es.retryBackoff = time.Minute
	}
}

func (es *EventStream) Client() *api.Client {
	return es.client
}

// fetchJobDiff fetches the diff between the current job version and the previous version
func (es *EventStream) fetchJobDiff(jobID string) (interface{}, error) {
	if es.client == nil {
		return nil, fmt.Errorf("nomad client not available")
	}

	// Get job versions with diffs enabled
	_, diffs, _, err := es.client.Jobs().Versions(jobID, true, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get job versions: %w", err)
	}

	if len(diffs) == 0 {
		return nil, fmt.Errorf("no job diffs available")
	}

	// Convert JobDiff struct to map for CEL compatibility
	// This ensures that CEL expressions like has(event.Diff.Type) work correctly
	diff := diffs[0]
	diffBytes, err := json.Marshal(diff)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job diff: %w", err)
	}
	
	var diffMap map[string]interface{}
	err = json.Unmarshal(diffBytes, &diffMap)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal job diff: %w", err)
	}

	return diffMap, nil
}
