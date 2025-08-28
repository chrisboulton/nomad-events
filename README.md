# Nomad Events Service

A Go service that connects to the Nomad HTTP events streaming API, processes events through a rules-based routing engine, and forwards them to various output destinations.

## Features

- **Nomad Event Streaming**: Connects to Nomad's event stream API with automatic reconnection and backoff
- **Hierarchical Routing**: CEL-based expression filtering with unlimited depth child routes (AlertManager-style)
- **Multiple Output Types**: Support for stdout, Slack, HTTP webhooks, RabbitMQ, and command execution  
- **Hot Configuration Reload**: Reload routing and output configuration via SIGHUP without restart
- **Fault Tolerance**: Automatic reconnection with exponential backoff and configurable retry logic
- **Index Tracking**: Resumes from last received event index after reconnection
- **Structured Logging**: Comprehensive structured logging with configurable levels and formats

## Configuration

The service is configured via YAML file (default: `config.yaml`):

```yaml
nomad:
  address: "http://localhost:4646"
  token: ""

outputs:
  stdout_output:
    type: stdout

  slack_alerts:
    type: slack
    webhook_url: "https://hooks.slack.com/services/..."
    channel: "#alerts"

routes:
  - filter: event.Topic == 'Node'
    output: slack_alerts
```

### Output Types

#### stdout
Outputs events to standard output with configurable formatting.
- `format`: Output format - "json" (default) or "text"
- `text`: Text template using Go template syntax (required when format is "text")

**JSON Format:**
Outputs events as single-line JSON for easy parsing and processing.

```yaml
stdout_json:
  type: stdout
  format: json
```

**Text Format:**
Outputs events using Go templates for custom formatting:

```yaml
stdout_text:
  type: stdout
  format: text
  text: "{{ .Topic }}/{{ .Type }} on {{ .Payload.Node.Name | default \"unknown\" }} (Index: {{ .Index }})"
```

#### slack
Sends events to Slack channels via webhooks with optional text templating and BlockKit formatting.
- `webhook_url`: Slack webhook URL (required)
- `channel`: Target channel (optional)
- `text`: Message text template using Go template syntax (optional)
- `blocks`: Optional BlockKit blocks configuration for rich message formatting

**Text Templating:**
Use Go template syntax to create dynamic messages from event data:

```yaml
slack_alerts:
  type: slack
  webhook_url: "https://hooks.slack.com/services/..."
  channel: "#alerts"
  text: "🚨 {{ .Topic }}/{{ .Type }} event on {{ .Payload.Node.Name | default \"unknown node\" }}"
```

**BlockKit Configuration:**
The `blocks` configuration allows you to create rich, interactive Slack messages using Go templates:

```yaml
slack_deployments:
  type: slack
  webhook_url: "https://hooks.slack.com/services/..."
  channel: "#deployments"
  blocks:
    # Conditional block - only for deployment events
    - condition: "event.Topic == 'Deployment'"
      type: header
      text: "🚀 Deployment: {{ .Payload.DeploymentID }}"
    - type: section
      text:
        type: mrkdwn
        text: "*Status:* {{ .Payload.Status }}\n*Node:* {{ .Payload.Node }}"
      fields:
        - type: mrkdwn
          text: "*Started:*"
        - type: plain_text
          text: "{{ .Payload.StartTime }}"
        # Conditional field - only if there are failures
        - condition: "event.Payload.Failed > 0"
          type: mrkdwn
          text: "*Failures:*"
        - condition: "event.Payload.Failed > 0"
          type: plain_text
          text: "{{ .Payload.Failed }}"
    - type: actions
      elements:
        - type: button
          text:
            type: plain_text
            text: "View Details"
          url: "https://nomad.example.com/ui/deployments/{{ .Payload.DeploymentID }}"
        # Conditional element - failure investigation button
        - condition: "event.Payload.Failed > 0"
          type: button
          text:
            type: plain_text
            text: "Investigate Failures"
          url: "https://nomad.example.com/ui/deployments/{{ .Payload.DeploymentID }}/failures"
    # Conditional range - only show running services
    - range: .Payload.Services
      condition: "event.Status == 'running'"
      type: context
      elements:
        - type: mrkdwn
          text: "Service: {{ .Name }} ({{ .Version }})"
```

**Supported Block Types:**
- `header`: Large header text
- `section`: Text with optional fields and accessories
- `divider`: Visual separator line
- `context`: Small contextual text elements
- `actions`: Interactive buttons and select menus
- `image`: Images with optional titles

