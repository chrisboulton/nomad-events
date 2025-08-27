package template

import (
	"bytes"
	"text/template"

	"nomad-events/internal/nomad"

	"github.com/Masterminds/sprig/v3"
	"github.com/hashicorp/nomad/api"
)

type Engine struct {
	funcMap     template.FuncMap
	nomadClient *api.Client
}

func NewEngine(nomadClient *api.Client) *Engine {
	e := &Engine{
		funcMap:     sprig.FuncMap(),
		nomadClient: nomadClient,
	}

	// Add our custom Nomad API functions
	e.addNomadFunctions()

	return e
}

func (e *Engine) addNomadFunctions() {
	// job: retrieve a Nomad job by job ID
	e.funcMap["job"] = e.jobFunc

	// jobAllocs: get allocations for job
	e.funcMap["jobAllocs"] = e.jobAllocsFunc

	// jobEvaluations: get evaluations for job
	e.funcMap["jobEvaluations"] = e.jobEvaluationsFunc

	// jobSummary: get job summary
	e.funcMap["jobSummary"] = e.jobSummaryFunc

	// evaluation: retrieve evaluation by ID
	e.funcMap["evaluation"] = e.evaluationFunc

	// evaluationAllocs: get allocations for evaluation
	e.funcMap["evaluationAllocs"] = e.evaluationAllocsFunc

	// deploymentAllocs: get allocations for deployment
	e.funcMap["deploymentAllocs"] = e.deploymentAllocsFunc
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

// jobFunc retrieves a Nomad job by job ID
func (e *Engine) jobFunc(jobID string) (*api.Job, error) {
	if e.nomadClient == nil {
		return nil, nil
	}

	job, _, err := e.nomadClient.Jobs().Info(jobID, nil)
	return job, err
}

// jobAllocsFunc gets allocations for a job
func (e *Engine) jobAllocsFunc(jobID string) ([]*api.AllocationListStub, error) {
	if e.nomadClient == nil {
		return nil, nil
	}

	allocs, _, err := e.nomadClient.Jobs().Allocations(jobID, true, nil)
	return allocs, err
}

// jobEvaluationsFunc gets evaluations for a job
func (e *Engine) jobEvaluationsFunc(jobID string) ([]*api.Evaluation, error) {
	if e.nomadClient == nil {
		return nil, nil
	}

	evals, _, err := e.nomadClient.Jobs().Evaluations(jobID, nil)
	return evals, err
}

// jobSummaryFunc gets job summary
func (e *Engine) jobSummaryFunc(jobID string) (*api.JobSummary, error) {
	if e.nomadClient == nil {
		return nil, nil
	}

	summary, _, err := e.nomadClient.Jobs().Summary(jobID, nil)
	return summary, err
}

// evaluationFunc retrieves evaluation by ID
func (e *Engine) evaluationFunc(evalID string) (*api.Evaluation, error) {
	if e.nomadClient == nil {
		return nil, nil
	}

	eval, _, err := e.nomadClient.Evaluations().Info(evalID, nil)
	return eval, err
}

// evaluationAllocsFunc gets allocations for evaluation
func (e *Engine) evaluationAllocsFunc(evalID string) ([]*api.AllocationListStub, error) {
	if e.nomadClient == nil {
		return nil, nil
	}

	allocs, _, err := e.nomadClient.Evaluations().Allocations(evalID, nil)
	return allocs, err
}

// deploymentAllocsFunc gets allocations for deployment
func (e *Engine) deploymentAllocsFunc(deploymentID string) ([]*api.AllocationListStub, error) {
	if e.nomadClient == nil {
		return nil, nil
	}

	allocs, _, err := e.nomadClient.Deployments().Allocations(deploymentID, nil)
	return allocs, err
}
