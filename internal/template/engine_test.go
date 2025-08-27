package template

import (
	"testing"

	"nomad-events/internal/nomad"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEngine(t *testing.T) {
	t.Run("without nomad client", func(t *testing.T) {
		engine := NewEngine(nil)
		assert.NotNil(t, engine)
		assert.NotNil(t, engine.funcMap)
		assert.Nil(t, engine.nomadClient)
	})

	t.Run("with nomad client", func(t *testing.T) {
		client, err := api.NewClient(api.DefaultConfig())
		require.NoError(t, err)

		engine := NewEngine(client)
		assert.NotNil(t, engine)
		assert.NotNil(t, engine.funcMap)
		assert.Equal(t, client, engine.nomadClient)
	})
}

func TestEngineProcessText(t *testing.T) {
	engine := NewEngine(nil)

	event := nomad.Event{
		Topic:     "Job",
		Type:      "JobRegistered",
		Key:       "example-job",
		Namespace: "default",
		Index:     12345,
		Payload: map[string]interface{}{
			"Job": map[string]interface{}{
				"ID":   "example-job",
				"Name": "example",
			},
		},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "simple topic template",
			template: "{{ .Topic }}",
			expected: "Job",
		},
		{
			name:     "combined template",
			template: "{{ .Topic }}/{{ .Type }} - {{ .Payload.Job.ID }}",
			expected: "Job/JobRegistered - example-job",
		},
		{
			name:     "index template",
			template: "Index: {{ .Index }}",
			expected: "Index: 12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.ProcessText(tt.template, event)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEngineCreateTemplateData(t *testing.T) {
	engine := NewEngine(nil)

	event := nomad.Event{
		Topic:     "Job",
		Type:      "JobRegistered",
		Key:       "example-job",
		Namespace: "default",
		Index:     12345,
		Payload: map[string]interface{}{
			"Job": map[string]interface{}{
				"ID": "example-job",
			},
		},
	}

	data := engine.CreateTemplateData(event)

	assert.Equal(t, "Job", data["Topic"])
	assert.Equal(t, "JobRegistered", data["Type"])
	assert.Equal(t, "example-job", data["Key"])
	assert.Equal(t, "default", data["Namespace"])
	assert.Equal(t, uint64(12345), data["Index"])
	assert.NotNil(t, data["Payload"])

	payload := data["Payload"].(map[string]interface{})
	job := payload["Job"].(map[string]interface{})
	assert.Equal(t, "example-job", job["ID"])
}

func TestEngineNomadAPIFunctions(t *testing.T) {
	t.Run("without nomad client", func(t *testing.T) {
		engine := NewEngine(nil)

		// Test that functions return nil when client is nil
		job, err := engine.jobFunc("test-job")
		assert.NoError(t, err)
		assert.Nil(t, job)

		allocs, err := engine.jobAllocsFunc("test-job")
		assert.NoError(t, err)
		assert.Nil(t, allocs)

		evals, err := engine.jobEvaluationsFunc("test-job")
		assert.NoError(t, err)
		assert.Nil(t, evals)

		summary, err := engine.jobSummaryFunc("test-job")
		assert.NoError(t, err)
		assert.Nil(t, summary)

		eval, err := engine.evaluationFunc("test-eval")
		assert.NoError(t, err)
		assert.Nil(t, eval)

		evalAllocs, err := engine.evaluationAllocsFunc("test-eval")
		assert.NoError(t, err)
		assert.Nil(t, evalAllocs)

		deployAllocs, err := engine.deploymentAllocsFunc("test-deployment")
		assert.NoError(t, err)
		assert.Nil(t, deployAllocs)
	})
}
