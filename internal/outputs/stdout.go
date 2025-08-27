package outputs

import (
	"encoding/json"
	"fmt"
	"os"

	"nomad-events/internal/nomad"
	"nomad-events/internal/template"

	"github.com/hashicorp/nomad/api"
)

type StdoutOutput struct {
	format         string
	textTemplate   string
	templateEngine *template.Engine
}

func NewStdoutOutput(config map[string]interface{}, nomadClient *api.Client) (*StdoutOutput, error) {
	format, _ := config["format"].(string)
	if format == "" {
		format = "json" // Default to JSON format
	}

	if format != "json" && format != "text" {
		return nil, fmt.Errorf("invalid format %q: must be 'json' or 'text'", format)
	}

	textTemplate, _ := config["text"].(string)

	var templateEngine *template.Engine
	if format == "text" {
		if textTemplate == "" {
			return nil, fmt.Errorf("text template is required when format is 'text'")
		}
		templateEngine = template.NewEngine(nomadClient)
	}

	return &StdoutOutput{
		format:         format,
		textTemplate:   textTemplate,
		templateEngine: templateEngine,
	}, nil
}

func (o *StdoutOutput) Send(event nomad.Event) error {
	var output string
	var err error

	switch o.format {
	case "json":
		eventJSON, jsonErr := json.Marshal(event)
		if jsonErr != nil {
			return fmt.Errorf("failed to marshal event to JSON: %w", jsonErr)
		}
		output = string(eventJSON)

	case "text":
		output, err = o.templateEngine.ProcessText(o.textTemplate, event)
		if err != nil {
			return fmt.Errorf("failed to process text template: %w", err)
		}

	default:
		return fmt.Errorf("unsupported format: %s", o.format)
	}

	_, err = os.Stdout.WriteString(output + "\n")
	if err != nil {
		return fmt.Errorf("failed to write to stdout: %w", err)
	}

	return nil
}