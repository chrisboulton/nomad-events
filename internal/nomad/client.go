package nomad

import (
	"context"
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