# OCPP 2.0.1 Message Flow Documentation

## Overview

This document describes the complete message flow for OCPP 2.0.1 protocol in the current implementation, using **Transaction Start** as a practical example.

---

## Architecture Components

### Layer 1: Transport (WebSocket)
- `server/server.go` - WebSocket connection management
- `server/pool.go` - Connection pool for all charge points

### Layer 2: Protocol Routing
- `server/central_system.go` - Main router and message handler
- Protocol version detection and routing

### Layer 3: Message Handling
- `ocpp/v201/handlers/handler.go` - OCPP 2.0.1 message router
- `server/v201_handlers.go` - Business logic handlers

### Layer 4: Protocol Adaptation
- `server/protocol_adapter.go` - Version conversion layer
- Converts OCPP 2.0.1 types ↔ Internal entities

### Layer 5: Business Logic
- `server/system_handler.go` - Core business logic
- `entity/transaction.go` - Internal transaction entity
- `billing/billing_service.go` - Billing calculations

### Layer 6: Persistence
- `internal/mongo.go` - Database operations
- Multi-version data storage

---

## Transaction Start Flow (OCPP 2.0.1)

### Step 1: WebSocket Connection Establishment

```
Charge Point (OCPP 2.0.1)
    |
    | HTTP Upgrade Request
    | Sec-WebSocket-Protocol: ocpp2.0.1
    |
    v
server.go: ServeWS()
    |
    | - Negotiates subprotocol
    | - Creates WebSocket with protocol = OCPP201
    | - Stores in pool
    |
    v
Pool: register channel
    |
    v
connections.Store(chargePointId, OCPP201)
```

**Files**: `server/server.go:202-266`, `server/central_system.go:97-98`

**Key Code**:
```go
// server/server.go:246-257
protocol := common.ParseProtocolVersion(requestedProto)
ws := WebSocket{
    conn:     conn,
    id:       chargePointId,
    protocol: protocol,  // OCPP201
    ...
}
```

---

### Step 2: User Authorization (Pre-Transaction)

```
Charge Point
    |
    | WebSocket Message:
    | [2, "unique-id", "Authorize", {
    |   "idToken": {"idToken": "AABBCCDD", "type": "ISO14443"}
    | }]
    |
    v
central_system.go: handleIncomingMessage()
    |
    | - protocol = ws.GetProtocol() → OCPP201
    | - connections.Store(chargePointId, protocol)
    |
    | IF routingEnabled == true:
    |
    v
handleIncomingMessageVersionAware()
    |
    | - ParseRequestVersionAware(message, protocol, featureRegistry)
    | - Creates AuthorizeRequest from payload
    |
    v
routeOCPP201Request(chargePointId, "Authorize", request)
    |
    v
v201Handlers.OnAuthorize()
    |
    | 1. Convert IdToken → IdTag
    |    protocolAdapter.IdToken201ToIdTag(&request.IdToken)
    |    → "AABBCCDD"
    |
    | 2. Call business logic
    |    systemHandler.getUserTag(idTag)
    |    systemHandler.authorize(locationId, evseId, idTag)
    |
    | 3. Build response
    |    AuthorizeResponse{
    |        IdTokenInfo: {Status: Accepted}
    |    }
    |
    v
Response sent: [3, "unique-id", {"idTokenInfo": {"status": "Accepted"}}]
```

**Files**:
- `server/central_system.go:93-127` - Message handling
- `server/central_system.go:179-229` - Version-aware routing
- `server/central_system.go:264-288` - OCPP201 routing
- `server/v201_handlers.go:136-195` - Authorization handler
- `server/protocol_adapter.go:26-33` - IdToken conversion

---

### Step 3: Transaction Start Event

