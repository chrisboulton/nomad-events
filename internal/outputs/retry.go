package outputs

import (
	"fmt"
	"log/slog"
	"time"

	"nomad-events/internal/nomad"
)

// RetryOutput wraps another Output with retry logic
type RetryOutput struct {
	output     Output
	maxRetries int
	baseDelay  time.Duration
}

// RetryConfig holds configuration for retry behavior
type RetryConfig struct {
	MaxRetries int           `yaml:"max_retries"`
	BaseDelay  time.Duration `yaml:"base_delay"`
}

// NewRetryOutput creates a new RetryOutput wrapper
func NewRetryOutput(output Output, config RetryConfig) *RetryOutput {
	// Set defaults if not specified
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.BaseDelay == 0 {
		config.BaseDelay = 1 * time.Second
	}

	return &RetryOutput{
		output:     output,
		maxRetries: config.MaxRetries,
		baseDelay:  config.BaseDelay,
	}
}

// Send implements the Output interface with retry logic
func (r *RetryOutput) Send(event nomad.Event) error {
	var lastErr error
	
	for attempt := 1; attempt <= r.maxRetries; attempt++ {
		err := r.output.Send(event)
		if err == nil {
			// Success
			if attempt > 1 {
				slog.Info("Event sent successfully after retry",
					"topic", event.Topic,
					"type", event.Type,
					"attempt", attempt)
			}
			return nil
		}

		lastErr = err
		
		if attempt < r.maxRetries {
			// Calculate exponential backoff delay
			delay := r.baseDelay * time.Duration(1<<(attempt-1)) // 1s, 2s, 4s, 8s, etc.
			
			slog.Warn("Output send failed, retrying",
				"topic", event.Topic,
				"type", event.Type,
				"attempt", attempt,
				"max_retries", r.maxRetries,
				"error", err,
				"retry_delay", delay)
			
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("failed to send event after %d attempts: %w", r.maxRetries, lastErr)
}