package outputs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"nomad-events/internal/nomad"
)

type HTTPOutput struct {
	url        string
	method     string
	headers    map[string]string
	httpClient *http.Client
}

func NewHTTPOutput(config map[string]interface{}) (*HTTPOutput, error) {
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("url is required for HTTP output")
	}

	method, ok := config["method"].(string)
	if !ok {
		method = "POST"
	}

	headers := make(map[string]string)
	if h, ok := config["headers"].(map[string]interface{}); ok {
		for k, v := range h {
			if s, ok := v.(string); ok {
				headers[k] = s
			}
		}
	}

	timeout := 10 * time.Second
	if t, ok := config["timeout"].(int); ok {
		timeout = time.Duration(t) * time.Second
	}

	return &HTTPOutput{
		url:        url,
		method:     method,
		headers:    headers,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

func (o *HTTPOutput) Send(event nomad.Event) error {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event to JSON: %w", err)
	}

	req, err := http.NewRequest(o.method, o.url, bytes.NewBuffer(eventJSON))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range o.headers {
		req.Header.Set(k, v)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP request returned status %d", resp.StatusCode)
	}

	return nil
}