```
Charge Point
    |
    | WebSocket Message:
    | [2, "tx-start-id", "TransactionEvent", {
    |   "eventType": "Started",
    |   "timestamp": "2025-11-07T18:00:00Z",
    |   "triggerReason": "Authorized",
    |   "seqNo": 0,
    |   "transactionInfo": {
    |     "transactionId": "CP001-TX-001"
    |   },
    |   "idToken": {"idToken": "AABBCCDD", "type": "ISO14443"},
    |   "evse": {"id": 1, "connectorId": 1},
    |   "meterValue": [{
    |     "timestamp": "2025-11-07T18:00:00Z",
    |     "sampledValue": [{
    |       "value": 1000.0,
    |       "measurand": "Energy.Active.Import.Register",
    |       "context": "Transaction.Begin"
    |     }]
    |   }]
    | }]
    |
    v
central_system.go: handleIncomingMessage()
    |
    v
handleIncomingMessageVersionAware()
    |
    | - Identifies protocol: OCPP201
    | - Feature name: "TransactionEvent"
    | - Unmarshals into TransactionEventRequest
    |
    v
routeOCPP201Request("CP001", "TransactionEvent", request)
    |
    v
v201Handlers.OnTransactionEvent("CP001", request)
```

**Files**: `server/central_system.go:282`, `server/v201_handlers.go:211-382`

---

### Step 4: Protocol Adaptation (2.0.1 → Internal)

```
v201Handlers.OnTransactionEvent()
    |
    | Convert OCPP 2.0.1 types to internal entities:
    |
    v
protocolAdapter.TransactionEventToEntity(
    eventType:      Started,
    transactionInfo: {transactionId: "CP001-TX-001"},
    idToken:        {idToken: "AABBCCDD", type: "ISO14443"},
    evse:           {id: 1, connectorId: 1},
    meterValues:    [{...}],
    timestamp:      2025-11-07T18:00:00Z,
    chargePointId:  "CP001"
)
    |
    | Creates internal Transaction entity:
    |
    v
transaction := &entity.Transaction{
    ChargePointId:   "CP001",
    ConnectorId:     1,              // From EVSE.connectorId
    IdTag:           "AABBCCDD",     // From IdToken.idToken
    TimeStart:       2025-11-07T18:00:00Z,
    MeterStart:      1000,           // From meterValue
    ProtocolVersion: "ocpp2.0.1",   // NEW: tracks protocol
    EvseId:          &1,             // NEW: OCPP 2.0.1 EVSE ID
    Metadata: {                      // NEW: flexible storage
        "ocpp201_transaction_id": "CP001-TX-001",
        "trigger_reason": "Authorized"
    }
}
```

**Files**:
- `server/protocol_adapter.go:89-177` - TransactionEvent conversion
- `entity/transaction.go:7-80` - Transaction entity with multi-version fields

**Key Conversions**:
1. **IdToken → IdTag**: `IdToken.idToken` field extracted as string
2. **EVSE → ConnectorId**: EVSE hierarchy flattened to connector ID
3. **MeterValue → MeterStart**: First energy reading extracted
4. **Metadata Storage**: OCPP 2.0.1 specific data stored in flexible map

---

### Step 5: Business Logic Processing

```
v201Handlers.OnTransactionEvent() [continued]
    |
    | Get connector from charge point state:
    |
    v
systemHandler.getChargePoint("CP001")
    |
    v
connector = getConnectorByEvseAndConnectorId(state, evseId=1, connectorId=1)
    |
    | Update connector EVSE ID (for 2.0.1):
    |
    v
updateConnectorEvseId(connector, &1)
    |
    | Process transaction start:
    |
    v
switch request.EventType {
case TransactionEventStarted:
    |
    | 1. Initialize transaction
    v
    transaction.Init()  // Sets timestamps, status
    systemHandler.setTransactionProtocolVersion(transaction, "CP001")

    | 2. Get user information
    v
    userTag := getUserTag("AABBCCDD")
    transaction.UserTag = userTag
    transaction.Username = userTag.Username

    | 3. Assign transaction ID
    v
    newTransactionId++  // Global counter
    transaction.Id = newTransactionId

    | 4. Call billing service
    v
    billing.OnTransactionStart(transaction)
        |
        | - Selects PaymentPlan
        | - Initializes pricing
        | - Records start in CDR

    | 5. Save to database
    v
    database.AddTransaction(transaction)

    | 6. Update connector state
    v
    connector.CurrentTransactionId = transaction.Id
    database.UpdateConnector(connector)

    | 7. Register in charge point state
    v
    state.registerTransaction(transaction.Id)
    updateActiveTransactionsCounter()

    | 8. Notify event listeners
    v
    notifyEventListeners(TransactionStart, &EventMessage{
        ChargePointId: "CP001",
        ConnectorId:   1,
        TransactionId: newTransactionId,
        Username:      userTag.Username,
        IdTag:         "AABBCCDD",
        Time:          transaction.TimeStart
    })
        |
        | → Telegram bot notification
        | → OCPI notification (roaming)
        | → Metrics update
}
```

