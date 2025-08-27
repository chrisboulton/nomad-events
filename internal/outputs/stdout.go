package outputs

import (
	"encoding/json"
	"fmt"
	"os"

	"nomad-events/internal/nomad"
)

type StdoutOutput struct{}

func NewStdoutOutput(config map[string]interface{}) (*StdoutOutput, error) {
	return &StdoutOutput{}, nil
}

func (o *StdoutOutput) Send(event nomad.Event) error {
	eventJSON, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal event to JSON: %w", err)
	}

	_, err = os.Stdout.Write(eventJSON)
	if err != nil {
		return fmt.Errorf("failed to write to stdout: %w", err)
	}

	_, err = os.Stdout.WriteString("\n")
	return err
}