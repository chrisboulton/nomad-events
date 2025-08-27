package template

import (
	"bytes"
	"text/template"

	"nomad-events/internal/nomad"

	"github.com/Masterminds/sprig/v3"
)

type Engine struct {
	funcMap template.FuncMap
}

func NewEngine() *Engine {
	return &Engine{
		funcMap: sprig.FuncMap(),
	}
}

func (e *Engine) ProcessText(text string, event nomad.Event) (string, error) {
	eventData := e.createTemplateData(event)
	return e.processText(text, eventData)
}

func (e *Engine) ProcessTextWithData(text string, eventData map[string]interface{}) (string, error) {
	return e.processText(text, eventData)
}

func (e *Engine) CreateTemplateData(event nomad.Event) map[string]interface{} {
	return e.createTemplateData(event)
}

func (e *Engine) processText(text string, eventData map[string]interface{}) (string, error) {
	tmpl, err := template.New("template").Funcs(e.funcMap).Parse(text)
	if err != nil {
		return text, nil // Return original text on parse error
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, eventData); err != nil {
		return text, nil // Return original text on execution error
	}

	return buf.String(), nil
}

func (e *Engine) createTemplateData(event nomad.Event) map[string]interface{} {
	data := map[string]interface{}{
		"Topic":     event.Topic,
		"Type":      event.Type,
		"Key":       event.Key,
		"Namespace": event.Namespace,
		"Index":     event.Index,
	}

	if event.Payload != nil {
		data["Payload"] = event.Payload
	}

	return data
}
