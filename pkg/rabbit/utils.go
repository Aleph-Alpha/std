package rabbit

import (
	"context"
	"fmt"
	"github.com/streadway/amqp"
	"sync"
	"time"
)

type Message interface {
	AckMsg() error
	NackMsg(requeue bool) error
	Body() []byte
}

type ConsumerMessage struct {
	body     []byte
	delivery *amqp.Delivery
}

// ConsumeQueue consumes messages from a specified queue
func (rb *Rabbit) consumeQueue(ctx context.Context, wg *sync.WaitGroup, queueName string) <-chan Message {
	outChan := make(chan Message, 100)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(outChan)
	outerLoop:
		for {
			select {
			case <-rb.shutdownSignal:
				rb.logger.Info("consumer is shutting down due to shutdown signal", nil, nil)
				return
			case <-ctx.Done():
				rb.logger.Info("consumer is shutting down due to context cancellation", ctx.Err(), nil)
				return
			default:
				rb.mu.RLock()
				msgs, err := rb.Channel.Consume(
					queueName,
					"",    // consumer
					false, // autoAck
					false, // exclusive
					false, // noLocal
					false, // noWait
					nil,   // args
				)
				rb.mu.RUnlock()

				if err != nil {
					rb.logger.Error("error in establishing consumer for rabbit", err, map[string]interface{}{
						"queue_name": queueName,
					})
					time.Sleep(100 * time.Millisecond)
					continue
				}

				for {
					select {
					case <-ctx.Done():
						rb.logger.Info("consumer is shutting down due to context cancellation", ctx.Err(), nil)
						return
					case <-rb.shutdownSignal:
						rb.logger.Info("consumer is shutting down due to shutdown signal", nil, nil)
						return
					case msg, ok := <-msgs:
						if !ok {
							continue outerLoop
						}
						rb.logger.Debug("message consumed from rabbit", nil, map[string]interface{}{
							"queue_name": queueName,
							"payload":    fmt.Sprintf("%v", string(msg.Body)),
						})
						outChan <- &ConsumerMessage{
							body:     msg.Body,
							delivery: &msg,
						}
					}
				}
			}
		}
	}()
	return outChan
}

// Consume is now a wrapper for backward compatibility
func (rb *Rabbit) Consume(ctx context.Context, wg *sync.WaitGroup) <-chan Message {
	return rb.consumeQueue(ctx, wg, rb.cfg.Channel.QueueName)
}

// ConsumeDLQ is a convenience wrapper for consuming from DLQ
func (rb *Rabbit) ConsumeDLQ(ctx context.Context, wg *sync.WaitGroup) <-chan Message {
	return rb.consumeQueue(ctx, wg, "dlq-queue")
}

// Publish sends a message to the RabbitMQ exchange.
func (rb *Rabbit) Publish(ctx context.Context, msg []byte) error {

	select {
	case <-ctx.Done():
		rb.logger.Error("context error for publishing msg into rabbit", ctx.Err(), nil)
		return ctx.Err()
	default:
		rb.mu.RLock()
		err := rb.Channel.Publish(rb.cfg.Channel.ExchangeName,
			rb.cfg.Channel.RoutingKey,
			false,
			false,
			amqp.Publishing{
				//Headers:         nil,
				ContentType: rb.cfg.Channel.ContentType,
				//ContentEncoding: "",
				//DeliveryMode:    0,
				//Priority:        0,
				//CorrelationId:   "",
				//ReplyTo:         "",
				//Expiration:      "",
				//MessageId:       "",
				//Timestamp:       time.Time{},
				//Type:            "",
				//UserId:          "",
				//AppId:           "",
				Body: msg,
			},
		)
		rb.mu.RUnlock()

		if err == nil {
			return nil
		}
		rb.logger.Error("error in publishing msg into rabbit", err, nil)
		return err
	}
}

func (rb *ConsumerMessage) AckMsg() error {
	return rb.delivery.Ack(false)
}

func (rb *ConsumerMessage) NackMsg(requeue bool) error {
	return rb.delivery.Nack(false, requeue)
}

func (rb *ConsumerMessage) Body() []byte {
	return rb.body
}
