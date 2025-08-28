package outputs

import (
	"errors"
	"testing"
	"time"

	"nomad-events/internal/nomad"
	"github.com/stretchr/testify/assert"
)

// MockOutput implements Output interface for testing
type MockOutput struct {
	sendCalls   int
	shouldFail  bool
	failTimes   int
	lastEvent   nomad.Event
}

func (m *MockOutput) Send(event nomad.Event) error {
	m.sendCalls++
	m.lastEvent = event
	
	if m.shouldFail && m.sendCalls <= m.failTimes {
		return errors.New("mock error")
	}
	return nil
}

func TestRetryOutput(t *testing.T) {
	testEvent := nomad.Event{
		Topic: "Test",
		Type:  "TestEvent",
		Key:   "test-key",
		Index: 1,
	}

	t.Run("success_on_first_try", func(t *testing.T) {
		mock := &MockOutput{}
		retry := NewRetryOutput(mock, RetryConfig{
			MaxRetries: 3,
			BaseDelay:  10 * time.Millisecond,
		})

		err := retry.Send(testEvent)
		assert.NoError(t, err)
		assert.Equal(t, 1, mock.sendCalls)
	})

	t.Run("success_after_retry", func(t *testing.T) {
		mock := &MockOutput{
			shouldFail: true,
			failTimes:  2, // Fail first 2 attempts
		}
		retry := NewRetryOutput(mock, RetryConfig{
			MaxRetries: 3,
			BaseDelay:  10 * time.Millisecond,
		})

		err := retry.Send(testEvent)
		assert.NoError(t, err)
		assert.Equal(t, 3, mock.sendCalls) // Failed twice, succeeded on third
	})

	t.Run("failure_after_max_retries", func(t *testing.T) {
		mock := &MockOutput{
			shouldFail: true,
			failTimes:  10, // Always fail
		}
		retry := NewRetryOutput(mock, RetryConfig{
			MaxRetries: 3,
			BaseDelay:  10 * time.Millisecond,
		})

		err := retry.Send(testEvent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send event after 3 attempts")
		assert.Equal(t, 3, mock.sendCalls)
	})

	t.Run("default_configuration", func(t *testing.T) {
		mock := &MockOutput{}
		retry := NewRetryOutput(mock, RetryConfig{}) // Empty config should use defaults

		assert.Equal(t, 3, retry.maxRetries)
		assert.Equal(t, 1*time.Second, retry.baseDelay)
	})
}