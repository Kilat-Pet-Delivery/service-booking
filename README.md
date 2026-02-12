# service-booking

Booking lifecycle management service with state machine orchestration and dynamic pricing.

## Description

This service manages the complete booking lifecycle for pet transport requests. It implements a robust state machine to handle booking transitions, calculates pricing based on distance and pet specifications, and publishes events to coordinate with other services.

## Features

- Booking creation with pet specifications
- State machine for booking lifecycle management
- Dynamic pricing calculation strategy
- Distance-based fare computation
- Pet size and special requirement handling
- Kafka event publishing for state changes
- Compensating transaction support

## API Endpoints

| Method | Endpoint                      | Access        | Description                    |
|--------|-------------------------------|---------------|--------------------------------|
| POST   | /api/v1/bookings              | Owner         | Create new booking             |
| GET    | /api/v1/bookings              | Owner/Runner  | List bookings                  |
| GET    | /api/v1/bookings/:id          | Owner/Runner  | Get booking details            |
| POST   | /api/v1/bookings/:id/accept   | Runner        | Accept booking                 |
| POST   | /api/v1/bookings/:id/pickup   | Runner        | Mark pet picked up             |
| POST   | /api/v1/bookings/:id/deliver  | Runner        | Mark pet delivered             |
| POST   | /api/v1/bookings/:id/confirm  | Owner         | Confirm delivery               |
| POST   | /api/v1/bookings/:id/cancel   | Owner/Runner  | Cancel booking                 |

## State Machine

Booking states: `requested` → `accepted` → `in_progress` → `delivered` → `completed`

Alternative paths: Any state → `cancelled`

## Kafka Integration

**Events Published:**
- booking.created
- booking.accepted
- booking.pickup_confirmed
- booking.delivery_confirmed
- booking.completed
- booking.cancelled

**Events Consumed:**
- payment.escrow_released

## Configuration

The service requires the following environment variables:

```
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=booking_db
SERVICE_PORT=8001
KAFKA_BROKERS=localhost:9092
KAFKA_TOPIC_PREFIX=kilat-pet-runner
BASE_FARE=10.0
PRICE_PER_KM=2.5
```

## Tech Stack

- **Language**: Go 1.24
- **Web Framework**: Gin
- **ORM**: GORM
- **Database**: PostgreSQL
- **Message Queue**: Kafka (shopify/sarama)

## Running the Service

```bash
# Install dependencies
go mod download

# Run migrations
go run cmd/migrate/main.go

# Start the service
go run cmd/server/main.go
```

The service will start on port 8001.

## Database Schema

- **bookings**: Core booking table with state tracking
- **pets**: Pet specifications for each booking
- **pricing**: Calculated pricing breakdown
