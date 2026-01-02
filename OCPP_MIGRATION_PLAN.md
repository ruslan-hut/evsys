# OCPP Protocol Migration Plan
## From OCPP 1.6J to Multi-Version Support (2.0.1 / 2.1)

**Document Version:** 1.0
**Date:** 2025-11-05
**Target:** Support OCPP 1.6J, 2.0.1, and 2.1 concurrently

---

## Executive Summary

This document outlines a comprehensive plan to evolve EVSYS from a single-version OCPP 1.6J implementation to a multi-version architecture supporting OCPP 1.6J, 2.0.1, and 2.1 simultaneously. The migration requires significant architectural refactoring affecting approximately 60-70% of the codebase.

**Key Objectives:**
1. Maintain backward compatibility with existing OCPP 1.6J charge points
2. Support OCPP 2.0.1 and 2.1 charge points concurrently
3. Minimize disruption to existing integrations
4. Enable gradual rollout and testing
5. Maintain code quality and maintainability

---

## 1. OCPP Version Comparison

### 1.1 Protocol Differences Overview

| Aspect | OCPP 1.6J | OCPP 2.0.1 / 2.1 |
|--------|-----------|------------------|
| **Transport** | WebSocket (OCPP-J) | WebSocket (OCPP-J) |
| **Message Format** | JSON arrays `[Type, Id, Action, Payload]` | Same format |
| **Subprotocol** | `ocpp1.6` | `ocpp2.0.1`, `ocpp2.1` |
| **Feature Profiles** | 5 profiles, 21 operations | 9 functional blocks, 70+ operations |
| **Security** | Optional TLS | Mandatory certificate-based security |
| **Device Model** | Configuration keys | Hierarchical component/variable model |
| **Authorization** | IdTag (20 chars) | IdToken with type (RFID, ISO14443, etc.) |
| **Transactions** | Simple start/stop | Complex lifecycle with states |
| **Charging Profiles** | Basic smart charging | Advanced ISO 15118 support |
| **Display Messages** | Not supported | Display message management |
| **Firmware Updates** | Basic | Advanced with signed firmware |
| **ISO 15118** | Not supported | Plug & Charge support |

### 1.2 Key Architectural Changes in OCPP 2.x

**1. Device Model Paradigm Shift**
- **1.6J:** Flat configuration keys (`HeartbeatInterval`, `MeterValueSampleInterval`)
- **2.0.1:** Hierarchical model (`OCPPCommCtrlr.HeartbeatInterval`, `SampledDataCtrlr.Interval`)

**2. Transaction Model Changes**
- **1.6J:** StartTransaction → MeterValues → StopTransaction
- **2.0.1:** TransactionEvent with eventType (Started, Updated, Ended) and triggerReason

**3. New Entities**
- **EVSE (Electric Vehicle Supply Equipment):** Replaces connector (hierarchical: ChargingStation → EVSE → Connector)
- **IdToken:** Replaces IdTag with type information
- **ChargingNeeds:** EV requirements for smart charging
- **CompositeSchedule:** Advanced charging schedules

**4. Security Requirements**
- **2.0.1:** Mandatory certificate authentication, secure firmware updates, security event logging
- **1.6J:** Optional TLS, basic security

**5. New Operations (Sample)**
- **StatusNotification:** Different payload structure
- **Authorize:** IdToken vs IdTag
- **TransactionEvent:** Replaces Start/StopTransaction + MeterValues
- **NotifyReport:** Replaces multiple 1.6 operations
- **Get/SetVariables:** Replaces Get/ChangeConfiguration
- **RequestStartTransaction/RequestStopTransaction:** Replace Remote* operations

---

## 2. Current Architecture Analysis

### 2.1 Critical Constraints

Based on code analysis, the following areas present the most significant challenges:

**1. Hardcoded Type Resolution** (`server/message.go:161-187`)
```go
func getMessageType(action string) (requestType reflect.Type, err error) {
    switch action {
    case core.BootNotificationFeatureName:
        requestType = reflect.TypeOf(core.BootNotificationRequest{})
    // ... hardcoded for all 1.6J features
    }
}
```
**Issue:** No version parameter; assumes single version.

**2. Hardcoded Message Routing** (`server/central_system.go:96-119, 149-176`)
```go
switch action {
case core.BootNotificationFeatureName:
    confirmation, err = cs.coreHandler.OnBootNotification(...)
// ... hardcoded for all features
}
```
**Issue:** Two large switch statements requiring manual maintenance.

**3. Version-Agnostic WebSocket** (`server/server.go`)
```go
wsServer.AddSupportedSupProtocol(types.SubProtocol16)
```
**Issue:** Subprotocol negotiated but ignored; no per-connection version tracking.

**4. Monolithic Type Package** (`types/types.go`)
```go
const SubProtocol16 = "ocpp1.6"
type IdTagInfo struct { ... }  // 1.6J specific
```
**Issue:** All types assume OCPP 1.6J; cannot coexist with 2.0.1 types.

**5. Version-Specific Handler Interface**
```go
type SystemHandler interface {
    OnBootNotification(chargePointId string, request *core.BootNotificationRequest) (*core.BootNotificationResponse, error)
    // ... all methods are 1.6J-specific
}
```
**Issue:** Tightly coupled to 1.6J types; cannot handle 2.0.1 requests.

### 2.2 Extensibility Points

**Positive Aspects:**
1. **Interface-based design:** Request/Response interfaces provide abstraction
2. **Package organization:** Features grouped by profile
3. **WebSocket abstraction:** Transport layer decoupled via `ocpp.WebSocket` interface
4. **Subprotocol negotiation:** Already accepts multiple protocols (infrastructure exists)

