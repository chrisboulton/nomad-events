# Nomad Events Service

A Go service that connects to the Nomad HTTP events streaming API, processes events through a rules-based routing engine, and forwards them to various output destinations.

## Features

- **Nomad Event Streaming**: Connects to Nomad's event stream API with automatic reconnection and backoff
- **Configurable Routing**: CEL-based expression filtering to route events to specific outputs  
- **Multiple Output Types**: Support for stdout, Slack, HTTP webhooks, RabbitMQ, and command execution
- **Fault Tolerance**: Automatic reconnection with exponential backoff when connections are lost
- **Index Tracking**: Resumes from last received event index after reconnection

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
Outputs events to standard output as formatted JSON.

#### slack  
Sends events to Slack channels via webhooks with optional BlockKit formatting.
- `webhook_url`: Slack webhook URL (required)
- `channel`: Target channel (optional)
- `blocks`: Optional BlockKit blocks configuration for rich message formatting

**BlockKit Configuration:**
The `blocks` configuration allows you to create rich, interactive Slack messages using Go templates:

```yaml
slack_deployments:
  type: slack
  webhook_url: "https://hooks.slack.com/services/..."
  channel: "#deployments"
  blocks:
    - type: header
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
    - type: actions
      elements:
        - type: button
          text:
            type: plain_text
            text: "View Details"
          url: "https://nomad.example.com/ui/deployments/{{ .Payload.DeploymentID }}"
    - range: .Payload.Services
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
- Template functions: `upper`, `lower`, `title`
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

#### http
Sends events via HTTP requests.
- `url`: Target URL (required) 
- `method`: HTTP method (default: POST)
- `headers`: Custom headers map
- `timeout`: Request timeout in seconds (default: 10)

#### rabbitmq
Publishes events to RabbitMQ.
- `url`: AMQP connection URL (required)
- `exchange`: Exchange name
- `routing_key`: Routing key pattern
- `queue`: Queue name  
- `durable`: Durable queues/exchanges (default: true)
- `auto_delete`: Auto-delete queues/exchanges (default: false)

#### exec
Executes a command with event data passed via stdin as JSON.
- `command`: Command to execute (required) - can be string or array
- `timeout`: Command timeout in seconds (default: 30)  
- `workdir`: Working directory for command execution
- `env`: Environment variables map

### Routing Rules

Routes use CEL (Common Expression Language) to filter events:

```yaml
routes:
  # All events
  - filter: ""
    output: stdout_output
    
  # Node registration events only
  - filter: event.Topic == 'Node' && event.Type == 'NodeRegistration' 
    output: slack_alerts
    
  # Events for specific node
  - filter: event.Topic == 'Node' && event.Payload.Node.Name == 'worker-1'
    output: node_specific_output
```

Event fields available in filters:
- `event.Topic`: Event topic (Node, Job, Allocation, etc.)
- `event.Type`: Event type 
- `event.Key`: Event key
- `event.Namespace`: Nomad namespace
- `event.Index`: Event index
- `event.Payload`: Parsed JSON payload

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
```

## Example Event

Events received from Nomad have the following structure:

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