**Template Features:**
- Full Go template syntax with event data interpolation
- Custom `range` directive for iterating over arrays at block and property level
- Mixed static and dynamic content in all array properties
- **Conditional formatting** using CEL expressions for blocks, fields, elements, and options
- Template functions: `upper`, `lower`, `title`
- Nomad API helper functions for retrieving job and cluster information
- Graceful fallback to simple message format on errors

**Enhanced Range Support:**
The `range` directive now works within any array property, allowing you to mix static and dynamic content:

```yaml
# Section with mixed static/dynamic fields
- type: section
  text: "Service Status Report"
  fields:
    # Static fields
    - type: mrkdwn
      text: "*Total Services:*"
    - type: plain_text
      text: "{{ .Payload.ServiceCount }}"
    # Dynamic fields from range
    - range: .Payload.Services
      type: mrkdwn
      text: "*{{ .Name }}:*"
    - range: .Payload.Services
      type: plain_text
      text: "{{ .Status }} ({{ .Version }})"

# Actions with mixed static/dynamic elements
- type: actions
  elements:
    # Static button
    - type: button
      text: "View All"
      url: "/ui/services"
    # Dynamic buttons from range
    - range: .Payload.QuickActions
      type: button
      text: "{{ .Label }}"
      url: "{{ .URL }}"
      action_id: "quick_{{ .ID }}"

# Select with mixed static/dynamic options
- type: static_select
  options:
    # Static option
    - text: "All Items"
      value: "all"
    # Dynamic options from range
    - range: .Payload.Items
      text: "{{ .Name }} ({{ .Status }})"
      value: "item_{{ .ID }}"
```

**Conditional Formatting:**
The `condition` property allows blocks, fields, elements, and options to be conditionally included based on CEL expressions:

```yaml
blocks:
  # Conditional block - only shows for Deployment events
  - condition: "event.Topic == 'Deployment'"
    type: header
    text: "🚀 Deployment: {{ .Payload.DeploymentID }}"

  # Section with conditional fields
  - type: section
    text:
      type: mrkdwn
      text: "*Event Details:*"
    fields:
      # Always included
      - type: mrkdwn
        text: "*Topic:*"
      - type: plain_text
        text: "{{ .Topic }}"
      # Only included if StartTime field exists
      - condition: "has(event.Payload.StartTime)"
        type: mrkdwn
        text: "*Started:*"
      - condition: "has(event.Payload.StartTime)"
        type: plain_text
        text: "{{ .Payload.StartTime }}"

  # Actions with conditional elements
  - type: actions
    elements:
      # Always included
      - type: button
        text:
          type: plain_text
          text: "View Details"
        url: "https://nomad.example.com/ui/jobs/{{ .Payload.Job.ID }}"
      # Only included if there are failures
      - condition: "event.Payload.Failed > 0"
        type: button
        text:
          type: plain_text
          text: "View Failures"
        url: "https://nomad.example.com/ui/jobs/{{ .Payload.Job.ID }}/failures"
```

**Conditional with Range:**
Conditions work seamlessly with range directives:

```yaml
# Show only running services
- type: section
  text: "Running Services:"
  fields:
    - range: .Payload.Services
      condition: "event.Status == 'running'" # event refers to the current item in range
      type: mrkdwn
      text: "*{{ .Name }}:* {{ .Status }}"

# Mixed conditional and non-conditional options
- type: static_select
  placeholder:
    type: plain_text
    text: "Select action..."
  options:
    # Static option
    - text:
        type: plain_text
        text: "View All"
      value: "all"
    # Conditional options from range
    - range: .Payload.Actions
      condition: "event.Enabled == true" # Only include enabled actions
      text:
        type: plain_text
        text: "{{ .Label }}"
      value: "action_{{ .ID }}"
```

**Condition Expression Syntax:**
Conditions use CEL (Common Expression Language) with access to full event data:

- `event.Topic == 'Node'` - Check event topic
- `event.Type == 'JobRegistered'` - Check event type
- `has(event.Payload.StartTime)` - Check if field exists
- `event.Payload.Failed > 0` - Numeric comparisons
- `event.Topic == 'Job' && event.Type == 'JobFailed'` - Complex conditions with AND/OR
- `event.Payload.Node.Name in ['worker-1', 'worker-2']` - Check if value is in list

**Error Handling:**
- Invalid conditions gracefully degrade to always include the item
- Missing CEL environment falls back to including all items
- Condition evaluation errors are logged but don't break message formatting

