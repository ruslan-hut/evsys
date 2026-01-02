# OCPP 2.0.1 API Reference

This document describes OCPP 2.0.1 features supported by EVSYS API.

## Table of Contents

- [Overview](#overview)
- [Provisioning Features](#provisioning-features)
  - [GetVariables](#getvariables)
  - [SetVariables](#setvariables)
  - [Reset](#reset)
- [Remote Control Features](#remote-control-features)
  - [RequestStartTransaction](#requeststarttransaction)
  - [RequestStopTransaction](#requeststoptransaction)
- [Incoming Messages](#incoming-messages-charge-point--central-system)
- [Common Types](#common-types)

---

## Overview

OCPP 2.0.1 introduces significant changes from OCPP 1.6:

- **Device Model**: Configuration is now managed through a structured device model with components and variables
- **EVSE/Connector Model**: Explicit hierarchy of Charging Station > EVSE > Connector
- **Transaction Events**: Single `TransactionEvent` message replaces StartTransaction/StopTransaction
- **ISO 15118**: Native support for Plug & Charge
- **Improved Security**: Certificate-based authentication

### Protocol Version Selection

To use OCPP 2.0.1 features, either:
1. Connect the charge point using OCPP 2.0.1 protocol (auto-detected)
2. Explicitly specify the protocol version in requests:

```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "GetVariables",
  "payload": "...",
  "protocol_version": "ocpp2.0.1"
}
```

---

## Provisioning Features

### GetVariables

Retrieve device model variables from the charging station.

**Feature Name:** `GetVariables`

**Direction:** Central System -> Charging Station

#### Request

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| getVariableData | GetVariableDataType[] | Yes | List of variables to retrieve |

**GetVariableDataType Structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| component | ComponentType | Yes | Component containing the variable |
| variable | VariableType | Yes | Variable to retrieve |
| attributeType | AttributeType | No | Attribute type (Actual, Target, MinSet, MaxSet) |

**ComponentType Structure:**

| Field | Type | Required | Constraints | Description |
|-------|------|----------|-------------|-------------|
| name | string | Yes | max 50 chars | Component name |
| instance | string | No | max 50 chars | Component instance |
| evse | EVSE | No | - | EVSE reference if component is EVSE-specific |

**VariableType Structure:**

| Field | Type | Required | Constraints | Description |
|-------|------|----------|-------------|-------------|
| name | string | Yes | max 50 chars | Variable name |
| instance | string | No | max 50 chars | Variable instance |

**Example - Get HeartbeatInterval:**
```json
{
  "charge_point_id": "CS001",
  "connector_id": 0,
  "feature_name": "GetVariables",
  "protocol_version": "ocpp2.0.1",
  "payload": "{\"getVariableData\":[{\"component\":{\"name\":\"OCPPCommCtrlr\"},\"variable\":{\"name\":\"HeartbeatInterval\"}}]}"
}
```

**Example - Get Multiple Variables:**
```json
{
  "charge_point_id": "CS001",
  "connector_id": 0,
  "feature_name": "GetVariables",
  "protocol_version": "ocpp2.0.1",
  "payload": "{\"getVariableData\":[{\"component\":{\"name\":\"OCPPCommCtrlr\"},\"variable\":{\"name\":\"HeartbeatInterval\"}},{\"component\":{\"name\":\"OCPPCommCtrlr\"},\"variable\":{\"name\":\"NetworkConfigurationPriority\"}}]}"
}
```

**Example - Get EVSE-specific Variable:**
```json
{
  "charge_point_id": "CS001",
  "connector_id": 0,
  "feature_name": "GetVariables",
  "protocol_version": "ocpp2.0.1",
  "payload": "{\"getVariableData\":[{\"component\":{\"name\":\"EVSE\",\"evse\":{\"id\":1}},\"variable\":{\"name\":\"Available\"}}]}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| getVariableResult | GetVariableResultType[] | Results for each requested variable |

**GetVariableResultType Structure:**

| Field | Type | Description |
|-------|------|-------------|
| attributeStatus | GetVariableStatusType | Result status |
| attributeType | AttributeType | Attribute type returned |
| attributeValue | string | Variable value (if successful) |
| component | ComponentType | Component reference |
| variable | VariableType | Variable reference |
| attributeStatusInfo | StatusInfo | Additional status info |

**GetVariableStatusType Values:**

| Status | Description |
|--------|-------------|
| Accepted | Variable retrieved successfully |
| Rejected | Request rejected |
| UnknownComponent | Component not found |
| UnknownVariable | Variable not found in component |
| NotSupportedAttributeType | Requested attribute type not supported |

**Example Response:**
```json
{
  "getVariableResult": [
    {
      "attributeStatus": "Accepted",
      "attributeType": "Actual",
      "attributeValue": "300",
      "component": {"name": "OCPPCommCtrlr"},
      "variable": {"name": "HeartbeatInterval"}
    }
  ]
}
```

#### Common Components and Variables

| Component | Variable | Description |
|-----------|----------|-------------|
| OCPPCommCtrlr | HeartbeatInterval | Heartbeat interval in seconds |
| OCPPCommCtrlr | NetworkConfigurationPriority | Network priority list |
| OCPPCommCtrlr | WebSocketPingInterval | WebSocket ping interval |
| AuthCtrlr | AuthorizeRemoteStart | Require authorization for remote start |
| AuthCtrlr | LocalAuthListEnabled | Enable local authorization list |
| TxCtrlr | StopTxOnInvalidId | Stop transaction on invalid ID |
| EVSE | Available | EVSE availability |
| EVSE | Power | EVSE power capability |
| Connector | Available | Connector availability |

---

### SetVariables

Modify device model variables on the charging station.

**Feature Name:** `SetVariables`

**Direction:** Central System -> Charging Station

#### Request

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| setVariableData | SetVariableDataType[] | Yes | List of variables to set |

**SetVariableDataType Structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| attributeType | AttributeType | No | Attribute to set (defaults to Actual) |
| attributeValue | string | Yes | New value (max 2500 chars) |
| component | ComponentType | Yes | Target component |
| variable | VariableType | Yes | Target variable |

**Example - Set HeartbeatInterval:**
```json
{
  "charge_point_id": "CS001",
  "connector_id": 0,
  "feature_name": "SetVariables",
  "protocol_version": "ocpp2.0.1",
  "payload": "{\"setVariableData\":[{\"component\":{\"name\":\"OCPPCommCtrlr\"},\"variable\":{\"name\":\"HeartbeatInterval\"},\"attributeValue\":\"300\"}]}"
}
```

**Example - Set Multiple Variables:**
```json
{
  "charge_point_id": "CS001",
  "connector_id": 0,
  "feature_name": "SetVariables",
  "protocol_version": "ocpp2.0.1",
  "payload": "{\"setVariableData\":[{\"component\":{\"name\":\"OCPPCommCtrlr\"},\"variable\":{\"name\":\"HeartbeatInterval\"},\"attributeValue\":\"300\"},{\"component\":{\"name\":\"AuthCtrlr\"},\"variable\":{\"name\":\"AuthorizeRemoteStart\"},\"attributeValue\":\"true\"}]}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| setVariableResult | SetVariableResultType[] | Results for each set operation |

**SetVariableResultType Structure:**

| Field | Type | Description |
|-------|------|-------------|
| attributeStatus | SetVariableStatusType | Result status |
| attributeType | AttributeType | Attribute type set |
| component | ComponentType | Component reference |
| variable | VariableType | Variable reference |
| attributeStatusInfo | StatusInfo | Additional status info |

**SetVariableStatusType Values:**

| Status | Description |
|--------|-------------|
| Accepted | Variable set successfully |
| Rejected | Set operation rejected |
| UnknownComponent | Component not found |
| UnknownVariable | Variable not found |
| NotSupportedAttributeType | Attribute type not supported |
| RebootRequired | Value accepted, reboot required to apply |

**Example Response:**
```json
{
  "setVariableResult": [
    {
      "attributeStatus": "Accepted",
      "attributeType": "Actual",
      "component": {"name": "OCPPCommCtrlr"},
      "variable": {"name": "HeartbeatInterval"}
    }
  ]
}
```

---

### Reset

Reset (reboot) the charging station or specific EVSE.

**Feature Name:** `Reset`

**Direction:** Central System -> Charging Station

#### Request

| Field | Type | Required | Constraints | Description |
|-------|------|----------|-------------|-------------|
| type | ResetType | Yes | - | Type of reset |
| evseId | integer | No | >= 1 | EVSE to reset (omit for full station reset) |

**ResetType Values:**

| Type | Description |
|------|-------------|
| Immediate | Immediate reset, all operations terminate |
| OnIdle | Reset when all transactions complete |

**Reset Entire Station (Immediate):**
```json
{
  "charge_point_id": "CS001",
  "connector_id": 0,
  "feature_name": "Reset",
  "protocol_version": "ocpp2.0.1",
  "payload": "{\"type\":\"Immediate\"}"
}
```

**Reset Station When Idle:**
```json
{
  "charge_point_id": "CS001",
  "connector_id": 0,
  "feature_name": "Reset",
  "protocol_version": "ocpp2.0.1",
  "payload": "{\"type\":\"OnIdle\"}"
}
```

**Reset Specific EVSE:**
```json
{
  "charge_point_id": "CS001",
  "connector_id": 0,
  "feature_name": "Reset",
  "protocol_version": "ocpp2.0.1",
  "payload": "{\"type\":\"Immediate\",\"evseId\":1}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| status | ResetStatusType | Result of reset request |
| statusInfo | StatusInfo | Additional status information |

**ResetStatusType Values:**

| Status | Description |
|--------|-------------|
| Accepted | Reset accepted, will be performed |
| Rejected | Reset rejected |
| Scheduled | Reset scheduled (for OnIdle type) |

**Example Response:**
```json
{
  "status": "Accepted"
}
```

---

## Remote Control Features

### RequestStartTransaction

Remotely start a charging session.

**Feature Name:** `RequestStartTransaction`

**Direction:** Central System -> Charging Station

#### Request

| Field | Type | Required | Constraints | Description |
|-------|------|----------|-------------|-------------|
| idToken | IdToken | Yes | - | Authorization token |
| remoteStartId | integer | Yes | - | Unique identifier for this request |
| evseId | integer | No | >= 1 | EVSE to use |
| chargingProfile | ChargingProfile | No | - | Charging profile to apply |
| groupIdToken | IdToken | No | - | Group authorization token |

**IdToken Structure:**

| Field | Type | Required | Constraints | Description |
|-------|------|----------|-------------|-------------|
| idToken | string | Yes | max 36 chars | Token identifier |
| type | IdTokenType | Yes | - | Token type |
| additionalInfo | AdditionalInfo[] | No | - | Additional token info |

**IdTokenType Values:**

| Type | Description |
|------|-------------|
| Central | Centrally managed ID |
| eMAID | e-Mobility Account Identifier |
| ISO14443 | RFID card |
| ISO15693 | RFID card (alternative) |
| KeyCode | PIN code |
| Local | Local system ID |
| MacAddress | MAC address |
| NoAuthorization | No authorization required |

**Basic Request:**
```json
{
  "charge_point_id": "CS001",
  "connector_id": 0,
  "feature_name": "RequestStartTransaction",
  "protocol_version": "ocpp2.0.1",
  "payload": "{\"idToken\":{\"idToken\":\"USER001\",\"type\":\"Central\"},\"remoteStartId\":12345}"
}
```

**Request with EVSE Selection:**
```json
{
  "charge_point_id": "CS001",
  "connector_id": 0,
  "feature_name": "RequestStartTransaction",
  "protocol_version": "ocpp2.0.1",
  "payload": "{\"idToken\":{\"idToken\":\"USER001\",\"type\":\"Central\"},\"remoteStartId\":12345,\"evseId\":1}"
}
```

**Request with RFID Token:**
```json
{
  "charge_point_id": "CS001",
  "connector_id": 0,
  "feature_name": "RequestStartTransaction",
  "protocol_version": "ocpp2.0.1",
  "payload": "{\"idToken\":{\"idToken\":\"04A2B3C4D5E6F7\",\"type\":\"ISO14443\"},\"remoteStartId\":12346,\"evseId\":1}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| status | RequestStartStopStatusType | Result of request |
| statusInfo | StatusInfo | Additional status information |
| transactionId | string | Transaction ID if started (max 36 chars) |

**RequestStartStopStatusType Values:**

| Status | Description |
|--------|-------------|
| Accepted | Request accepted |
| Rejected | Request rejected |

**Example Response:**
```json
{
  "status": "Accepted",
  "transactionId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

---

### RequestStopTransaction

Remotely stop an active charging session.

**Feature Name:** `RequestStopTransaction`

**Direction:** Central System -> Charging Station

#### Request

| Field | Type | Required | Constraints | Description |
|-------|------|----------|-------------|-------------|
| transactionId | string | Yes | max 36 chars | Transaction to stop |

**Request Format:**
```json
{
  "charge_point_id": "CS001",
  "connector_id": 0,
  "feature_name": "RequestStopTransaction",
  "protocol_version": "ocpp2.0.1",
  "payload": "{\"transactionId\":\"a1b2c3d4-e5f6-7890-abcd-ef1234567890\"}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| status | RequestStartStopStatusType | Result of request |
| statusInfo | StatusInfo | Additional status information |

**Example Response:**
```json
{
  "status": "Accepted"
}
```

---

## Incoming Messages (Charge Point -> Central System)

These messages are sent by charging stations to the central system.

### BootNotification

Sent when a charging station boots or reconnects.

**ChargingStation Structure:**

| Field | Type | Description |
|-------|------|-------------|
| model | string | Model name (max 20 chars) |
| vendorName | string | Vendor name (max 50 chars) |
| serialNumber | string | Serial number (max 25 chars) |
| firmwareVersion | string | Firmware version (max 50 chars) |
| modem | Modem | Modem information |

**BootReasonType Values:** ApplicationReset, FirmwareUpdate, LocalReset, PowerUp, RemoteReset, ScheduledReset, Triggered, Unknown, Watchdog

### Heartbeat

Periodic keepalive message. No payload.

### StatusNotification

Connector status update.

| Field | Type | Description |
|-------|------|-------------|
| timestamp | DateTime | Status change time |
| connectorStatus | ConnectorStatusType | Current status |
| evseId | integer | EVSE identifier |
| connectorId | integer | Connector identifier |

**ConnectorStatusType Values:** Available, Occupied, Reserved, Unavailable, Faulted

### TransactionEvent

Transaction state changes (replaces Start/StopTransaction from 1.6).

| Field | Type | Description |
|-------|------|-------------|
| eventType | TransactionEventType | Type of event |
| timestamp | DateTime | Event time |
| triggerReason | TriggerReasonType | Why event was triggered |
| seqNo | integer | Sequence number |
| transactionInfo | Transaction | Transaction details |
| offline | boolean | Whether event occurred offline |
| numberOfPhasesUsed | integer | Phases used (1-3) |
| cableMaxCurrent | decimal | Cable max current |
| reservationId | integer | Associated reservation |
| idToken | IdToken | Authorization token |
| evse | EVSE | EVSE information |
| meterValue | MeterValue[] | Meter readings |

**TransactionEventType Values:**

| Type | Description |
|------|-------------|
| Started | Transaction started |
| Updated | Transaction update (meter values, etc.) |
| Ended | Transaction ended |

**TriggerReasonType Values:** Authorized, CablePluggedIn, ChargingRateChanged, ChargingStateChanged, Deauthorized, EnergyLimitReached, EVCommunicationLost, EVConnectTimeout, MeterValueClock, MeterValuePeriodic, TimeLimitReached, Trigger, UnlockCommand, StopAuthorized, EVDeparted, EVDetected, RemoteStart, RemoteStop, AbnormalCondition, SignedDataReceived, ResetCommand

### NotifyReport

Report device model configuration.

| Field | Type | Description |
|-------|------|-------------|
| requestId | integer | Original request ID |
| generatedAt | DateTime | Report generation time |
| tbc | boolean | To be continued flag |
| seqNo | integer | Sequence number |
| reportData | ReportDataType[] | Reported data |

### Authorize

Request authorization for a token.

| Field | Type | Description |
|-------|------|-------------|
| idToken | IdToken | Token to authorize |
| certificate | string | ISO 15118 certificate |
| iso15118CertificateHashData | OCSPRequestDataType[] | Certificate hash data |

---

## Common Types

### DateTime

ISO 8601 formatted timestamp: `2024-01-15T14:30:00Z`

### StatusInfo

Additional status information.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| reasonCode | string | max 20 chars | Standardized reason code |
| additionalInfo | string | max 512 chars | Additional information |

### EVSE

EVSE reference.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | integer | >= 1 | EVSE identifier |
| connectorId | integer | >= 1 | Connector identifier (optional) |

### IdTokenInfo

Authorization response information.

| Field | Type | Description |
|-------|------|-------------|
| status | AuthorizationStatusType | Authorization result |
| cacheExpiryDateTime | DateTime | When cache entry expires |
| chargingPriority | integer | Charging priority (-9 to 9) |
| language1 | string | Preferred language |
| evseId | integer[] | Authorized EVSEs |
| groupIdToken | IdToken | Group token |
| personalMessage | MessageContent | Message to display |

**AuthorizationStatusType Values:**

| Status | Description |
|--------|-------------|
| Accepted | Token authorized |
| Blocked | Token blocked |
| ConcurrentTx | Token already in use |
| Expired | Token expired |
| Invalid | Token invalid |
| NoCredit | No credit available |
| NotAllowedTypeEVSE | Not allowed on this EVSE type |
| NotAtThisLocation | Not valid at this location |
| NotAtThisTime | Not valid at this time |
| Unknown | Token unknown |

### Transaction

Transaction information.

| Field | Type | Description |
|-------|------|-------------|
| transactionId | string | Unique transaction ID (max 36 chars) |
| chargingState | ChargingStateType | Current charging state |
| timeSpentCharging | integer | Seconds spent charging |
| stoppedReason | ReasonType | Why transaction stopped |
| remoteStartId | integer | Remote start request ID |

**ChargingStateType Values:** Charging, EVConnected, SuspendedEV, SuspendedEVSE, Idle

### MeterValue

Collection of meter measurements.

| Field | Type | Description |
|-------|------|-------------|
| timestamp | DateTime | Measurement time |
| sampledValue | SampledValue[] | Individual measurements |

### SampledValue

Individual meter measurement.

| Field | Type | Description |
|-------|------|-------------|
| value | decimal | Measured value |
| context | ReadingContextType | Measurement context |
| measurand | MeasurandType | What was measured |
| phase | PhaseType | AC phase |
| location | LocationType | Measurement point |
| signedMeterValue | SignedMeterValue | Signed meter data |
| unitOfMeasure | UnitOfMeasure | Unit |

**Common Measurand Values:** Energy.Active.Import.Register, Power.Active.Import, Current.Import, Voltage, SoC

### ChargingProfile (OCPP 2.0.1)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | integer | Yes | Profile ID |
| stackLevel | integer | Yes | Priority (0-255) |
| chargingProfilePurpose | ChargingProfilePurposeType | Yes | Purpose |
| chargingProfileKind | ChargingProfileKindType | Yes | Schedule type |
| recurrencyKind | RecurrencyKindType | No | Recurrence |
| validFrom | DateTime | No | Validity start |
| validTo | DateTime | No | Validity end |
| transactionId | string | No | Associated transaction |
| chargingSchedule | ChargingSchedule[] | Yes | Schedules |

**ChargingProfilePurposeType Values:** ChargingStationExternalConstraints, ChargingStationMaxProfile, TxDefaultProfile, TxProfile

### AttributeType

Variable attribute types.

| Type | Description |
|------|-------------|
| Actual | Current actual value |
| Target | Target/setpoint value |
| MinSet | Minimum allowed setpoint |
| MaxSet | Maximum allowed setpoint |

---

## Migration from OCPP 1.6

### Configuration -> Variables

| OCPP 1.6 | OCPP 2.0.1 Component | OCPP 2.0.1 Variable |
|----------|---------------------|---------------------|
| GetConfiguration | GetVariables | Various |
| ChangeConfiguration | SetVariables | Various |
| HeartbeatInterval | OCPPCommCtrlr | HeartbeatInterval |
| AuthorizeRemoteTxRequests | AuthCtrlr | AuthorizeRemoteStart |
| LocalPreAuthorize | AuthCtrlr | LocalPreAuthorize |

### Transactions

| OCPP 1.6 | OCPP 2.0.1 |
|----------|------------|
| RemoteStartTransaction | RequestStartTransaction |
| RemoteStopTransaction | RequestStopTransaction |
| StartTransaction (incoming) | TransactionEvent (Started) |
| StopTransaction (incoming) | TransactionEvent (Ended) |
| MeterValues (incoming) | TransactionEvent (Updated) |

### Reset Types

| OCPP 1.6 | OCPP 2.0.1 |
|----------|------------|
| Soft | OnIdle |
| Hard | Immediate |
