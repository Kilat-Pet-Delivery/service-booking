//go:build integration

package main_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Kilat-Pet-Delivery/lib-common/kafka"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/application"
	bookingDomain "github.com/Kilat-Pet-Delivery/service-booking/internal/domain/booking"
	bookingEvents "github.com/Kilat-Pet-Delivery/service-booking/internal/events"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/repository"
	"net"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	kafkamodule "github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// testInfra holds shared test infrastructure.
type testInfra struct {
	DB           *gorm.DB
	KafkaBrokers []string
	Cleanup      func()
}

// bookingStack holds wired-up booking service components.
type bookingStack struct {
	Service        *application.BookingService
	Consumer       *bookingEvents.PaymentEventConsumer
	CleanupProducer func()
}

// setupContainers starts PostgreSQL and Kafka testcontainers and returns a connected GORM DB.
func setupContainers(t *testing.T) *testInfra {
	t.Helper()
	ctx := context.Background()

	// Start PostgreSQL (PostGIS) container with log-based wait strategy.
	pgReq := testcontainers.ContainerRequest{
		Image:        "postgis/postgis:16-3.4-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "test_booking",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}
	pgContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: pgReq,
		Started:          true,
	})
	require.NoError(t, err, "failed to start PostgreSQL container")

	pgHost, err := pgContainer.Host(ctx)
	require.NoError(t, err)
	pgPort, err := pgContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	dsn := fmt.Sprintf("host=%s port=%s user=test password=test dbname=test_booking sslmode=disable", pgHost, pgPort.Port())

	// Poll until GORM can actually connect and ping.
	var db *gorm.DB
	require.Eventually(t, func() bool {
		var err error
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			return false
		}
		sqlDB, err := db.DB()
		if err != nil {
			return false
		}
		return sqlDB.Ping() == nil
	}, 30*time.Second, 1*time.Second, "PostgreSQL not ready for connections")

	// Enable uuid-ossp and auto-migrate.
	require.NoError(t, db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`).Error)
	require.NoError(t, db.AutoMigrate(&repository.BookingModel{}))

	// Start Kafka container using confluent-local (supports KRaft natively).
	kafkaContainer, err := kafkamodule.Run(ctx, "confluentinc/confluent-local:7.5.0")
	require.NoError(t, err, "failed to start Kafka container")

	kafkaBrokers, err := kafkaContainer.Brokers(ctx)
	require.NoError(t, err, "failed to get Kafka brokers")

	// Pre-create required topics.
	createTopics(t, kafkaBrokers, "booking.events", "payment.events")

	cleanup := func() {
		if err := kafkaContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate Kafka container: %v", err)
		}
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate PostgreSQL container: %v", err)
		}
	}

	return &testInfra{
		DB:           db,
		KafkaBrokers: kafkaBrokers,
		Cleanup:      cleanup,
	}
}

// setupBookingStack wires up the full booking service stack.
func setupBookingStack(t *testing.T, db *gorm.DB, brokers []string) *bookingStack {
	t.Helper()
	logger, _ := zap.NewDevelopment()

	bookingRepo := repository.NewGormBookingRepository(db)
	pricing := bookingDomain.NewStandardPricingStrategy()
	producer := kafka.NewProducer(brokers, logger)
	bookingSvc := application.NewBookingService(bookingRepo, pricing, producer, logger)

	groupID := fmt.Sprintf("test-booking-%s", uuid.New().String()[:8])
	consumer := bookingEvents.NewPaymentEventConsumer(brokers, groupID, bookingSvc, logger)

	return &bookingStack{
		Service:         bookingSvc,
		Consumer:        consumer,
		CleanupProducer: func() { _ = producer.Close() },
	}
}

// seedBookingInDeliveredState inserts a booking in "delivered" state for testing.
func seedBookingInDeliveredState(t *testing.T, db *gorm.DB, bookingID, ownerID, runnerID uuid.UUID) {
	t.Helper()
	now := time.Now().UTC()
	delivered := now.Add(-5 * time.Minute)
	pickedUp := now.Add(-30 * time.Minute)

	petSpec, _ := json.Marshal(map[string]interface{}{
		"pet_type":  "cat",
		"name":      "TestCat",
		"weight_kg": 4.0,
	})
	crateReq, _ := json.Marshal(map[string]interface{}{
		"minimum_size":            "small",
		"needs_ventilation":       true,
		"minimum_weight_capacity": 4.8,
	})
	pickup, _ := json.Marshal(map[string]interface{}{
		"line1": "123 Test St", "city": "KL", "state": "WP",
		"country": "MY", "latitude": 3.139, "longitude": 101.6869,
	})
	dropoff, _ := json.Marshal(map[string]interface{}{
		"line1": "456 Test Ave", "city": "KL", "state": "WP",
		"country": "MY", "latitude": 3.15, "longitude": 101.71,
	})

	model := repository.BookingModel{
		ID:                  bookingID,
		BookingNumber:       fmt.Sprintf("BK-INT%s", uuid.New().String()[:6]),
		OwnerID:             ownerID,
		RunnerID:            &runnerID,
		Status:              "delivered",
		PetSpec:             petSpec,
		CrateRequirement:    crateReq,
		PickupAddress:       pickup,
		DropoffAddress:      dropoff,
		EstimatedPriceCents: 150000,
		Currency:            "MYR",
		PickedUpAt:          &pickedUp,
		DeliveredAt:         &delivered,
		Notes:               "integration test",
		Version:             4,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	require.NoError(t, db.Create(&model).Error, "failed to seed booking")
}

// publishTestEvent publishes a CloudEvent to Kafka.
func publishTestEvent(t *testing.T, brokers []string, topic, source, eventType string, data interface{}) {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	producer := kafka.NewProducer(brokers, logger)
	defer func() { _ = producer.Close() }()

	ce, err := kafka.NewCloudEvent(source, eventType, data)
	require.NoError(t, err, "failed to create cloud event")

	err = producer.PublishEvent(context.Background(), topic, ce)
	require.NoError(t, err, "failed to publish event")
}

// waitForBookingStatus polls the bookings table until the status matches.
func waitForBookingStatus(t *testing.T, db *gorm.DB, bookingID uuid.UUID, expectedStatus string, timeout time.Duration) repository.BookingModel {
	t.Helper()
	var result repository.BookingModel
	require.Eventually(t, func() bool {
		var model repository.BookingModel
		err := db.Where("id = ?", bookingID).First(&model).Error
		if err != nil {
			return false
		}
		if model.Status == expectedStatus {
			result = model
			return true
		}
		return false
	}, timeout, 200*time.Millisecond, "booking did not transition to %s", expectedStatus)
	return result
}

// consumeOneEvent reads from a Kafka topic until it finds an event of the expected type.
func consumeOneEvent(t *testing.T, brokers []string, topic, expectedType string, timeout time.Duration) kafka.CloudEvent {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	groupID := fmt.Sprintf("test-assert-%s", uuid.New().String()[:8])
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     brokers,
		GroupID:     groupID,
		Topic:       topic,
		MinBytes:    1,
		MaxBytes:    10e6,
		StartOffset: kafkago.FirstOffset,
	})
	defer func() { _ = reader.Close() }()

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				t.Fatalf("timed out waiting for event type %q on topic %q", expectedType, topic)
			}
			continue
		}
		ce, err := kafka.ParseCloudEvent(msg.Value)
		if err != nil {
			continue
		}
		if ce.Type == expectedType {
			return ce
		}
	}
}

// createTopics pre-creates Kafka topics so producers don't fail with "Unknown Topic".
func createTopics(t *testing.T, brokers []string, topics ...string) {
	t.Helper()
	conn, err := kafkago.Dial("tcp", brokers[0])
	require.NoError(t, err, "failed to dial Kafka for topic creation")
	defer conn.Close()

	controller, err := conn.Controller()
	require.NoError(t, err, "failed to get Kafka controller")

	controllerConn, err := kafkago.Dial("tcp", net.JoinHostPort(controller.Host, fmt.Sprintf("%d", controller.Port)))
	require.NoError(t, err, "failed to connect to Kafka controller")
	defer controllerConn.Close()

	topicConfigs := make([]kafkago.TopicConfig, len(topics))
	for i, topic := range topics {
		topicConfigs[i] = kafkago.TopicConfig{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		}
	}
	err = controllerConn.CreateTopics(topicConfigs...)
	require.NoError(t, err, "failed to create Kafka topics")

	// Give Kafka a moment to propagate topic metadata.
	time.Sleep(1 * time.Second)
}