**Empty Message Handling:**
- If all blocks are filtered out by conditions, the Slack message is not sent
- Text-only messages (without blocks) are always sent regardless of conditions
- Messages with no content (no blocks and no text) are automatically skipped

#### http
Sends events via HTTP requests.
- `url`: Target URL (required)
- `method`: HTTP method (default: POST)
- `headers`: Custom headers map
- `timeout`: Request timeout in seconds (default: 10)

#### rabbitmq
Publishes events to RabbitMQ with support for Go templating in routing key names.
- `url`: AMQP connection URL (required)
- `exchange`: Exchange name (static)
- `routing_key`: Routing key template (supports Go templating, default: "nomad.{{ .Topic }}.{{ .Type }}")
- `queue`: Queue name (static)
- `durable`: Durable queues/exchanges (default: true)
- `auto_delete`: Auto-delete queues/exchanges (default: false)

**Routing Key Templates:**
The routing key supports Go template syntax with event data for dynamic message routing:

```yaml
rabbitmq_dynamic:
  type: rabbitmq
  url: "amqp://guest:guest@localhost:5672/"
  exchange: "nomad"
  queue: "events"
  routing_key: "{{ .Topic }}.{{ .Type }}.{{ .Payload.Job.Namespace | default \"default\" }}"
```

**Available Template Data:**
- `{{ .Topic }}`: Event topic (Node, Job, Allocation, etc.)
- `{{ .Type }}`: Event type (JobRegistered, NodeRegistration, etc.)
- `{{ .Key }}`: Event key
- `{{ .Namespace }}`: Nomad namespace
- `{{ .Index }}`: Event index
- `{{ .Payload }}`: Full event payload with job/node/allocation data

**Template Features:**
- Full Go template syntax with sprig functions (`upper`, `lower`, `title`, etc.)
- Automatic whitespace trimming for routing key names
- Graceful fallback to original template on errors
- Static exchange and queue names for infrastructure simplicity

#### exec
Executes a command with event data passed via stdin as JSON.
- `command`: Command to execute (required) - can be string or array
- `timeout`: Command timeout in seconds (default: 30)
- `workdir`: Working directory for command execution
- `env`: Environment variables map

### Template Functions

Templates support several helper functions for enriching output with additional data from the Nomad API:

#### Nomad API Functions
These functions allow templates to retrieve live data from the Nomad cluster:

- `job "job-id"`: Retrieve a Nomad job by ID
- `jobAllocs "job-id"`: Get allocations for a job
- `jobEvaluations "job-id"`: Get evaluations for a job
- `jobSummary "job-id"`: Get job summary
- `evaluation "eval-id"`: Retrieve evaluation by ID
- `evaluationAllocs "eval-id"`: Get allocations for evaluation
- `deploymentAllocs "deployment-id"`: Get allocations for deployment

**Example Usage:**
```yaml
slack_job_info:
  type: slack
  webhook_url: "https://hooks.slack.com/services/..."
  channel: "#jobs"
  text: |
    Job: {{ .Payload.Job.ID }}
    Current Status: {{ (job .Payload.Job.ID).Status }}
    Running Allocations: {{ len (jobAllocs .Payload.Job.ID) }}
    Summary: {{ (jobSummary .Payload.Job.ID).Summary }}
```

**Example with BlockKit:**
```yaml
slack_job_details:
  type: slack
  webhook_url: "https://hooks.slack.com/services/..."
  channel: "#jobs"
  blocks:
    - type: header
      text: "📋 Job: {{ .Payload.Job.ID }}"
    - type: section
      text:
        type: mrkdwn
        text: "*Status:* {{ (job .Payload.Job.ID).Status }}\n*Type:* {{ (job .Payload.Job.ID).Type }}"
      fields:
        - type: mrkdwn
          text: "*Running:*"
        - type: plain_text
          text: "{{ (jobSummary .Payload.Job.ID).Summary.Running }}"
        - type: mrkdwn
          text: "*Failed:*"
        - type: plain_text
          text: "{{ (jobSummary .Payload.Job.ID).Summary.Failed }}"
    - type: context
      elements:
        - type: mrkdwn
          text: "Total Allocations: {{ len (jobAllocs .Payload.Job.ID) }}"
```

Note: These functions make live API calls to Nomad, so use them judiciously to avoid performance impact. Functions return `nil` if the API call fails or if the Nomad client is not available.

