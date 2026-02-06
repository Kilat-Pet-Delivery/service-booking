package events

import (
	"context"
	"encoding/json"

	"github.com/Kilat-Pet-Delivery/lib-common/kafka"
	"github.com/Kilat-Pet-Delivery/lib-proto/events"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/application"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// PaymentEventConsumer listens to payment events and triggers booking completion.
type PaymentEventConsumer struct {
	consumer *kafka.Consumer
	service  *application.BookingService
	logger   *zap.Logger
}

// NewPaymentEventConsumer creates a new PaymentEventConsumer.
func NewPaymentEventConsumer(
	brokers []string,
	groupID string,
	service *application.BookingService,
	logger *zap.Logger,
) *PaymentEventConsumer {
	consumer := kafka.NewConsumer(brokers, groupID, events.TopicPaymentEvents, logger)
	return &PaymentEventConsumer{
		consumer: consumer,
		service:  service,
		logger:   logger,
	}
}

// Start begins consuming payment events. This blocks until the context is cancelled.
func (c *PaymentEventConsumer) Start(ctx context.Context) error {
	return c.consumer.Consume(ctx, c.handleMessage)
}

// Close closes the underlying Kafka consumer.
func (c *PaymentEventConsumer) Close() error {
	return c.consumer.Close()
}

func (c *PaymentEventConsumer) handleMessage(ctx context.Context, msg kafkago.Message) error {
	var cloudEvent kafka.CloudEvent
	if err := json.Unmarshal(msg.Value, &cloudEvent); err != nil {
		c.logger.Error("failed to parse cloud event from payment topic",
			zap.Error(err),
			zap.String("raw", string(msg.Value)),
		)
		return nil // Don't retry malformed messages
	}

	switch cloudEvent.Type {
	case events.PaymentEscrowReleased:
		return c.handleEscrowReleased(ctx, cloudEvent)
	default:
		c.logger.Debug("ignoring unhandled payment event type",
			zap.String("type", cloudEvent.Type),
		)
		return nil
	}
}

func (c *PaymentEventConsumer) handleEscrowReleased(ctx context.Context, cloudEvent kafka.CloudEvent) error {
	var evt events.EscrowReleasedEvent
	if err := cloudEvent.ParseData(&evt); err != nil {
		c.logger.Error("failed to parse EscrowReleasedEvent data",
			zap.Error(err),
		)
		return nil // Don't retry malformed data
	}

	c.logger.Info("processing escrow released event",
		zap.String("booking_id", evt.BookingID.String()),
		zap.String("payment_id", evt.PaymentID.String()),
	)

	_, err := c.service.CompleteBooking(ctx, evt.BookingID)
	if err != nil {
		c.logger.Error("failed to complete booking after escrow release",
			zap.String("booking_id", evt.BookingID.String()),
			zap.Error(err),
		)
		return err
	}

	c.logger.Info("booking completed after escrow release",
		zap.String("booking_id", evt.BookingID.String()),
	)
	return nil
}