**Files**:
- `server/v201_handlers.go:258-307` - Transaction start handling
- `server/system_handler.go:242-304` - User tag and transaction logic
- `billing/affleck.go:39-68` - Billing service integration

---

### Step 6: Response Generation

```
v201Handlers.OnTransactionEvent()
    |
    | Create response:
    |
    v
response := &transactions.TransactionEventResponse{
    // Optional fields - can include:
    // - TotalCost: calculated cost so far
    // - ChargingPriority: for smart charging
    // - IdTokenInfo: updated authorization status
    // - UpdatedPersonalMessage: display message for user
}
    |
    v
return response, nil
    |
    v
central_system.go: routeOCPP201Request() returns
    |
    v
handleIncomingMessageVersionAware() receives response
    |
    | - Checks if WebSocket still open
    | - Marshals response to JSON
    |
    v
server.SendResponse(ws, response)
    |
    | Sends WebSocket message:
    | [3, "tx-start-id", {}]
    |
    v
Charge Point receives confirmation
```

**Files**: `server/central_system.go:210-228`, `server/server.go:SendResponse`

---

### Step 7: Database Persistence

```
MongoDB Collections Updated:

1. transactions:
   {
     "_id": ObjectId("..."),
     "id": 12345,
     "charge_point_id": "CP001",
     "connector_id": 1,
     "id_tag": "AABBCCDD",
     "username": "john.doe",
     "time_start": ISODate("2025-11-07T18:00:00Z"),
     "meter_start": 1000,
     "protocol_version": "ocpp2.0.1",    // NEW
     "evse_id": 1,                        // NEW
     "metadata": {                        // NEW
       "ocpp201_transaction_id": "CP001-TX-001",
       "trigger_reason": "Authorized"
     },
     "is_finished": false
   }

2. connectors:
   {
     "id": 1,
     "charge_point_id": "CP001",
     "evse_id": 1,                        // NEW
     "status": "Occupied",
     "current_transaction_id": 12345,
     "status_time": ISODate("2025-11-07T18:00:00Z")
   }

3. charge_points:
   {
     "charge_point_id": "CP001",
     "protocol_version": "ocpp2.0.1",    // NEW
     "device_model": {},                  // NEW
     "last_heartbeat": ISODate("...")
   }
```

**Files**:
- `entity/transaction.go` - Transaction entity structure
- `entity/connector.go` - Connector with EvseId
- `internal/mongo.go` - Database operations
- `internal/migrations.go` - Migration that added new fields

---

## Transaction Update Flow (Meter Values)

```
Charge Point sends periodic meter values:

[2, "meter-id", "TransactionEvent", {
  "eventType": "Updated",
  "triggerReason": "MeterValuePeriodic",
  "seqNo": 5,
  "transactionInfo": {"transactionId": "CP001-TX-001"},
  "meterValue": [{
    "timestamp": "2025-11-07T18:05:00Z",
    "sampledValue": [{
      "value": 5000.0,
      "measurand": "Energy.Active.Import.Register"
    }, {
      "value": 7200.0,
      "measurand": "Power.Active.Import"
    }]
  }]
}]
    |
    v
OnTransactionEvent() with EventType=Updated
    |
    | 1. Find existing transaction
    v
    existingTx := database.GetTransaction(transaction.Id)

    | 2. Process meter values
    v
    for each meterValue in request.MeterValue:
        tm := protocolAdapter.MeterValue201ToTransactionMeter(meterValue, txId)

        // Converts to internal meter value:
        tm = {
            Id:           12345,
            Time:         2025-11-07T18:05:00Z,
            Value:        5000,
            PowerActive:  7200,
            Measurand:    "Energy.Active.Import.Register",
            Unit:         "Wh"
        }

        | 3. Calculate pricing
        v
        billing.OnMeterValue(existingTx, tm)
            |
            | - Updates energy consumption
            | - Calculates current cost
            | - Records in CDR

        | 4. Save meter value
        v
        database.AddTransactionMeterValue(tm)
```

