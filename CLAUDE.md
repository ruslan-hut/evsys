# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Workflow Settings

- **Always run a build check after changing code** - run `go build ./...` once a logical set of edits is complete, and fix any compile errors before reporting done
- Run `go test ./...` when changes touch tested packages

## Project Overview

EVSYS is an electric vehicle charging central system implementing OCPP 1.6J protocol. It manages charging points, users, and charging sessions as part of the Wattbrews platform.

## Linked Projects

| Repository | Local path | Role |
|---|---|---|
| [evsys-back](https://github.com/ruslan-hut/evsys-back) | `~/projects/evsys-back` | Reads the same MongoDB, serves REST/WebSocket to all end-user clients |
| [evsys-front](https://github.com/ruslan-hut/evsys-front) | `~/projects/evsys-front` | Angular web app (operator/admin) |
| Wattbrews | `~/projects/Wattbrews` | Android app, Jetpack Compose + Kotlin |
| wattbrews-web | `~/projects/wattbrews-web` | Angular 21 web app |
| [Electrum](https://github.com/ruslan-hut/electrum) | `~/projects/electrum` | Payment system integration |

evsys-front, Wattbrews and wattbrews-web are all clients of evsys-back, not of
this service - but they render the data this service writes, so a field added
here is invisible to users until it has been carried through every hop below.
The Android app ships through the Play Store and old versions stay in use, so
evsys-back changes must be additive; see its CLAUDE.md.

**evsys and evsys-back share a database, not an API.** evsys writes the
`charge_points`, `connectors`, `transactions` and `meter_values` collections;
evsys-back decodes those same documents into its own structs. The two schemas
are duplicated by hand and Go's BSON decoder silently ignores fields it has no
struct member for, so a field added here simply never surfaces downstream and
nothing fails loudly.

### Rule: propagate entity changes downstream

When adding or renaming a field on an entity in `/entity`, follow it through
every layer before considering the change done:

1. `~/projects/evsys-back/entity/` - the mirrored struct, matching the bson tag
2. `~/projects/evsys-back/entity/charge_state.go` - the `ChargeState` DTO, if
   the field belongs to a transaction; the transaction detail endpoint returns
   this DTO rather than the entity, so a field missing here reaches no client
3. `~/projects/evsys-back/impl/database/mongo.go` - the DTO mapping, and the
   `$set` list in `UpdateChargePoint` if the field should be writable
4. `~/projects/evsys-front/src/app/models/` - the TypeScript model
5. The component template, if the value should actually be displayed

Write a migration under `/migrations` when existing documents need the field
backfilled. A missing field decodes as the zero value, and for a bool that
means a feature silently switches off on every pre-existing document.

## Build and Run Commands

**Build:**
```bash
go build -o evsys
```

**Run (development):**
```bash
# With config file
./evsys -conf=config.yml

# Standard production path
./evsys -conf=/etc/conf/config.yml
```

**Dependencies:**
```bash
go mod download
go mod tidy
```

**Tests:**
```bash
# unit tests only
go test -short ./...

# everything, including the MongoDB-backed integration tests
docker run -d --name evsys-test-mongo -p 27019:27017 mongo:7
MONGO_TEST_URI=mongodb://localhost:27019 go test -race ./...
```

Tests live in `server/`, `ocpp/v201` with its subpackages, and `internal/`.
Handler tests in `server/` drive a `SystemHandler` against stub
`internal.Database` implementations that embed the interface as a nil field, so
any method the handler calls unexpectedly panics rather than silently returning
a zero value.

The `internal/` tests hit a real MongoDB and are skipped unless
`MONGO_TEST_URI` is set; `-short` skips them too. They cover the
abandoned-transaction sweep and the migrations, both of which live in
aggregation pipelines rather than in Go, so a mock would test nothing.

When touching the sweep, the migrations or `OnStopTransaction`, verify the
change by mutation: break the fix on purpose and confirm a test fails. Several
of these guard orderings and time windows that pass by accident under a weaker
assertion.

## Configuration

Configuration is YAML-based (see README.md for full example). Key sections:
- `listen`: WebSocket server for OCPP charge point connections (default port 5000, path `/ws/:id`)
- `api`: REST API for external requests (default port 5001, endpoint `/api`)
- `mongo`: MongoDB connection (optional - system runs standalone if disabled)
- `payment`: External payment service integration (optional)
- `telegram`: Bot notifications (optional)
- `ocpi`: Roaming operations via OCPI protocol (optional)
- `metrics`: Prometheus metrics endpoint (optional)

Feature flags:
- `accept_unknown_tag`: Allow unregistered authorization tokens
- `accept_unknown_chp`: Allow unregistered charge points
- `is_debug`: Enable verbose logging

## Architecture

### Core Components

**Entry Point:** `main.go` initializes configuration, metrics server, and CentralSystem

**Package Structure:**
- `/server` - OCPP central system, WebSocket pool, API handlers, message routing
- `/ocpp` - OCPP 1.6J protocol messages organized by feature profiles (core, smartcharging, firmware, localauth, remotetrigger)
- `/entity` - Domain models (ChargePoint, User, Transaction, PaymentPlan, Tariff, etc.)
- `/internal` - Infrastructure interfaces and implementations (database, config, logging)
- `/billing` - Payment calculations, billing service, payment worker
- `/power` - Load balancing and power management across locations
- `/telegram` - Telegram bot integration for notifications
- `/ocpi` - OCPI protocol client for roaming operations
- `/metrics` - Prometheus metrics exposure
- `/utility` - Common helpers
- `/types` - Shared type definitions

### OCPP Protocol Implementation

**WebSocket Architecture:**
- Pool pattern manages all active charge point connections with register/unregister channels
- Each charge point connects via `/ws/:id` with OCPP 1.6 subprotocol negotiation
- Dedicated read/write goroutines per connection (see `server/client.go`)
- Bidirectional message flow with envelope-based routing

**Message Routing Flow:**
```
Incoming: ChargePoint → WebSocket → handleIncomingMessage → FeatureName switch → SystemHandler.On* methods
Outgoing: API/Trigger → CentralSystem.SendRequest → Pool → WebSocket send channel → ChargePoint
```

**Request/Response Pattern:**
- All OCPP features implement `ocpp.Request` and `ocpp.Response` interfaces
- Each feature has `GetFeatureName()` for routing
- API uses synchronous request/response with 10-second timeout via pending request tracking

**OCPP Feature Profiles:**
- **Core** (`ocpp/core`): BootNotification, Authorize, StartTransaction, StopTransaction, Heartbeat, MeterValues, StatusNotification, etc.
- **Smart Charging** (`ocpp/smartcharging`): SetChargingProfile, ClearChargingProfile, GetCompositeSchedule
- **Firmware** (`ocpp/firmware`): Diagnostics, FirmwareStatus updates
- **Remote Trigger** (`ocpp/remotetrigger`): TriggerMessage for proactive requests
- **Local Auth** (`ocpp/localauth`): SendLocalList, GetLocalListVersion

### Domain Model Relationships

```
Location
├── ChargePoint (EVSE)
│   ├── Connectors[]
│   └── Power limit enforcement
│
User
├── UserTag (RFID/authorization token)
└── PaymentMethod

Transaction (charging session)
├── ChargePointId, ConnectorId
├── IdTag → UserTag
├── Username → User
├── PaymentPlan (time-based pricing rules)
├── Tariff (OCPI-compliant pricing structure)
├── MeterValues[] (energy consumption samples)
├── PaymentOrders[] (billing records)
└── PaymentMethod

PaymentPlan
├── PricePerKwh, PricePerHour
└── StartTime/EndTime (time-of-day pricing)

Tariff (OCPI standard)
└── Elements[] → PriceComponents[]
```

**Key Relationships:**
- ChargePoint belongs to Location (for power limit management)
- Connector belongs to ChargePoint (tracks current transaction)
- Transaction links User, ChargePoint, Connector, and billing entities
- PaymentPlan can be user-specific or location-specific with time range validation

### Transaction Lifecycle

Implemented in `server/system_handler.go`:

1. **Authorization** (`OnAuthorize`):
   - Validates user tag via database or OCPI auth service
   - Respects `accept_unknown_tag` config for development/testing

2. **Start Transaction** (`OnStartTransaction`):
   - Generates transaction ID (UUID)
   - Validates connector availability
   - Assigns PaymentPlan (user-specific > location-specific > default, with time-range matching)
   - Calls `BillingService.OnTransactionStart`
   - Triggers power manager for load balancing
   - Sends events to listeners (Telegram, OCPI)

3. **Meter Values** (`OnMeterValues`):
   - Tracks energy consumption samples over time
   - Calculates running price via billing service
   - Updates transaction state in database

4. **Stop Transaction** (`OnStopTransaction`):
   - Calculates final billing amount
   - Calls `BillingService.OnTransactionFinished`
   - Triggers payment worker (async processing)
   - Updates connector status to available
   - Power manager rebalancing
   - Event notifications

**State Management:**
- `ChargePointState` struct tracks each charge point's status, connectors, and active transactions
- In-memory map with mutex synchronization for fast access
- Database persistence for durability (when enabled)

### Background Services

Multiple concurrent goroutines run throughout application lifecycle:

1. **WebSocket Server** - Listens for charge point connections on configured port
2. **API Server** - HTTP REST interface for external commands
3. **Metrics Server** - Prometheus endpoint (optional)
4. **Telegram Bot** - Three goroutines: updates pump, send pump, event pump
5. **Payment Worker** - 3-minute ticker checking for unbilled transactions, calls external payment API
6. **Power Manager** - Triggered on transaction events to enforce location power limits
7. **Pool Manager** - Handles WebSocket connection registration/unregistration
8. **Read/Write Pumps** - Dedicated goroutines per WebSocket connection

### Integration Points

**MongoDB** (`internal/mongo.go`):
- Interface-based design (`internal.Database`) for flexibility
- Collections: charge_points, connectors, transactions, users, user_tags, payment_plans, tariffs, meter_values, sys_log, errors_log
- Optional mode: system runs standalone without database
- Connection pooling with context-based operations

**Payment API** (`billing/payment.go`):
- External HTTP service for transaction payment processing
- Background worker with retry logic (exponential backoff)
- RESTful: `GET /pay/{transactionId}` with Bearer token authentication
- Async processing decoupled from transaction lifecycle

**Telegram Bot** (`telegram/bot.go`):
- Implements `internal.EventHandler` interface
- Event-driven notifications (transaction events, status changes, alerts)
- User subscription via `/start` and `/stop` commands
- Channel-based message queuing

**OCPI Client** (`ocpi/`):
- Outbound integration for roaming operations with partner networks
- Event listener for transaction notifications
- Authorization service for roaming users
- HTTP client with retry logic (3 attempts, exponential backoff)

**Prometheus Metrics** (`metrics/`):
- Separate HTTP server on configurable port
- Counters for active transactions per location
- Standard `/metrics` endpoint

### Architectural Patterns

- **Event-Driven**: `EventHandler` interface allows multiple listeners (Telegram, OCPI) to react without coupling
- **Interface Segregation**: Database, LogHandler, EventHandler, BillingService interfaces enable modularity
- **Pool Pattern**: WebSocket connection management with centralized hub
- **Command Pattern**: API requests map to OCPP commands via feature name routing
- **Strategy Pattern**: PaymentPlan selection based on user, time range, and location
- **Repository Pattern**: Database interface abstracts persistence
- **Concurrent Safe State**: Mutexes protect shared state (ChargePointState map, connector updates)

### Key Design Decisions

1. **In-Memory State with DB Persistence**: Fast access with optional durability
2. **Optional Components**: System runs standalone; DB, payment, notifications are optional
3. **Time-Based Pricing**: PaymentPlan supports dynamic pricing by time of day (StartTime/EndTime fields)
4. **Load Balancing**: Power manager enforces location-level power limits across all charge points
5. **Dual Pricing Models**: Legacy PaymentPlan (simple kWh/hour) + OCPI Tariff (complex) coexist for backward compatibility
6. **Async Billing**: Payment processing decoupled from transaction stop to avoid blocking

## API Usage

**Endpoint:** `POST http://<server>:5001/api`

**Request Structure:**
```json
{
  "charge_point_id": "Wallbox3",
  "connector_id": 0,
  "feature_name": "GetConfiguration",
  "payload": "AllowOfflineTxForUnknownId"
}
```

**Common Features:**
- `GetConfiguration` / `ChangeConfiguration` - Charge point configuration management
- `RemoteStartTransaction` / `RemoteStopTransaction` - Initiate/stop charging remotely
- `SetChargingProfile` / `ClearChargingProfile` - Smart charging control
- `GetDiagnostics` - Retrieve diagnostics logs
- `Reset` - Reboot charge point (Soft/Hard)
- `TriggerMessage` - Request specific OCPP messages
- `SendLocalList` - Update local authorization list
- `GetServerStatus` - Custom command (not OCPP standard) to list connected charge points

## Development Guidelines

### Working with OCPP Messages

When adding new OCPP features:
1. Define request/response structs in appropriate `/ocpp/*` package
2. Implement `GetFeatureName()` method for routing
3. Add handler method in `server/system_handler.go` (pattern: `On<FeatureName>`)
4. Add case in `handleIncomingMessage` switch statement
5. Update API request mapping if needed

### Working with Domain Entities

- Entity definitions in `/entity` package
- Database operations via `internal.Database` interface methods
- State updates require mutex locks (see `ChargePointState` usage)
- Always validate time ranges when working with PaymentPlan (use `IsInRange()` method)

### Working with Billing

- Payment plan selection logic in `server/system_handler.go:selectPaymentPlan()`
- Price calculations in `billing/billing_service.go`
- New tariff structures should follow OCPI 2.2.1 specification
- Payment processing is async via `billing/payment.go` worker

### Working with Load Balancing

- Power manager in `/power` package
- Triggered on transaction start/stop and system initialization
- Enforces `power_limit` at Location level
- Algorithm: proportional allocation based on max current across connectors
- Updates sent via `SetChargingProfile` OCPP command

### Adding Event Handlers

Implement `internal.EventHandler` interface:
```go
type EventHandler interface {
    OnEvent(event *entity.Event)
}
```

Register in `main.go` or `server/central_system.go` initialization.

### Error Handling and Logging

- Errors logged via `internal.LogHandler` interface
- Database errors saved to `errors_log` collection (when DB enabled)
- System events logged to `sys_log` collection
- Use `log.Printf` for console output in debug mode
