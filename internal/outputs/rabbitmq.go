package outputs

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/streadway/amqp"
	"nomad-events/internal/nomad"
)

type RabbitMQOutput struct {
	connection   *amqp.Connection
	channel      *amqp.Channel
	exchange     string
	routingKey   string
	queue        string
	durable      bool
	autoDelete   bool
}

func NewRabbitMQOutput(config map[string]interface{}) (*RabbitMQOutput, error) {
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("url is required for RabbitMQ output")
	}

	exchange, _ := config["exchange"].(string)
	routingKey, _ := config["routing_key"].(string)
	queue, _ := config["queue"].(string)
	
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
		connection: conn,
		channel:    ch,
		exchange:   exchange,
		routingKey: routingKey,
		queue:      queue,
		durable:    durable,
		autoDelete: autoDelete,
	}

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
			if err := o.channel.QueueBind(
				o.queue,
				o.routingKey,
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
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event to JSON: %w", err)
	}

	routingKey := o.routingKey
	if routingKey == "" {
		routingKey = fmt.Sprintf("nomad.%s.%s", event.Topic, event.Type)
	}

	target := o.exchange
	if target == "" {
		target = o.queue
	}

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

func (o *RabbitMQOutput) Close() error {
	if o.channel != nil {
		o.channel.Close()
	}
	if o.connection != nil {
		return o.connection.Close()
	}
	return nil
}