**Files**:
- `server/v201_handlers.go:309-328` - Transaction update handling
- `server/protocol_adapter.go:179-256` - MeterValue conversion

---

## Transaction Stop Flow

```
Charge Point
    |
    | [2, "stop-id", "TransactionEvent", {
    |   "eventType": "Ended",
    |   "triggerReason": "StopAuthorized",
    |   "transactionInfo": {
    |     "transactionId": "CP001-TX-001",
    |     "stoppedReason": "Local"
    |   },
    |   "meterValue": [{...final reading...}]
    | }]
    |
    v
OnTransactionEvent() with EventType=Ended
    |
    | 1. Get existing transaction
    v
    existingTx := database.GetTransaction(transaction.Id)

    | 2. Update transaction
    v
    existingTx.Lock()
    existingTx.IsFinished = true
    existingTx.TimeStop = timestamp
    existingTx.MeterStop = finalMeterValue
    existingTx.Reason = stoppedReason

    | 3. Call billing
    v
    billing.OnTransactionFinished(existingTx)
        |
        | - Calculates final cost
        | - Closes CDR
        | - Prepares invoice

    | 4. Update database
    v
    database.UpdateTransaction(existingTx)

    | 5. Clear connector
    v
    connector.CurrentTransactionId = -1
    database.UpdateConnector(connector)

    | 6. Unregister transaction
    v
    state.unregisterTransaction(txId)
    updateActiveTransactionsCounter()

    | 7. Notify listeners
    v
    notifyEventListeners(TransactionStop, {...})

    | 8. Trigger payment
    v
    payment.TransactionPayment(existingTx)  // Async
```

**Files**: `server/v201_handlers.go:330-380`

---

## Key Differences: OCPP 1.6J vs 2.0.1

### Message Flow Comparison

| Aspect | OCPP 1.6J | OCPP 2.0.1 |
|--------|-----------|------------|
| **Transaction Start** | StartTransaction | TransactionEvent (Started) |
| **Meter Values** | MeterValues | TransactionEvent (Updated) |
| **Transaction Stop** | StopTransaction | TransactionEvent (Ended) |
| **Authorization** | IdTag (string) | IdToken (object with type) |
| **Connector** | ConnectorId (flat) | EVSE → Connector (hierarchical) |
| **Status** | StatusNotification | StatusNotification (different structure) |

### Data Storage

**OCPP 1.6J Transaction**:
```json
{
  "id": 12345,
  "charge_point_id": "CP001",
  "connector_id": 1,
  "id_tag": "AABBCCDD"
}
```

**OCPP 2.0.1 Transaction**:
```json
{
  "id": 12345,
  "charge_point_id": "CP001",
  "connector_id": 1,
  "id_tag": "AABBCCDD",
  "protocol_version": "ocpp2.0.1",
  "evse_id": 1,
  "metadata": {
    "ocpp201_transaction_id": "CP001-TX-001"
  }
}
```

---

## Protocol Adapter Role

The `ProtocolAdapter` acts as a translation layer:

### OCPP 2.0.1 → Internal
```go
// IdToken → IdTag
protocolAdapter.IdToken201ToIdTag(&IdToken{
    IdToken: "AABBCCDD",
    Type:    "ISO14443"
})
→ "AABBCCDD"

// EVSE → ConnectorId
protocolAdapter.EvseToConnectorId(&EVSE{
    Id:          1,
    ConnectorId: &1
})
→ 1

// TransactionEvent → Transaction Entity
protocolAdapter.TransactionEventToEntity(...)
→ &entity.Transaction{...}
```

### Internal → OCPP 2.0.1
```go
// IdTag → IdToken
protocolAdapter.IdTagToIdToken201("AABBCCDD")
→ &IdToken{IdToken: "AABBCCDD", Type: "ISO14443"}

// ConnectorId → EVSE
protocolAdapter.ConnectorIdToEvse(1, &1)
→ &EVSE{Id: 1, ConnectorId: &1}
```

**File**: `server/protocol_adapter.go`

---

## Version-Aware Routing Decision Flow

