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
	webhookURL     string
	channel        string
	httpClient     *http.Client
	blockConfigs   []BlockConfig
	templateEngine *SlackTemplateEngine
}

type SlackMessage struct {
	Channel     string      `json:"channel,omitempty"`
	Text        string      `json:"text,omitempty"`
	Blocks      interface{} `json:"blocks,omitempty"`
	Attachments interface{} `json:"attachments,omitempty"`
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

// mapBlockConfig maps a configuration map to a BlockConfig struct
func mapBlockConfig(config map[string]interface{}) BlockConfig {
	var block BlockConfig

	// Map fields using a simple lookup approach
	if v, ok := config["type"].(string); ok {
		block.Type = v
	}
	if v, ok := config["text"]; ok {
		block.Text = v
	}
	if v, ok := config["fields"].([]interface{}); ok {
		block.Fields = v
	}
	if v, ok := config["elements"].([]interface{}); ok {
		block.Elements = v
	}
	if v, ok := config["options"].([]interface{}); ok {
		block.Options = v
	}
	if v, ok := config["image_url"].(string); ok {
		block.ImageURL = v
	}
	if v, ok := config["alt_text"].(string); ok {
		block.AltText = v
	}
	if v, ok := config["title"]; ok {
		block.Title = v
	}
	if v, ok := config["label"]; ok {
		block.Label = v
	}
	if v, ok := config["hint"]; ok {
		block.Hint = v
	}
	if v, ok := config["optional"].(bool); ok {
		block.Optional = v
	}
	if v, ok := config["block_id"].(string); ok {
		block.BlockID = v
	}

	return block
}

func NewSlackOutput(config map[string]interface{}) (*SlackOutput, error) {
	webhookURL, ok := config["webhook_url"].(string)
	if !ok || webhookURL == "" {
		return nil, fmt.Errorf("webhook_url is required for Slack output")
	}

	channel, _ := config["channel"].(string)

	var blockConfigs []BlockConfig
	if blocksConfig, ok := config["blocks"].([]interface{}); ok {
		for _, blockConfig := range blocksConfig {
			if blockMap, ok := blockConfig.(map[string]interface{}); ok {
				block := mapBlockConfig(blockMap)
				blockConfigs = append(blockConfigs, block)
			}
		}
	}

	var templateEngine *SlackTemplateEngine
	if len(blockConfigs) > 0 {
		templateEngine = NewSlackTemplateEngine()
	}

	return &SlackOutput{
		webhookURL:     webhookURL,
		channel:        channel,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		blockConfigs:   blockConfigs,
		templateEngine: templateEngine,
	}, nil
}

func (o *SlackOutput) Send(event nomad.Event) error {
	var message SlackMessage
	var err error

	if o.templateEngine != nil && len(o.blockConfigs) > 0 {
		message, err = o.formatEventWithBlocks(event)
		if err != nil {
			message = o.formatEvent(event)
		}
	} else {
		message = o.formatEvent(event)
	}

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

func (o *SlackOutput) formatEventWithBlocks(event nomad.Event) (SlackMessage, error) {
	blocks, err := o.templateEngine.ProcessBlocks(o.blockConfigs, event)
	if err != nil {
		return SlackMessage{}, fmt.Errorf("failed to process blocks: %w", err)
	}

	message := SlackMessage{
		Blocks: blocks,
	}

	if o.channel != "" {
		message.Channel = o.channel
	}

	return message, nil
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