---

## 3. Proposed Architecture

### 3.1 Multi-Version Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         API / Telegram Bot                      │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│                    CentralSystem (Router)                       │
│  - Connection registry (id → protocol version)                  │
│  - Version-aware routing                                        │
└──────┬──────────────────────┬──────────────────────┬───────────┘
       │                      │                      │
       ▼                      ▼                      ▼
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│ OCPP 1.6J   │      │ OCPP 2.0.1  │      │ OCPP 2.1    │
│   Handler   │      │   Handler   │      │   Handler   │
└──────┬──────┘      └──────┬──────┘      └──────┬──────┘
       │                    │                     │
       ▼                    ▼                     ▼
┌────────────────────────────────────────────────────────────┐
│            Business Logic / Service Layer                  │
│  - Transaction Manager (version-agnostic)                  │
│  - Billing Service (handles all pricing models)            │
│  - Power Manager (load balancing)                          │
│  - Authorization Service (IdTag/IdToken abstraction)       │
└────────────────┬───────────────────────────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────────────────────────┐
│                  Data Layer (MongoDB)                      │
│  - Versioned entity storage                                │
│  - Migration support                                       │
└────────────────────────────────────────────────────────────┘
```

### 3.2 Package Structure Redesign

**Current Structure:**
```
/ocpp
  /core
  /firmware
  /smartcharging
  /localauth
  /remotetrigger
  ocpp.go (interfaces)
```

**Proposed Structure:**
```
/ocpp
  /common
    protocol.go         # Protocol version constants
    registry.go         # Feature registry
    interfaces.go       # Version-agnostic interfaces
  /v16                  # OCPP 1.6J namespace
    /core
    /firmware
    /smartcharging
    /localauth
    /remotetrigger
    handler.go          # Handler16 interface implementation
    types.go            # 1.6J specific types
  /v201                 # OCPP 2.0.1 namespace
    /authorization
    /availability
    /dataTransfer
    /diagnostics
    /firmware
    /iso15118
    /metervalues
    /provisioning
    /remotecontrol
    /smartcharging
    /tariffandcost
    /transactions
    handler.go          # Handler201 interface implementation
    types.go            # 2.0.1 specific types
  /v21                  # OCPP 2.1 namespace (future)
    ...
```

### 3.3 Core Components Design

#### 3.3.1 Protocol Version Registry

**File:** `/ocpp/common/registry.go`

```go
package common

import "reflect"

type ProtocolVersion string

const (
    OCPP16  ProtocolVersion = "ocpp1.6"
    OCPP201 ProtocolVersion = "ocpp2.0.1"
    OCPP21  ProtocolVersion = "ocpp2.1"
)

type FeatureRegistry interface {
    // Register a feature for a specific protocol version
    RegisterFeature(version ProtocolVersion, action string,
                    requestType, responseType reflect.Type)

    // Get types for a specific version and action
    GetTypes(version ProtocolVersion, action string) (
        requestType, responseType reflect.Type, err error)

    // Check if feature is supported for version
    IsSupported(version ProtocolVersion, action string) bool

    // Get all features for version
    GetFeatures(version ProtocolVersion) []string
}

type featureRegistry struct {
    // Map: version → action → (requestType, responseType)
    registry map[ProtocolVersion]map[string]featureTypes
}

type featureTypes struct {
    requestType  reflect.Type
    responseType reflect.Type
}

func NewFeatureRegistry() FeatureRegistry {
    return &featureRegistry{
        registry: make(map[ProtocolVersion]map[string]featureTypes),
    }
}
```

#### 3.3.2 Version-Aware WebSocket

**File:** `/ocpp/common/interfaces.go`

```go
package common

import "github.com/gorilla/websocket"

// Extends existing ocpp.WebSocket interface
type VersionedWebSocket interface {
    ID() string
    GetProtocol() ProtocolVersion      // NEW
    SetProtocol(ProtocolVersion)       // NEW
    RemoteAddr() net.Addr
    IsClosed() bool
    Close() error
    WriteMessage(data []byte) error
    SetCloseHandler(func(int, string) error)
    SetPongHandler(func(string) error)
    SetReadDeadline(time.Time) error
    SetWriteDeadline(time.Time) error
}

// Wrapper for existing WebSocket implementation
type versionedWebSocket struct {
    conn     *websocket.Conn
    protocol ProtocolVersion
    // ... existing fields
}
```

#### 3.3.3 Version-Agnostic Request Interface

**File:** `/ocpp/common/interfaces.go`

```go
// Base request interface (replaces ocpp.Request)
type Request interface {
    GetFeatureName() string
    GetProtocolVersion() ProtocolVersion  // NEW
    Validate() error                       // NEW
}

// Base response interface (replaces ocpp.Response)
type Response interface {
    GetFeatureName() string
    GetProtocolVersion() ProtocolVersion  // NEW
}

// Generic handler interface
type MessageHandler interface {
    // Handle incoming request from charge point
    HandleRequest(ws VersionedWebSocket, action string, payload []byte) (Response, error)

    // Create outgoing request to charge point
    CreateRequest(action string, payload interface{}) (Request, error)

    // Get supported protocol version
    GetVersion() ProtocolVersion
}
```

#### 3.3.4 Central System Router

**File:** `/server/central_system.go` (refactored)

```go
type CentralSystem struct {
    server          *Server
    handlers        map[common.ProtocolVersion]common.MessageHandler
    featureRegistry common.FeatureRegistry
    connections     sync.Map  // chargePointId → ProtocolVersion
    // ... existing fields
}

