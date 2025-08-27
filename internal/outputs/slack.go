package outputs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"nomad-events/internal/nomad"
)

type SlackOutput struct {
	webhookURL string
	channel    string
	httpClient *http.Client
}

type SlackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Text        string            `json:"text"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

type SlackAttachment struct {
	Color  string       `json:"color,omitempty"`
	Fields []SlackField `json:"fields,omitempty"`
	Text   string       `json:"text,omitempty"`
}

type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

func NewSlackOutput(config map[string]interface{}) (*SlackOutput, error) {
	webhookURL, ok := config["webhook_url"].(string)
	if !ok || webhookURL == "" {
		return nil, fmt.Errorf("webhook_url is required for Slack output")
	}

	channel, _ := config["channel"].(string)

	return &SlackOutput{
		webhookURL: webhookURL,
		channel:    channel,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (o *SlackOutput) Send(event nomad.Event) error {
	message := o.formatEvent(event)

	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack message: %w", err)
	}

	resp, err := o.httpClient.Post(o.webhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to send Slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack API returned status %d", resp.StatusCode)
	}

	return nil
}

func (o *SlackOutput) formatEvent(event nomad.Event) SlackMessage {
	var color string
	switch event.Topic {
	case "Node":
		color = "good"
	case "Allocation":
		color = "warning"
	case "Job":
		color = "#439FE0"
	case "Evaluation":
		color = "#764FA5"
	default:
		color = "danger"
	}

	attachment := SlackAttachment{
		Color: color,
		Fields: []SlackField{
			{Title: "Topic", Value: event.Topic, Short: true},
			{Title: "Type", Value: event.Type, Short: true},
			{Title: "Index", Value: fmt.Sprintf("%d", event.Index), Short: true},
		},
	}

	if event.Key != "" {
		attachment.Fields = append(attachment.Fields, SlackField{
			Title: "Key", Value: event.Key, Short: true,
		})
	}

	if event.Namespace != "" {
		attachment.Fields = append(attachment.Fields, SlackField{
			Title: "Namespace", Value: event.Namespace, Short: true,
		})
	}

	if len(event.Payload) > 0 {
		var payload interface{}
		if err := json.Unmarshal(event.Payload, &payload); err == nil {
			payloadJSON, _ := json.MarshalIndent(payload, "", "  ")
			attachment.Text = fmt.Sprintf("```\n%s\n```", string(payloadJSON))
		}
	}

	message := SlackMessage{
		Text:        fmt.Sprintf("Nomad Event: %s/%s", event.Topic, event.Type),
		Attachments: []SlackAttachment{attachment},
	}

	if o.channel != "" {
		message.Channel = o.channel
	}

	return message
}