### Job Diffs

For `JobRegistered` events with version > 1, the service automatically fetches job version diffs from the Nomad API and makes them available in both filters and templates. New jobs (version 1) do not have diffs since there's no previous version to compare against.

**Filter Usage:**
```yaml
routes:
  # Route jobs with scaling changes
  - filter: "event.Topic == 'Job' && event.Type == 'JobRegistered' && has(event.Diff)"
    output: job_changes

  # Route specific task group changes
  - filter: "event.Topic == 'Job' && event.Type == 'JobRegistered' && event.Diff.TaskGroups.Name == 'web'"
    output: web_scaling
```

**Template Usage:**
```yaml
stdout_job_scaling:
  type: stdout
  format: text
  text: |
    🚀 {{ .Topic }}/{{ .Type }}: {{ .Payload.Job.ID }}
    {{ if .Diff }}
    Changes:
    {{ range $tg := .Diff.TaskGroups }}
      Task Group: {{ $tg.Name }}
      {{ range $field := $tg.Fields }}
        {{ $field.Name }}: {{ $field.Old }} → {{ $field.New }}
      {{ end }}
    {{ end }}
    {{ end }}
```

**Diff Structure:**
The diff object contains structured information about changes between job versions:
- `Type`: Type of diff (e.g., "Added", "Edited", "Deleted")
- `TaskGroups`: Array of task group changes
  - `Name`: Task group name
  - `Type`: Change type for this task group
  - `Fields`: Array of field-level changes
    - `Name`: Field name (e.g., "Count", "Resources")
    - `Old`: Previous value
    - `New`: New value
- `Objects`: Top-level job specification changes

**Error Handling:**
- If the job diff API call fails, the event is still processed but `event.Diff` will be `nil`
- Use `has(event.Diff)` in filters to check for diff availability
- Templates should check for diff existence: `{{ if .Diff }}...{{ end }}`

### Routing Rules

Routes use CEL (Common Expression Language) to filter events:

```yaml
routes:
  # Parent route processes all Node events
  - filter: event.Topic == 'Node'
    output: node_events
    routes:
      # Child routes for specific node event types
      - filter: event.Type == 'NodeRegistration'
        output: node_registrations
      - filter: event.Type == 'NodeDrain'
        output: node_drains
        continue: false  # Stop processing other child routes

  # Job events with continue=false stops other top-level routes
  - filter: event.Topic == 'Job'
    continue: false
    routes:
      - filter: event.Type == 'JobRegistered'
        output: job_events
```

Event fields available in filters:
- `event.Topic`: Event topic (Node, Job, Allocation, etc.)
- `event.Type`: Event type
- `event.Key`: Event key
- `event.Namespace`: Nomad namespace
- `event.Index`: Event index
- `event.Payload`: Parsed JSON payload
- `diff`: Direct access to diff data (only available for JobRegistered events with version > 1)

## Usage

```bash
# Install dependencies
go mod download

# Build
go build -o nomad-events cmd/nomad-events/main.go

# Run with default config
./nomad-events

# Run with custom config
./nomad-events -config /path/to/config.yaml

# Validate configuration before running
./nomad-events -validate-config -config /path/to/config.yaml

# Run with debug logging
./nomad-events -log-level debug -log-format json
```

### Configuration Reload

The service supports hot configuration reloading via SIGHUP signal:

```bash
# Start the service
./nomad-events -config config.yaml &
PID=$!

# Update config.yaml with new settings
vim config.yaml

# Reload configuration without restart
kill -HUP $PID
```

**Reload Behavior:**
- ✅ **Zero downtime**: Event processing continues during reload
- ✅ **Atomic updates**: New configuration is validated before applying
- ✅ **Graceful fallback**: Service continues with current config if reload fails
- ✅ **Thread-safe**: Concurrent event processing is fully supported
- ✅ **Comprehensive logging**: Detailed reload status and error reporting

**What can be reloaded:**
- Routing rules and filters
- Output configurations and settings
- Retry policies
- Template configurations

**What requires restart:**
- Nomad connection settings (address, token)
- Log level and format settings

## Example Events

Events received from Nomad have the following structure:

**Node Event:**
```json
{
  "Topic": "Node",
  "Type": "NodeRegistration",
  "Key": "node-id",
  "Namespace": "default",
  "Index": 12345,
  "Payload": {
    "Node": {
      "Name": "worker-1",
      "Datacenter": "dc1"
    }
  }
}
```