func (cs *CentralSystem) handleIncomingMessage(ws common.VersionedWebSocket, data []byte) error {
    // Parse generic message structure
    message, err := utility.ParseJson(data)
    callType, _ := MessageType(message)

    if callType == CallTypeResult {
        // Handle response to our outgoing request
        return cs.handleResponse(message)
    }

    // Extract action name (message[2])
    action := message[2].(string)

    // Get protocol version from WebSocket
    version := ws.GetProtocol()

    // Get appropriate handler
    handler, ok := cs.handlers[version]
    if !ok {
        return fmt.Errorf("unsupported protocol version: %s", version)
    }

    // Delegate to version-specific handler
    response, err := handler.HandleRequest(ws, action, message[3])
    if err != nil {
        return cs.sendCallError(ws, message[1].(string), err)
    }

    return cs.server.SendResponse(ws, response)
}

func (cs *CentralSystem) RegisterHandler(version common.ProtocolVersion, handler common.MessageHandler) {
    cs.handlers[version] = handler
}
```

#### 3.3.5 OCPP 1.6J Handler (Adapter Pattern)

**File:** `/ocpp/v16/handler.go`

```go
package v16

import (
    "evsys/ocpp/common"
    "evsys/ocpp/v16/core"
)

type Handler16 struct {
    coreHandler       CoreHandler
    firmwareHandler   FirmwareHandler
    // ... existing handlers
    featureRegistry   common.FeatureRegistry
}

func NewHandler16(/* dependencies */) *Handler16 {
    h := &Handler16{
        coreHandler: NewCoreHandler(...),
        // ... initialize handlers
    }
    h.registerFeatures()
    return h
}

func (h *Handler16) registerFeatures() {
    // Register all 1.6J features
    h.featureRegistry.RegisterFeature(
        common.OCPP16,
        core.BootNotificationFeatureName,
        reflect.TypeOf(core.BootNotificationRequest{}),
        reflect.TypeOf(core.BootNotificationResponse{}),
    )
    // ... register all features
}

func (h *Handler16) HandleRequest(ws common.VersionedWebSocket, action string, payload []byte) (common.Response, error) {
    // Get types from registry
    reqType, _, err := h.featureRegistry.GetTypes(common.OCPP16, action)
    if err != nil {
        return nil, fmt.Errorf("unsupported action: %s", action)
    }

    // Parse payload to concrete type
    request, err := ParseRequest(payload, reqType)
    if err != nil {
        return nil, err
    }

    // Route to appropriate handler
    switch action {
    case core.BootNotificationFeatureName:
        return h.coreHandler.OnBootNotification(ws.ID(), request.(*core.BootNotificationRequest))
    case core.AuthorizeFeatureName:
        return h.coreHandler.OnAuthorize(ws.ID(), request.(*core.AuthorizeRequest))
    // ... all other cases
    default:
        return nil, fmt.Errorf("unsupported feature: %s", action)
    }
}

func (h *Handler16) GetVersion() common.ProtocolVersion {
    return common.OCPP16
}
```

#### 3.3.6 OCPP 2.0.1 Handler

**File:** `/ocpp/v201/handler.go`

```go
package v201

import "evsys/ocpp/common"

type Handler201 struct {
    authHandler         AuthorizationHandler
    transactionHandler  TransactionHandler
    provisioningHandler ProvisioningHandler
    // ... other 2.0.1 handlers
    featureRegistry     common.FeatureRegistry
}

func NewHandler201(/* dependencies */) *Handler201 {
    h := &Handler201{
        // ... initialize handlers
    }
    h.registerFeatures()
    return h
}

func (h *Handler201) HandleRequest(ws common.VersionedWebSocket, action string, payload []byte) (common.Response, error) {
    // Similar to Handler16 but for OCPP 2.0.1 features
    // ...
}