```
Message arrives
    |
    v
Is routingEnabled == true?
    |
    +--NO--→ Legacy routing (OCPP 1.6 only)
    |
    +--YES-→ handleIncomingMessageVersionAware()
                |
                v
            protocol = ws.GetProtocol()
                |
                v
            switch protocol {
            case OCPP16:
                → routeOCPP16Request()
                → coreHandler.OnStartTransaction()

            case OCPP201:
                → routeOCPP201Request()
                → v201Handlers.OnTransactionEvent()

            case OCPP21:
                → (Future implementation)
            }
```

**File**: `server/central_system.go:125-127`, `main.go:44` (EnableVersionAwareRouting)

---

## Configuration

### Enabling OCPP 2.0.1 Support

**File**: `main.go`
```go
centralSystem, err := server.NewCentralSystem(conf)
if err != nil {
    log.Println("central system initialization failed", err)
    return
}

// Enable version-aware routing to support OCPP 2.0.1
centralSystem.EnableVersionAwareRouting()

centralSystem.Start()
```

### Supported Subprotocols

**File**: `server/central_system.go:489-490`
```go
wsServer.AddSupportedSupProtocol("ocpp1.6")      // OCPP 1.6J
wsServer.AddSupportedSupProtocol("ocpp2.0.1")    // OCPP 2.0.1
```

---

## Error Handling

### Protocol Mismatch
```
1. Charge point connects with unsupported protocol
   → WebSocket handshake fails
   → Connection rejected

2. Invalid message format
   → ParseRequestVersionAware() returns error
   → Error response sent to charge point

3. Unknown feature
   → routeOCPP201Request() returns error
   → "feature not supported" response
```

### Transaction Errors
```
1. Transaction not found (on Update/Ended)
   → Error logged
   → Empty response sent (transaction may have expired)

2. Connector not found
   → Error returned
   → Charge point receives error response

3. Database failure
   → Transaction stored in memory
   → Retry on next operation
```

---

## Performance Considerations

### Connection Tracking
- Protocol version stored once per connection
- O(1) lookup via sync.Map
- No protocol detection on every message

### Message Parsing
- Type registry for fast unmarshaling
- Pre-allocated structs for common messages
- Zero-copy JSON parsing where possible

### Database Operations
- Batch meter value inserts
- Indexed queries on protocol_version
- Async payment processing

---

## Testing

### Unit Tests
- **64 tests** covering all OCPP 2.0.1 message types
- Protocol adapter conversion tests
- Serialization/deserialization tests

### Integration Tests
```bash
# Test OCPP 2.0.1 transaction flow
go test ./server -run "TestOCPP201Transaction" -v

# Test protocol adapter
go test ./server -run "TestProtocolAdapter" -v

# Test message types
go test ./ocpp/v201/... -v
```

**Files**: See `PHASE2_TASK2.8_TEST_REPORT.md` for full test coverage

---

## Debugging

### Enable Debug Logging
**File**: `config.yml`
```yaml
is_debug: true
```

### Trace Message Flow
1. WebSocket connection: `server/server.go:212`
2. Protocol detection: `server/server.go:246`
3. Message routing: `server/central_system.go:125`
4. Handler execution: `server/v201_handlers.go:211`
5. Database save: `internal/mongo.go`

### Common Issues

**Issue**: OCPP 2.0.1 messages rejected
- **Cause**: Version-aware routing not enabled
- **Fix**: Call `centralSystem.EnableVersionAwareRouting()` in main.go

**Issue**: Transaction not found on TransactionEvent (Updated)
- **Cause**: Transaction ID mismatch or expired session
- **Fix**: Check metadata mapping for OCPP 2.0.1 transaction IDs

**Issue**: EVSE information lost
- **Cause**: Connector created before EVSE ID set
- **Fix**: Migration adds evse_id field, update via updateConnectorEvseId()

---

## Summary

The OCPP 2.0.1 implementation provides:

✅ **Full protocol support** for core transaction flow
✅ **Seamless version coexistence** with OCPP 1.6J
✅ **Protocol adapter** for transparent conversion
✅ **Version-aware routing** for correct handler selection
✅ **Multi-version persistence** in database
✅ **Backward compatibility** with existing 1.6J infrastructure

The architecture ensures that OCPP 1.6J and 2.0.1 charge points can operate simultaneously without interference, while sharing common business logic through the protocol adapter layer.
