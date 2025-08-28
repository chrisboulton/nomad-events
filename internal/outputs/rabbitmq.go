package outputs

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/streadway/amqp"
	"nomad-events/internal/nomad"
	"nomad-events/internal/template"
)

type RabbitMQOutput struct {
	connection         *amqp.Connection
	channel            *amqp.Channel
	exchange           string
	routingKeyTemplate string
	queue              string
	durable            bool
	autoDelete         bool
	templateEngine     *template.Engine
}

func NewRabbitMQOutput(config map[string]interface{}) (*RabbitMQOutput, error) {
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("url is required for RabbitMQ output")
	}

	exchange, _ := config["exchange"].(string)
	routingKeyTemplate, _ := config["routing_key"].(string)
	queue, _ := config["queue"].(string)
	
	// Set default routing key template
	if routingKeyTemplate == "" {
		routingKeyTemplate = "nomad.{{ .Topic }}.{{ .Type }}"
	}
	
	durable := true
	if d, ok := config["durable"].(bool); ok {
		durable = d
	}
	
	autoDelete := false
	if a, ok := config["auto_delete"].(bool); ok {
		autoDelete = a
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open RabbitMQ channel: %w", err)
	}

	output := &RabbitMQOutput{
		connection:         conn,
		channel:           ch,
		exchange:          exchange,
		routingKeyTemplate: routingKeyTemplate,
		queue:             queue,
		durable:           durable,
		autoDelete:        autoDelete,
		templateEngine:    template.NewEngine(),
	}

	// Setup exchange and queue since names are now static
	if err := output.setup(); err != nil {
		output.Close()
		return nil, err
	}

	return output, nil
}

func (o *RabbitMQOutput) setup() error {
	if o.exchange != "" {
		if err := o.channel.ExchangeDeclare(
			o.exchange,
			"topic",
			o.durable,
			o.autoDelete,
			false,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("failed to declare exchange: %w", err)
		}
	}

	if o.queue != "" {
		_, err := o.channel.QueueDeclare(
			o.queue,
			o.durable,
			o.autoDelete,
			false,
			false,
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to declare queue: %w", err)
		}

		if o.exchange != "" {
			// For static queue binding, we'll bind with a wildcard since routing key is dynamic
			if err := o.channel.QueueBind(
				o.queue,
				"#", // Wildcard to match all routing keys
				o.exchange,
				false,
				nil,
			); err != nil {
				return fmt.Errorf("failed to bind queue: %w", err)
			}
		}
	}

	return nil
}

func (o *RabbitMQOutput) Send(event nomad.Event) error {
	// Process routing key template only
	routingKey, err := o.processTemplate(o.routingKeyTemplate, event)
	if err != nil {
		return fmt.Errorf("failed to process routing key template: %w", err)
	}

	// Trim whitespace from routing key name
	routingKey = strings.TrimSpace(routingKey)

	// Marshal event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event to JSON: %w", err)
	}

	// Publish message
	if err := o.channel.Publish(
		o.exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Timestamp:   time.Now(),
			Body:        eventJSON,
		},
	); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// processTemplate processes a template string with the event data
func (o *RabbitMQOutput) processTemplate(templateStr string, event nomad.Event) (string, error) {
	if templateStr == "" {
		return "", nil
	}
	
	result, err := o.templateEngine.ProcessText(templateStr, event)
	if err != nil {
		return templateStr, err // Return original template on error
	}
	
	return result, nil
}

func (o *RabbitMQOutput) Close() error {
	if o.channel != nil {
		o.channel.Close()
	}
	if o.connection != nil {
		return o.connection.Close()
	}
	return nil
}