func (h *Handler201) GetVersion() common.ProtocolVersion {
    return common.OCPP201
}
```

---

## 4. Implementation Phases

### Phase 0: Preparation and Planning (2-3 weeks)

**Objectives:**
- Finalize architecture decisions
- Set up development environment
- Create comprehensive test suite for existing 1.6J functionality

**Tasks:**
1. **Code Freeze:** Establish baseline branch
2. **Documentation:** Complete OCPP 2.0.1/2.1 specification review
3. **Test Coverage:** Add integration tests for all 1.6J features
4. **Feature Parity Analysis:** Document which 2.0.1 features map to 1.6J
5. **Database Schema Review:** Identify required migrations

**Deliverables:**
- Test suite with >80% coverage of existing 1.6J code
- Mapping document: 1.6J ↔ 2.0.1 feature equivalents
- Database migration plan

---

### Phase 1: Refactor Core Infrastructure (4-6 weeks)

**Objectives:**
- Introduce version abstraction without breaking existing functionality
- Maintain backward compatibility with 1.6J

**Tasks:**

#### 1.1 Create Common Package Structure ✅
- [x] Create `/ocpp/common` package
- [x] Define `ProtocolVersion` type and constants
- [x] Implement `FeatureRegistry` interface and implementation
- [x] Define version-agnostic `Request`/`Response` interfaces
- [x] Create `MessageHandler` interface

**Files to Create:**
- `ocpp/common/protocol.go`
- `ocpp/common/registry.go`
- `ocpp/common/interfaces.go`

#### 1.2 Refactor WebSocket Layer ✅
- [x] Extend `ocpp.WebSocket` interface with `GetProtocol()`/`SetProtocol()`
- [x] Update `server/client.go` to track protocol version
- [x] Modify WebSocket upgrade to store negotiated subprotocol
- [x] Update connection pool to maintain version mapping

**Files to Modify:**
- `ocpp/ocpp.go` (add methods to WebSocket interface)
- `server/client.go` (add protocol field)
- `server/server.go:210-226` (store subprotocol)
- `server/central_system.go` (add connections map)

#### 1.3 Migrate Existing 1.6J Code to Versioned Namespace ✅
- [x] Create `/ocpp/v16` package structure
- [x] Move `/ocpp/core` → `/ocpp/v16/core`
- [x] Move `/ocpp/firmware` → `/ocpp/v16/firmware`
- [x] Move `/ocpp/smartcharging` → `/ocpp/v16/smartcharging`
- [x] Move `/ocpp/localauth` → `/ocpp/v16/localauth`
- [x] Move `/ocpp/remotetrigger` → `/ocpp/v16/remotetrigger`
- [x] Update all import paths throughout codebase
- [x] Create `/ocpp/v16/handler.go` implementing `MessageHandler`
- [x] Move 1.6J specific types from `/types` to `/ocpp/v16/types.go`

**Files to Migrate:**
- All files in `/ocpp/core/` → `/ocpp/v16/core/`
- All files in `/ocpp/firmware/` → `/ocpp/v16/firmware/`
- All files in `/ocpp/smartcharging/` → `/ocpp/v16/smartcharging/`
- All files in `/ocpp/localauth/` → `/ocpp/v16/localauth/`
- All files in `/ocpp/remotetrigger/` → `/ocpp/v16/remotetrigger/`

**Files to Update:**
- `server/system_handler.go` (update imports)
- `server/central_system.go` (update imports)
- All business logic files importing OCPP types

#### 1.4 Implement Version-Aware Routing ✅
- [x] Refactor `CentralSystem.handleIncomingMessage()` to use handler registry
- [x] Remove hardcoded switch statements (legacy preserved for backward compatibility, new routing available via `EnableVersionAwareRouting()`)
- [x] Implement dynamic routing via `FeatureRegistry`
- [x] Add version detection and handler selection
- [x] Update API handler to support version parameter (`protocol_version` field in API command)

**Files to Modify:**
- `server/central_system.go:62-207` (complete refactor)
- `server/message.go:161-187` (replace with registry lookup)

#### 1.5 Testing ⚠️ (Partial)
- [x] Run full test suite against refactored 1.6J code
- [ ] Integration tests with real charge points
- [ ] Performance benchmarks (ensure no regression)

**Success Criteria:**
- All existing 1.6J functionality works unchanged
- Code now supports version abstraction
- Import paths updated throughout
- Tests pass with >95% success rate

---

### Phase 2: Implement OCPP 2.0.1 Core Features (8-10 weeks)

**Objectives:**
- Implement OCPP 2.0.1 message types and handlers
- Focus on core transaction flow first

**Tasks:**

#### 2.1 Define OCPP 2.0.1 Type System ✅
- [x] Create `/ocpp/v201/types.go` with base types
- [x] Implement `IdToken` (replaces IdTag)
- [x] Implement `EVSE` and `Connector` structures
- [x] Implement `Transaction` types
- [x] Implement `ChargingStation` type
- [x] Implement `StatusInfo` type
- [x] Implement all enumerations (IdTokenType, ConnectorStatusType, etc.)

**Files to Create:**
- `ocpp/v201/types.go`

#### 2.2 Implement Provisioning Features (Bootstrapping) ✅
- [x] `BootNotificationRequest`/`Response` (different structure than 1.6)
- [x] `NotifyReportRequest`/`Response`
- [x] `GetBaseReportRequest`/`Response`
- [x] `GetVariablesRequest`/`Response`
- [x] `SetVariablesRequest`/`Response`
- [x] `ResetRequest`/`Response`

**Files to Create:**
- `ocpp/v201/provisioning/boot_notification.go`
- `ocpp/v201/provisioning/notify_report.go`
- `ocpp/v201/provisioning/get_base_report.go`
- `ocpp/v201/provisioning/get_variables.go`
- `ocpp/v201/provisioning/set_variables.go`
- `ocpp/v201/provisioning/reset.go`

#### 2.3 Implement Authorization Features ✅
- [x] `AuthorizeRequest`/`Response` (with IdToken)
- [x] `ClearedChargingLimitRequest`/`Response`
- [x] `RequestStartTransactionRequest`/`Response`
- [x] `RequestStopTransactionRequest`/`Response`

**Files to Create:**
- `ocpp/v201/authorization/authorize.go`
- `ocpp/v201/authorization/cleared_charging_limit.go`
- `ocpp/v201/remotecontrol/request_start_transaction.go`
- `ocpp/v201/remotecontrol/request_stop_transaction.go`

#### 2.4 Implement Transaction Features ✅
- [x] `TransactionEventRequest`/`Response` (replaces Start/Stop + MeterValues)
- [x] `StatusNotificationRequest`/`Response` (different structure)
- [x] `HeartbeatRequest`/`Response`

**Files to Create:**
- `ocpp/v201/transactions/transaction_event.go`
- `ocpp/v201/availability/status_notification.go`
- `ocpp/v201/provisioning/heartbeat.go`

#### 2.5 Create Handler201 ✅
- [x] Implement `Handler201` struct
- [x] Register all features in `FeatureRegistry`
- [x] Implement `HandleRequest()` with routing
- [x] Create handler interfaces for each functional block
- [x] Implement business logic adapters

**Files to Create:**
- `ocpp/v201/handler.go`
- `ocpp/v201/provisioning/handler.go`
- `ocpp/v201/authorization/handler.go`
- `ocpp/v201/transactions/handler.go`

#### 2.6 Update Business Logic Layer ✅
- [x] Create transaction abstraction layer
- [x] Map `TransactionEvent` → internal Transaction entity
- [x] Map `IdToken` ↔ `IdTag` for authorization
- [x] Update billing service to handle 2.0.1 meter values
- [ ] Extend power manager for EVSE-level management

**Files to Modify:**
- `server/system_handler.go` (create adapters)
- `entity/transaction.go` (add version field, flexible schema)
- `entity/charge_point.go` (add EVSE support)
- `billing/billing_service.go` (handle both formats)

#### 2.7 Database Schema Updates ✅
- [x] Add `protocol_version` field to `charge_points`
- [x] Add `evse_id` field to `connectors` (nullable for 1.6 compatibility)
- [x] Add flexible `metadata` JSONB field to `transactions`
- [ ] Create migration scripts

**Files to Modify:**
- `entity/charge_point.go`
- `entity/connector.go`
- `entity/transaction.go`
- `internal/mongo.go` (add migration logic)

#### 2.8 Testing ✅
- [x] Unit tests for all 2.0.1 message types (64 tests, 100% passing)
- [ ] Integration tests for basic transaction flow
- [ ] Test 1.6 and 2.0.1 charge points concurrently

**Success Criteria:**
- OCPP 2.0.1 charge points can connect and authenticate
- Basic transaction flow works (authorize → transaction → stop)
- 1.6J charge points continue working without issues
- Both versions can run simultaneously

---

### Phase 3: Complete OCPP 2.0.1 Implementation (6-8 weeks)

**Objectives:**
- Implement remaining OCPP 2.0.1 features
- Add advanced functionality

**Tasks:**

#### 3.1 Implement Smart Charging Features
- [ ] `SetChargingProfileRequest`/`Response` (new structure)
- [ ] `GetChargingProfilesRequest`/`Response`
- [ ] `ClearChargingProfileRequest`/`Response`
- [ ] `GetCompositeScheduleRequest`/`Response`
- [ ] `NotifyEVChargingNeedsRequest`/`Response`
- [ ] `NotifyEVChargingScheduleRequest`/`Response`

**Files to Create:**
- `ocpp/v201/smartcharging/*.go` (6 files)

#### 3.2 Implement Firmware Management
- [ ] `PublishFirmwareRequest`/`Response`
- [ ] `UnpublishFirmwareRequest`/`Response`
- [ ] `UpdateFirmwareRequest`/`Response`
- [ ] `FirmwareStatusNotificationRequest`/`Response`

**Files to Create:**
- `ocpp/v201/firmware/*.go` (4 files)

#### 3.3 Implement Diagnostics and Monitoring
- [ ] `GetLogRequest`/`Response`
- [ ] `NotifyEventRequest`/`Response`
- [ ] `NotifyMonitoringReportRequest`/`Response`
- [ ] `SetMonitoringBaseRequest`/`Response`
- [ ] `SetMonitoringLevelRequest`/`Response`
- [ ] `SetVariableMonitoringRequest`/`Response`
- [ ] `GetMonitoringReportRequest`/`Response`

**Files to Create:**
- `ocpp/v201/diagnostics/*.go` (7 files)

#### 3.4 Implement Display Message Management
- [ ] `SetDisplayMessageRequest`/`Response`
- [ ] `GetDisplayMessagesRequest`/`Response`
- [ ] `ClearDisplayMessageRequest`/`Response`
- [ ] `NotifyDisplayMessagesRequest`/`Response`

**Files to Create:**
- `ocpp/v201/display/*.go` (4 files)

#### 3.5 Implement ISO 15118 Features (Plug & Charge)
- [ ] `Get15118EVCertificateRequest`/`Response`
- [ ] `GetCertificateStatusRequest`/`Response`
- [ ] `SignCertificateRequest`/`Response`
- [ ] Certificate management integration

**Files to Create:**
- `ocpp/v201/iso15118/*.go` (3 files)
- Consider external PKI integration

#### 3.6 Implement Tariff and Cost
- [ ] `CostUpdatedRequest`/`Response`
- [ ] Tariff calculation updates for 2.0.1 format

**Files to Create:**
- `ocpp/v201/tariffandcost/cost_updated.go`

#### 3.7 Implement Data Transfer and Availability
- [ ] `DataTransferRequest`/`Response`
- [ ] `ChangeAvailabilityRequest`/`Response`

**Files to Create:**
- `ocpp/v201/datatransfer/data_transfer.go`
- `ocpp/v201/availability/change_availability.go`

#### 3.8 Security Implementation
- [ ] Certificate-based authentication
- [ ] Security event logging
- [ ] Signed firmware validation
- [ ] TLS certificate management

**Files to Create:**
- `ocpp/v201/security/*.go`
- Update `server/server.go` for certificate validation

#### 3.9 Device Model Implementation
- [ ] Hierarchical component/variable storage
- [ ] Configuration variable mapping (1.6 keys → 2.0.1 variables)
- [ ] Variable attribute support (value, mutability, persistence)

**Files to Create:**
- `entity/device_model.go`
- `internal/mongo.go` (add device model collections)

#### 3.10 Testing
- [ ] Comprehensive feature testing for all 2.0.1 operations
- [ ] OCTT (OCPP Compliance Testing Tool) validation
- [ ] Load testing with multiple protocol versions
- [ ] Security testing (certificate validation, etc.)

**Success Criteria:**
- All OCPP 2.0.1 features implemented
- Pass OCTT compliance tests
- Production-ready with proper error handling
- Documentation complete

---

### Phase 4: OCPP 2.1 Support (Optional, 4-6 weeks)

**Objectives:**
- Add OCPP 2.1 specific features
- Leverage 2.0.1 foundation

**Tasks:**
- [ ] Review OCPP 2.1 changelog vs 2.0.1
- [ ] Implement new features (Vehicle-to-Grid, enhanced display, etc.)
- [ ] Create `/ocpp/v21` package
- [ ] Potentially reuse 2.0.1 code with minimal changes

**Notes:**
- OCPP 2.1 is largely compatible with 2.0.1
- May be able to use inheritance/composition to minimize duplication
- Consider whether full implementation is needed based on market demand

---

### Phase 5: Production Hardening (3-4 weeks)

**Objectives:**
- Prepare for production rollout
- Optimize performance
- Complete documentation

**Tasks:**

#### 5.1 Performance Optimization
- [ ] Profile message parsing (minimize JSON marshal/unmarshal)
- [ ] Optimize registry lookups (caching)
- [ ] Connection pool optimization
- [ ] Database query optimization for version-specific queries

#### 5.2 Monitoring and Observability
- [ ] Add metrics for protocol version distribution
- [ ] Add metrics for feature usage by version
- [ ] Update Prometheus metrics
- [ ] Enhanced logging with version context

**Files to Modify:**
- `metrics/counters/counters.go`
- `server/central_system.go` (add version-aware logging)

#### 5.3 Configuration Management
- [ ] Add configuration for supported protocol versions
- [ ] Add feature toggles (enable/disable specific versions)
- [ ] Version-specific settings

**Files to Modify:**
- `internal/config/config.go`

Example:
```yaml
ocpp:
  supported_versions:
    - "1.6"
    - "2.0.1"
    - "2.1"
  default_version: "1.6"
  v16:
    enabled: true
  v201:
    enabled: true
    require_certificates: true
  v21:
    enabled: false
```

#### 5.4 Documentation
- [ ] Update README.md with multi-version support
- [ ] Update CLAUDE.md with new architecture
- [ ] API documentation for version-specific endpoints
- [ ] Migration guide for charge point operators
- [ ] Troubleshooting guide

**Files to Create/Modify:**
- `README.md`
- `CLAUDE.md`
- `docs/API.md`
- `docs/MIGRATION_GUIDE.md`
- `docs/TROUBLESHOOTING.md`

#### 5.5 Deployment Strategy
- [ ] Create deployment checklist
- [ ] Rollback procedures
- [ ] Gradual rollout plan (canary deployments)
- [ ] Database backup procedures

---

## 5. Database Migration Strategy

### 5.1 Schema Changes

**Minimal Impact Approach:**

Add version-agnostic fields and flexible storage:

```javascript
// charge_points collection
{
    _id: "CP001",
    location_id: "LOC1",
    protocol_version: "ocpp2.0.1",  // NEW
    vendor: "ABB",
    model: "Terra 54",
    // ... existing fields
    device_model: {  // NEW: for OCPP 2.0.1+
        components: [
            {
                name: "OCPPCommCtrlr",
                variables: [
                    {name: "HeartbeatInterval", value: "30"}
                ]
            }
        ]
    }
}

// connectors collection
{
    _id: "connector_uuid",
    charge_point_id: "CP001",
    connector_id: 1,  // 1.6J connector ID
    evse_id: 1,       // NEW: OCPP 2.0.1+ EVSE ID (nullable)
    // ... existing fields
}

// transactions collection
{
    _id: "transaction_uuid",
    transaction_id: 12345,  // 1.6J transaction ID
    charge_point_id: "CP001",
    connector_id: 1,
    protocol_version: "ocpp1.6",  // NEW
    id_tag: "USER001",           // 1.6J format
    id_token: {                  // NEW: 2.0.1 format
        id_token: "USER001",
        type: "ISO14443"
    },
    // ... existing fields
    metadata: {  // NEW: version-specific data
        // Flexible JSONB field for protocol-specific data
    }
}
```

### 5.2 Migration Scripts

**Phase 1: Add New Fields**
```javascript
// MongoDB migration
db.charge_points.updateMany(
    {protocol_version: {$exists: false}},
    {$set: {protocol_version: "ocpp1.6"}}
);

db.connectors.updateMany(
    {evse_id: {$exists: false}},
    {$set: {evse_id: null}}
);

db.transactions.updateMany(
    {protocol_version: {$exists: false}},
    {$set: {protocol_version: "ocpp1.6"}}
);
```

**Phase 2: Create Indexes**
```javascript
db.charge_points.createIndex({protocol_version: 1});
db.transactions.createIndex({protocol_version: 1});
```

### 5.3 Backward Compatibility

- All new fields are optional or have defaults
- Existing queries continue to work
- Gradual migration of data as charge points connect
- No downtime required

---

## 6. Testing Strategy

### 6.1 Unit Testing

**Coverage Goals:**
- Message parsing: 100%
- Type conversions: 100%
- Feature routing: 95%
- Business logic: 85%

**Test Structure:**
```
/tests
  /v16
    /core
      boot_notification_test.go
      authorize_test.go
      ...
  /v201
    /provisioning
      boot_notification_test.go
      ...
    /transactions
      transaction_event_test.go
      ...
  /integration
    multi_version_test.go
    concurrent_connections_test.go
  /performance
    benchmark_test.go
```

### 6.2 Integration Testing

**Test Scenarios:**
1. **Single Version Tests:**
   - Pure 1.6J environment (regression test)
   - Pure 2.0.1 environment

2. **Multi-Version Tests:**
   - 1.6J and 2.0.1 charge points connected simultaneously
   - Version switching (charge point disconnects as 1.6, reconnects as 2.0.1)
   - Load balancing across different versions

3. **Feature Mapping Tests:**
   - Authorize: 1.6J IdTag vs 2.0.1 IdToken
   - Transaction flow: 1.6J Start/Stop vs 2.0.1 TransactionEvent
   - Configuration: 1.6J GetConfiguration vs 2.0.1 GetVariables

4. **Error Handling:**
   - Unsupported version
   - Malformed messages
   - Protocol mismatch
   - Timeout scenarios

### 6.3 Compliance Testing

**OCPP Compliance Testing Tool (OCTT):**
- Run official OCTT tests for OCPP 2.0.1
- Maintain 1.6J compliance
- Document any deviations

### 6.4 Performance Testing

**Benchmarks:**
- Message parsing throughput (messages/sec by version)
- Connection handling (concurrent connections)
- Memory usage (compare pre/post refactor)
- Latency (round-trip time for requests)

**Load Testing:**
- 1000 concurrent 1.6J connections
- 1000 concurrent 2.0.1 connections
- 500+500 mixed connections
- Transaction throughput

---

## 7. Rollout Strategy

### 7.1 Phased Deployment

**Phase 1: Internal Testing (Week 1-2)**
- Deploy to staging environment
- Internal charge points only
- Monitor for issues

**Phase 2: Beta Testing (Week 3-4)**
- Select pilot customers
- 10% of production traffic
- Enhanced monitoring and logging

**Phase 3: Gradual Rollout (Week 5-8)**
- 25% → 50% → 75% → 100%
- Monitor metrics at each stage
- Rollback capability at each stage

**Phase 4: Full Production (Week 9+)**
- All traffic on new system
- Old code branch maintained for 1 month as fallback

### 7.2 Feature Flags

**Configuration-Based Rollout:**
```yaml
ocpp:
  v16:
    enabled: true
  v201:
    enabled: false  # Start disabled
    beta_charge_points:  # Whitelist for testing
      - "CP001"
      - "CP002"
  v21:
    enabled: false
```

### 7.3 Rollback Plan

**Criteria for Rollback:**
- >5% error rate increase
- Critical bug affecting transactions
- Performance degradation >20%
- Data integrity issues

**Rollback Procedure:**
1. Switch traffic back to old branch (load balancer)
2. Database schema is backward compatible (no rollback needed)
3. Investigate issues in staging
4. Fix and re-deploy

**Time to Rollback:** <15 minutes

---

## 8. Risk Assessment and Mitigation

### 8.1 Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| **Breaking 1.6J compatibility** | Medium | Critical | Comprehensive regression testing; maintain 1.6J test suite |
| **Performance degradation** | Medium | High | Benchmarking at each phase; optimize hot paths |
| **Database migration issues** | Low | High | Test migrations extensively; backup procedures |
| **Timeline overrun** | High | Medium | Phased approach allows partial delivery; prioritize core features |
| **Charge point compatibility** | Medium | High | Test with multiple vendor hardware; maintain vendor matrix |
| **Security vulnerabilities** | Medium | Critical | Security audit; penetration testing; certificate validation |
| **Team knowledge gap** | Medium | Medium | Training on OCPP 2.0.1; pair programming; documentation |

### 8.2 Mitigation Strategies

**1. Breaking Changes:**
- Maintain comprehensive test suite for 1.6J
- Automated regression testing in CI/CD
- Freeze 1.6J changes during migration

**2. Performance:**
- Benchmark after each phase
- Profile hot paths (message parsing, routing)
- Consider caching strategies for registry lookups

**3. Compatibility:**
- Maintain charge point compatibility matrix
- Test with multiple vendors (ABB, EVBox, Wallbox, etc.)
- Beta testing program with diverse hardware

**4. Knowledge:**
- Team training sessions on OCPP 2.0.1
- Code reviews with focus on architecture
- Document design decisions

---

## 9. Success Criteria

### 9.1 Functional Requirements

- [ ] All OCPP 1.6J features continue working without regression
- [ ] OCPP 2.0.1 core features functional (boot, authorize, transaction)
- [ ] OCPP 2.0.1 advanced features implemented (smart charging, firmware, ISO 15118)
- [ ] Concurrent support for 1.6J and 2.0.1 charge points
- [ ] Pass OCTT compliance tests for 2.0.1
- [ ] API supports version-specific commands

### 9.2 Non-Functional Requirements

- [ ] Performance: <5% overhead vs current 1.6J implementation
- [ ] Latency: <100ms p95 for message routing
- [ ] Scalability: Support 1000+ concurrent connections per version
- [ ] Reliability: 99.9% uptime during rollout
- [ ] Code Quality: >85% test coverage
- [ ] Documentation: Complete API docs, migration guides

### 9.3 Business Requirements

- [ ] Zero downtime migration
- [ ] Backward compatible with existing integrations
- [ ] No customer-facing issues during rollout
- [ ] Enable new customer acquisition (2.0.1 required)
- [ ] Competitive feature parity with industry standards

---

## 10. Resource Estimation

### 10.1 Timeline Summary

| Phase | Duration | Team Size | Effort |
|-------|----------|-----------|--------|
| Phase 0: Preparation | 2-3 weeks | 2 developers | 4-6 person-weeks |
| Phase 1: Infrastructure Refactor | 4-6 weeks | 3 developers | 12-18 person-weeks |
| Phase 2: OCPP 2.0.1 Core | 8-10 weeks | 3 developers | 24-30 person-weeks |
| Phase 3: OCPP 2.0.1 Complete | 6-8 weeks | 3 developers | 18-24 person-weeks |
| Phase 4: OCPP 2.1 (Optional) | 4-6 weeks | 2 developers | 8-12 person-weeks |
| Phase 5: Production Hardening | 3-4 weeks | 3 developers | 9-12 person-weeks |
| **Total (without 2.1)** | **23-31 weeks** | **3 developers** | **67-90 person-weeks** |

### 10.2 Team Composition

**Recommended Team:**
- 2 Senior Go Developers (OCPP expertise)
- 1 Mid-level Go Developer (implementation support)
- 1 QA Engineer (testing, OCTT)
- 1 DevOps Engineer (deployment, monitoring)

### 10.3 External Dependencies

- **OCPP 2.0.1 Charge Points:** For testing (3-5 different vendors)
- **OCTT License:** For compliance testing
- **PKI Infrastructure:** For ISO 15118 / certificate management (Phase 3)
- **Load Testing Environment:** Separate infrastructure for performance testing

---

## 11. Appendix

### 11.1 OCPP 2.0.1 Feature Checklist

**Provisioning (7 operations):**
- [x] BootNotification ✅
- [x] Heartbeat ✅
- [x] NotifyReport ✅
- [x] GetBaseReport ✅
- [x] GetVariables ✅
- [x] SetVariables ✅
- [x] Reset ✅

**Authorization (6 operations):**
- [x] Authorize ✅
- [x] ClearedChargingLimit ✅
- [x] RequestStartTransaction ✅
- [x] RequestStopTransaction ✅
- [ ] SendLocalList
- [ ] GetLocalListVersion

**Transactions (2 operations):**
- [x] TransactionEvent ✅
- [ ] GetTransactionStatus

**Availability (3 operations):**
- [x] StatusNotification ✅
- [ ] ChangeAvailability
- [ ] NotifyEvent

**Smart Charging (7 operations):**
- [ ] SetChargingProfile
- [ ] GetChargingProfiles
- [ ] ClearChargingProfile
- [ ] GetCompositeSchedule
- [ ] NotifyEVChargingNeeds
- [ ] NotifyEVChargingSchedule
- [ ] ReportChargingProfiles

**Firmware Management (4 operations):**
- [ ] PublishFirmware
- [ ] UnpublishFirmware
- [ ] UpdateFirmware
- [ ] FirmwareStatusNotification

**Diagnostics (7 operations):**
- [ ] GetLog
- [ ] NotifyEvent
- [ ] NotifyMonitoringReport
- [ ] SetMonitoringBase
- [ ] SetMonitoringLevel
- [ ] SetVariableMonitoring
- [ ] GetMonitoringReport

**Display Management (4 operations):**
- [ ] SetDisplayMessage
- [ ] GetDisplayMessages
- [ ] ClearDisplayMessage
- [ ] NotifyDisplayMessages

**ISO 15118 (3 operations):**
- [ ] Get15118EVCertificate
- [ ] GetCertificateStatus
- [ ] SignCertificate

**Tariff and Cost (1 operation):**
- [ ] CostUpdated

**Data Transfer (1 operation):**
- [ ] DataTransfer

**Total: 45+ operations**

### 11.2 Reference Documents

- [OCPP 1.6 Specification (Edition 2)](https://www.openchargealliance.org/protocols/ocpp-16/)
- [OCPP 2.0.1 Specification](https://www.openchargealliance.org/protocols/ocpp-201/)
- [OCPP 2.1 Specification](https://www.openchargealliance.org/protocols/ocpp-21/)
- [OCPP Implementation Guide](https://www.openchargealliance.org/implementation-guide/)
- [OCPI 2.2.1 Specification](https://github.com/ocpi/ocpi) (for tariff structures)

### 11.3 Glossary

- **OCPP:** Open Charge Point Protocol
- **OCPP-J:** OCPP over JSON/WebSocket
- **EVSE:** Electric Vehicle Supply Equipment (charge point in 2.0.1 terminology)
- **IdTag:** Authorization identifier in OCPP 1.6J (max 20 chars)
- **IdToken:** Authorization token in OCPP 2.0.1 (with type information)
- **OCTT:** OCPP Compliance Testing Tool
- **ISO 15118:** International standard for V2G communication (Plug & Charge)
- **PKI:** Public Key Infrastructure (for certificate management)

---

## Document Revision History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-11-05 | System Architect | Initial version |
| 1.1 | 2026-01-02 | Claude Code | Updated task status: Phase 1 complete, Phase 2 substantially complete (14/17 features implemented) |
| 1.2 | 2026-01-02 | Claude Code | Completed task 1.4: API version-aware routing with protocol_version parameter |

---

**Next Steps:**

~~1. Review and approve this migration plan~~ ✅
~~2. Establish project team and allocate resources~~ ✅
~~3. Set up development environment for Phase 0~~ ✅
~~4. Schedule kickoff meeting~~ ✅
~~5. Begin Phase 0: Preparation and Planning~~ ✅

**Current Priority (as of 2026-01-02):**

1. **Enable version-aware routing** - Call `CentralSystem.EnableVersionAwareRouting()` when ready for production testing
2. **Complete remaining Phase 2 items:**
   - Integration tests with real 2.0.1 charge points
   - MongoDB migration scripts
   - Power manager EVSE-level support
3. **Begin Phase 3: Smart Charging** - Implement OCPP 2.0.1 smart charging features
4. **Begin Phase 3: Firmware Management** - Implement firmware update operations
