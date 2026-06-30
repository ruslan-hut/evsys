# Charger Test Checklist

## Multi-Version OCPP Testing Guide

This document provides a comprehensive checklist for testing OCPP 1.6J and OCPP 2.0.1 charge points with the EVSYS central system.

---

## Pre-Test Setup

### Server Configuration

- [x] **Version-aware routing is always on** — enabled unconditionally at `main.go:44` (`centralSystem.EnableVersionAwareRouting()`). No manual step required.

- [ ] **Verify supported subprotocols** are configured:
  - `ocpp1.6` (default)
  - `ocpp2.0.1` (added)

- [ ] **Check database migrations** ran successfully (version 2+)

- [ ] **Enable debug mode** for verbose logging:
  ```yaml
  is_debug: true
  ```

---

## OCPP 1.6J Test Cases

### 1. Connection & Bootstrapping

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] WebSocket connection with `Sec-WebSocket-Protocol: ocpp1.6` | Connection accepted | |
| [ ] BootNotification (vendor, model, serialNumber) | `status: Accepted`, heartbeat interval returned | |
| [ ] Heartbeat | Current timestamp returned | |
| [ ] StatusNotification for all connectors | Status recorded in database | |

**API Test:**
```json
POST /api
{
  "charge_point_id": "CP001",
  "feature_name": "GetServerStatus"
}
```

### 2. Authorization

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] Authorize with valid IdTag | `status: Accepted` | |
| [ ] Authorize with invalid IdTag (when `accept_unknown_tag: false`) | `status: Invalid` | |
| [ ] Authorize with blocked IdTag | `status: Blocked` | |

### 3. Transaction Flow

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] StartTransaction with valid IdTag | `transactionId` assigned, `status: Accepted` | |
| [ ] MeterValues during transaction | Values stored, price calculated | |
| [ ] StopTransaction | Transaction marked finished, price computed (`payment_amount > 0`) | |
| [ ] Remote start via API | Transaction started on specified connector | |
| [ ] Remote stop via API | Transaction stopped | |

**API Tests:**
```json
// Remote Start
{
  "charge_point_id": "CP001",
  "connector_id": 1,
  "feature_name": "RemoteStartTransaction",
  "payload": "USER001"
}

// Remote Stop
{
  "charge_point_id": "CP001",
  "feature_name": "RemoteStopTransaction",
  "payload": "12345"
}
```

### 4. Configuration

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] GetConfiguration (all keys) | Configuration keys returned | |
| [ ] GetConfiguration (specific key) | Single value returned | |
| [ ] ChangeConfiguration | `status: Accepted` or `RebootRequired` | |

**API Test:**
```json
{
  "charge_point_id": "CP001",
  "feature_name": "GetConfiguration",
  "payload": "HeartbeatInterval"
}
```

### 5. Smart Charging

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] SetChargingProfile | Profile applied | |
| [ ] ClearChargingProfile | Profile removed | |
| [ ] GetCompositeSchedule | Schedule returned | |

### 6. Firmware & Diagnostics

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] GetDiagnostics | Diagnostics upload initiated | |
| [ ] DiagnosticsStatusNotification | Status received | |
| [ ] FirmwareStatusNotification | Status received | |

### 7. Other Operations

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] Reset (Soft) | Charge point reboots | |
| [ ] Reset (Hard) | Charge point reboots immediately | |
| [ ] TriggerMessage (BootNotification) | BootNotification sent | |
| [ ] TriggerMessage (StatusNotification) | StatusNotification sent | |
| [ ] SendLocalList | Local list updated | |

---

## OCPP 2.0.1 Test Cases

### 1. Connection & Bootstrapping

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] WebSocket connection with `Sec-WebSocket-Protocol: ocpp2.0.1` | Connection accepted | |
| [ ] BootNotification (ChargingStation, reason) | `status: Accepted`, interval returned | |
| [ ] Heartbeat | Current timestamp returned | |
| [ ] StatusNotification (per EVSE/Connector) | Status recorded with EVSE ID | |
| [ ] NotifyReport (device model data) | Report data stored | |

**API Test (with version):**
```json
POST /api
{
  "charge_point_id": "CP002",
  "feature_name": "GetServerStatus",
  "protocol_version": "ocpp2.0.1"
}
```

### 2. Authorization (IdToken)

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] Authorize with IdToken (type: ISO14443) | `idTokenInfo.status: Accepted` | |
| [ ] Authorize with IdToken (type: Local) | `idTokenInfo.status: Accepted` | |
| [ ] Authorize with invalid IdToken | `idTokenInfo.status: Invalid` | |
| [ ] Authorize with blocked IdToken | `idTokenInfo.status: Blocked` | |

### 3. Transaction Flow (TransactionEvent)

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] TransactionEvent (Started) with IdToken | Transaction created, ID assigned | |
| [ ] TransactionEvent (Updated) with MeterValue | Meter values stored, price updated | |
| [ ] TransactionEvent (Ended) | Transaction finished, price computed (`payment_amount > 0`) | |
| [ ] RequestStartTransaction via API | Transaction started on specified EVSE | |
| [ ] RequestStopTransaction via API | Transaction stopped | |

**API Tests (OCPP 2.0.1):**
```json
// Request Start Transaction
{
  "charge_point_id": "CP002",
  "connector_id": 1,
  "feature_name": "RequestStartTransaction",
  "payload": "USER001",
  "protocol_version": "ocpp2.0.1"
}

// Request Stop Transaction
{
  "charge_point_id": "CP002",
  "feature_name": "RequestStopTransaction",
  "payload": "tx-uuid-12345",
  "protocol_version": "ocpp2.0.1"
}
```

