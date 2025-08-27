package outputs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"nomad-events/internal/nomad"

	"github.com/hashicorp/nomad/api"
)

type SlackOutput struct {
	webhookURL     string
	channel        string
	textTemplate   string
	httpClient     *http.Client
	blockConfigs   []BlockConfig
	templateEngine *SlackTemplateEngine
}

type SlackMessage struct {
	Channel string      `json:"channel,omitempty"`
	Text    string      `json:"text,omitempty"`
	Blocks  interface{} `json:"blocks,omitempty"`
}

// mapBlockConfig maps a configuration map to a BlockConfig struct
func mapBlockConfig(config map[string]interface{}) BlockConfig {
	var block BlockConfig

	// Map fields using a simple lookup approach
	if v, ok := config["type"].(string); ok {
		block.Type = v
	}
	if v, ok := config["condition"].(string); ok {
		block.Condition = v
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

func NewSlackOutput(config map[string]interface{}, nomadClient *api.Client) (*SlackOutput, error) {
	webhookURL, ok := config["webhook_url"].(string)
	if !ok || webhookURL == "" {
		return nil, fmt.Errorf("webhook_url is required for Slack output")
	}

	channel, _ := config["channel"].(string)
	textTemplate, _ := config["text"].(string)

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
	if len(blockConfigs) > 0 || textTemplate != "" {
		templateEngine = NewSlackTemplateEngine(nomadClient)
	}

	return &SlackOutput{
		webhookURL:     webhookURL,
		channel:        channel,
		textTemplate:   textTemplate,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		blockConfigs:   blockConfigs,
		templateEngine: templateEngine,
	}, nil
}

func (o *SlackOutput) Send(event nomad.Event) error {
	var message SlackMessage
	var err error

	message, err = o.formatEvent(event)
	if err != nil {
		return fmt.Errorf("failed to format event: %w", err)
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

func (o *SlackOutput) formatEvent(event nomad.Event) (SlackMessage, error) {

	blocks, err := o.templateEngine.ProcessBlocks(o.blockConfigs, event)
	if err != nil {
		return SlackMessage{}, fmt.Errorf("failed to process blocks: %w", err)
	}

	text, err := o.templateEngine.ProcessText(o.textTemplate, event)
	if err != nil {
		return SlackMessage{}, fmt.Errorf("failed to process text: %w", err)
	}

	message := SlackMessage{
		Text:    text,
		Blocks:  blocks,
		Channel: o.channel,
	}

	return message, nil
}