### 4. Configuration (Variables)

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] GetVariables (single variable) | Variable value returned | |
| [ ] GetVariables (multiple variables) | All values returned | |
| [ ] SetVariables | `status: Accepted` or `RebootRequired` | |

**API Test:**
```json
{
  "charge_point_id": "CP002",
  "feature_name": "GetVariables",
  "payload": "HeartbeatInterval",
  "protocol_version": "ocpp2.0.1"
}
```

### 5. Reset

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] Reset (Immediate) | Charge point reboots | |
| [ ] Reset (OnIdle) | Scheduled reset when idle | |

**API Test:**
```json
{
  "charge_point_id": "CP002",
  "feature_name": "Reset",
  "payload": "Hard",
  "protocol_version": "ocpp2.0.1"
}
```

### 6. EVSE Hierarchy

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] Multiple EVSEs with multiple connectors | Correct EVSE/Connector hierarchy stored | |
| [ ] StatusNotification per EVSE | EVSE ID correctly tracked | |
| [ ] Transaction on specific EVSE | Transaction linked to correct EVSE | |

---

## Multi-Version Concurrent Testing

### Mixed Environment

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] OCPP 1.6 and 2.0.1 charge points connected simultaneously | Both handled correctly | |
| [ ] Protocol version auto-detection from connection | Correct version used | |
| [ ] API commands without explicit version | Auto-detect from connection registry | |
| [ ] API commands with explicit version | Override connection version | |

### Version Upgrade Simulation

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] Charge point disconnects as 1.6, reconnects as 2.0.1 | New version tracked correctly | |
| [ ] Historical data preserved after version change | Old transactions accessible | |

---

## Payment Settlement (Cross-Service: evsys → evsys-back)

Payment is no longer handled inside evsys. evsys only computes the price and writes
`payment_amount` on the finished transaction; **evsys-back** settles it via Redsys.
Both services share the same MongoDB. This leg has never been exercised end-to-end and
is the main production risk — test it for **both** 1.6 and 2.0.1 transactions.

**Contract (evsys side):** a finished transaction must have
`is_finished: true`, `payment_amount > 0`, `payment_billed < payment_amount`.

```javascript
// On the shared DB, after a charging session ends:
db.transactions.findOne({transaction_id: <id>})
// Verify: is_finished == true, payment_amount > 0, payment_billed < payment_amount
```

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] 1.6 session ends → `payment_amount` written | `payment_amount > 0`, `payment_billed == 0` | |
| [ ] 2.0.1 session ends → `payment_amount` written | `payment_amount > 0`, `payment_billed == 0` | |
| [ ] evsys-back processor picks up unbilled tx | Within 5 min, payment order created | |
| [ ] Redsys **sandbox** settles (saved card / MIT) | `payment_billed == payment_amount`, order `Result` = success | |
| [ ] Settled tx not re-charged on next tick | No duplicate payment order | |
| [ ] Payment failure path | Retry record created, warning email sent | |
| [ ] User with no payment method | tx marked billed, no charge attempted | |

**Notes:**
- evsys-back must run with `redsys.enabled: true` pointing at the **sandbox**
  (`sis-t.redsys.es`), not the live endpoint.
- Settlement is a 5-minute polling worker in evsys-back (`StartPaymentProcessor`), not
  event-driven — allow up to one tick before checking results.

---

## Database Verification

After running tests, verify database state:

### charge_points collection
```javascript
db.charge_points.findOne({_id: "CP002"})
// Verify: protocol_version: "ocpp2.0.1", device_model present
```

### connectors collection
```javascript
db.connectors.find({charge_point_id: "CP002"})
// Verify: evse_id field present for 2.0.1 connectors
```

### transactions collection
```javascript
db.transactions.find({charge_point_id: "CP002"}).sort({time_start: -1}).limit(5)
// Verify: protocol_version, evse_id, metadata fields
```

---

## Error Handling Tests

| Test | Expected Result | Status |
|------|-----------------|--------|
| [ ] Unsupported feature request (1.6) | `NotSupported` error | |
| [ ] Unsupported feature request (2.0.1) | `NotSupported` error | |
| [ ] Malformed message | Error logged, connection maintained | |
| [ ] Connection timeout | Reconnection handled | |
| [ ] Invalid protocol version in API | Default to 1.6 | |

---

## Performance Benchmarks

| Metric | Target | Measured |
|--------|--------|----------|
| Message round-trip time (p95) | <100ms | |
| Concurrent connections (1.6) | 500+ | |
| Concurrent connections (2.0.1) | 500+ | |
| Memory usage per connection | <1MB | |
| Transaction throughput | 100 tx/min | |

---

## Test Equipment

### Recommended Test Chargers

**OCPP 1.6J:**
- Any OCPP 1.6 compliant charger
- OCPP 1.6 simulator (e.g., Steve simulator)

**OCPP 2.0.1:**
- OCPP 2.0.1 compliant charger
- OCTT (OCPP Compliance Testing Tool)
- OCPP 2.0.1 simulator

### Simulator Options

1. **Steve Simulator** - Open source OCPP 1.6 simulator
2. **OCTT** - Official compliance testing tool from OCA
3. **Custom Python/Node.js scripts** using WebSocket libraries

---

## Notes

- Date tested: _______________
- Tester: _______________
- Server version: _______________
- Charge point models tested:
  - 1.6: _______________
  - 2.0.1: _______________

---

## Issues Found

| Issue # | Description | Severity | Status |
|---------|-------------|----------|--------|
| | | | |
| | | | |
| | | | |
