# OCPP 1.6 API Reference

This document describes all OCPP 1.6J features supported by EVSYS API.

## Table of Contents

- [Core Features](#core-features)
  - [GetConfiguration](#getconfiguration)
  - [ChangeConfiguration](#changeconfiguration)
  - [RemoteStartTransaction](#remotestarttransaction)
  - [RemoteStopTransaction](#remotestoptransaction)
  - [Reset](#reset)
  - [UnlockConnector](#unlockconnector)
  - [DataTransfer](#datatransfer)
- [Smart Charging Features](#smart-charging-features)
  - [SetChargingProfile](#setchargingprofile)
  - [ClearChargingProfile](#clearchargingprofile)
  - [GetCompositeSchedule](#getcompositeschedule)
- [Remote Trigger Features](#remote-trigger-features)
  - [TriggerMessage](#triggermessage)
- [Local Authorization Features](#local-authorization-features)
  - [SendLocalList](#sendlocallist)
  - [GetLocalListVersion](#getlocallistversion)
- [Firmware Management Features](#firmware-management-features)
  - [GetDiagnostics](#getdiagnostics)
  - [UpdateFirmware](#updatefirmware)
- [Incoming Messages](#incoming-messages-charge-point--central-system)
- [Common Types](#common-types)

---

## Core Features

### GetConfiguration

Retrieve configuration values from a charge point.

**Feature Name:** `GetConfiguration`

**Direction:** Central System -> Charge Point

#### Request

**Payload:** Configuration key name(s) to retrieve. Pass empty string or omit to get all keys.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| key | string[] | No | List of configuration keys to retrieve. If omitted, returns all keys. |

**Simple request (single key as string):**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "GetConfiguration",
  "payload": "HeartbeatInterval"
}
```

**Request for all keys:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "GetConfiguration",
  "payload": ""
}
```

**Request for multiple keys:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "GetConfiguration",
  "payload": "{\"key\":[\"HeartbeatInterval\",\"MeterValueSampleInterval\"]}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| configurationKey | ConfigurationKey[] | List of known configuration keys with values |
| unknownKey | string[] | List of requested keys not recognized by charge point |

**ConfigurationKey Structure:**

| Field | Type | Description |
|-------|------|-------------|
| key | string | Configuration key name (max 50 chars) |
| readonly | boolean | Whether the key can be modified |
| value | string | Current value (max 500 chars, omitted if write-only) |

**Example Response:**
```json
{
  "configurationKey": [
    {
      "key": "HeartbeatInterval",
      "readonly": false,
      "value": "300"
    },
    {
      "key": "MeterValueSampleInterval",
      "readonly": false,
      "value": "60"
    }
  ],
  "unknownKey": []
}
```

#### Common Configuration Keys

| Key | Description | Typical Values |
|-----|-------------|----------------|
| HeartbeatInterval | Seconds between heartbeats | 60-3600 |
| MeterValueSampleInterval | Seconds between meter value samples | 10-300 |
| ConnectionTimeOut | Max connection timeout in seconds | 30-120 |
| AuthorizeRemoteTxRequests | Require auth for remote start | true/false |
| LocalPreAuthorize | Enable local pre-authorization | true/false |
| StopTransactionOnInvalidId | Stop transaction if ID becomes invalid | true/false |

---

### ChangeConfiguration

Modify a configuration value on the charge point.

**Feature Name:** `ChangeConfiguration`

**Direction:** Central System -> Charge Point

#### Request

| Field | Type | Required | Constraints | Description |
|-------|------|----------|-------------|-------------|
| key | string | Yes | max 50 chars | Configuration key to modify |
| value | string | Yes | max 500 chars | New value for the key |

**Request Format:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "ChangeConfiguration",
  "payload": "{\"key\":\"HeartbeatInterval\",\"value\":\"300\"}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| status | ConfigurationStatus | Result of the configuration change |

**ConfigurationStatus Values:**

| Status | Description |
|--------|-------------|
| Accepted | Configuration change accepted and applied |
| Rejected | Configuration change rejected (read-only or invalid) |
| RebootRequired | Configuration accepted but requires reboot to apply |
| NotSupported | Configuration key not supported by charge point |

**Example Response:**
```json
{
  "status": "Accepted"
}
```

---

### RemoteStartTransaction

Remotely start a charging session on a charge point.

**Feature Name:** `RemoteStartTransaction`

**Direction:** Central System -> Charge Point

#### Request

| Field | Type | Required | Constraints | Description |
|-------|------|----------|-------------|-------------|
| connectorId | integer | No | > 0 | Connector to start charging on |
| idTag | string | Yes | max 20 chars | Authorization token for the session |
| chargingProfile | ChargingProfile | No | - | Optional charging profile to apply |

**Basic Request:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 1,
  "feature_name": "RemoteStartTransaction",
  "payload": "{\"idTag\":\"USER001\"}"
}
```

**Request with Connector:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 1,
  "feature_name": "RemoteStartTransaction",
  "payload": "{\"idTag\":\"USER001\",\"connectorId\":1}"
}
```

**Request with Charging Profile:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 1,
  "feature_name": "RemoteStartTransaction",
  "payload": "{\"idTag\":\"USER001\",\"connectorId\":1,\"chargingProfile\":{\"chargingProfileId\":1,\"stackLevel\":0,\"chargingProfilePurpose\":\"TxProfile\",\"chargingProfileKind\":\"Absolute\",\"chargingSchedule\":{\"chargingRateUnit\":\"A\",\"chargingSchedulePeriod\":[{\"startPeriod\":0,\"limit\":16.0}]}}}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| status | RemoteStartStopStatus | Result of the start request |

**RemoteStartStopStatus Values:**

| Status | Description |
|--------|-------------|
| Accepted | Request accepted, transaction will start |
| Rejected | Request rejected (connector unavailable, invalid ID, etc.) |

**Example Response:**
```json
{
  "status": "Accepted"
}
```

#### Notes

- If `connectorId` is omitted, the charge point selects an available connector
- The `idTag` must be valid (accepted via Authorize) unless `accept_unknown_tag` is enabled
- The charging profile, if provided, applies only to this transaction

---

### RemoteStopTransaction

Remotely stop an active charging session.

**Feature Name:** `RemoteStopTransaction`

**Direction:** Central System -> Charge Point

#### Request

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| transactionId | integer | Yes | ID of the transaction to stop |

**Request Format:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "RemoteStopTransaction",
  "payload": "{\"transactionId\":12345}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| status | RemoteStartStopStatus | Result of the stop request |

**Example Response:**
```json
{
  "status": "Accepted"
}
```

#### Notes

- The transaction ID is returned when the charging session starts
- Use `GetServerStatus` or query database to find active transaction IDs

---

### Reset

Reset (reboot) a charge point.

**Feature Name:** `Reset`

**Direction:** Central System -> Charge Point

#### Request

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| type | ResetType | Yes | Type of reset to perform |

**ResetType Values:**

| Type | Description |
|------|-------------|
| Soft | Graceful restart, ongoing transactions continue if possible |
| Hard | Immediate restart, all operations terminate |

**Soft Reset:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "Reset",
  "payload": "{\"type\":\"Soft\"}"
}
```

**Hard Reset:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "Reset",
  "payload": "{\"type\":\"Hard\"}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| status | ResetStatus | Result of the reset request |

**ResetStatus Values:**

| Status | Description |
|--------|-------------|
| Accepted | Reset command accepted, charge point will restart |
| Rejected | Reset command rejected |

**Example Response:**
```json
{
  "status": "Accepted"
}
```

---

### UnlockConnector

Unlock a specific connector on the charge point.

**Feature Name:** `UnlockConnector`

**Direction:** Central System -> Charge Point

#### Request

| Field | Type | Required | Constraints | Description |
|-------|------|----------|-------------|-------------|
| connectorId | integer | Yes | > 0 | Connector to unlock |

**Request Format:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 1,
  "feature_name": "UnlockConnector",
  "payload": "{\"connectorId\":1}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| status | UnlockStatus | Result of the unlock request |

**UnlockStatus Values:**

| Status | Description |
|--------|-------------|
| Unlocked | Connector successfully unlocked |
| UnlockFailed | Failed to unlock connector |
| NotSupported | Connector unlocking not supported |

**Example Response:**
```json
{
  "status": "Unlocked"
}
```

---

### DataTransfer

Send vendor-specific data to/from charge point.

**Feature Name:** `DataTransfer`

**Direction:** Bidirectional (Central System <-> Charge Point)

#### Request

| Field | Type | Required | Constraints | Description |
|-------|------|----------|-------------|-------------|
| vendorId | string | Yes | max 255 chars | Vendor identifier |
| messageId | string | No | max 50 chars | Message type identifier |
| data | any | No | - | Vendor-specific data |

**Request Format:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "DataTransfer",
  "payload": "{\"vendorId\":\"VendorX\",\"messageId\":\"CustomMessage\",\"data\":{\"customField\":\"value\"}}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| status | DataTransferStatus | Result of the data transfer |
| data | any | Optional response data |

**DataTransferStatus Values:**

| Status | Description |
|--------|-------------|
| Accepted | Message accepted and processed |
| Rejected | Message rejected |
| UnknownMessageId | Message ID not recognized |
| UnknownVendorId | Vendor ID not recognized |

**Example Response:**
```json
{
  "status": "Accepted",
  "data": {
    "responseField": "responseValue"
  }
}
```

---

## Smart Charging Features

### SetChargingProfile

Apply a charging profile to control power delivery.

**Feature Name:** `SetChargingProfile`

**Direction:** Central System -> Charge Point

#### Request

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| connectorId | integer | Yes | Connector (0 for charge point level) |
| csChargingProfiles | ChargingProfile | Yes | Charging profile to apply |

**ChargingProfile Structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| chargingProfileId | integer | Yes | Unique profile identifier |
| transactionId | integer | No | Link to specific transaction |
| stackLevel | integer | Yes | Priority level (0-255, higher = higher priority) |
| chargingProfilePurpose | ChargingProfilePurposeType | Yes | Purpose of the profile |
| chargingProfileKind | ChargingProfileKindType | Yes | How schedule is defined |
| recurrencyKind | RecurrencyKindType | No | Recurrence pattern |
| validFrom | DateTime | No | Profile validity start |
| validTo | DateTime | No | Profile validity end |
| chargingSchedule | ChargingSchedule | Yes | The actual schedule |

**ChargingProfilePurposeType Values:**

| Value | Description |
|-------|-------------|
| ChargePointMaxProfile | Max power for entire charge point |
| TxDefaultProfile | Default profile for new transactions |
| TxProfile | Profile for specific transaction |

**ChargingProfileKindType Values:**

| Value | Description |
|-------|-------------|
| Absolute | Fixed schedule with specific times |
| Recurring | Repeating schedule (daily/weekly) |
| Relative | Schedule relative to transaction start |

**ChargingSchedule Structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| duration | integer | No | Schedule duration in seconds |
| startSchedule | DateTime | No | Absolute schedule start time |
| chargingRateUnit | ChargingRateUnitType | Yes | Unit for power limits (A or W) |
| chargingSchedulePeriod | ChargingSchedulePeriod[] | Yes | Power limit periods |
| minChargingRate | decimal | No | Minimum charging rate |

**ChargingSchedulePeriod Structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| startPeriod | integer | Yes | Seconds from schedule start |
| limit | decimal | Yes | Power limit in schedule unit |
| numberPhases | integer | No | Number of phases (1-3) |

**Example - Set 16A Limit:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 1,
  "feature_name": "SetChargingProfile",
  "payload": "{\"connectorId\":1,\"csChargingProfiles\":{\"chargingProfileId\":1,\"stackLevel\":0,\"chargingProfilePurpose\":\"TxDefaultProfile\",\"chargingProfileKind\":\"Absolute\",\"chargingSchedule\":{\"chargingRateUnit\":\"A\",\"chargingSchedulePeriod\":[{\"startPeriod\":0,\"limit\":16.0}]}}}"
}
```

**Example - Time-based Limits:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "SetChargingProfile",
  "payload": "{\"connectorId\":0,\"csChargingProfiles\":{\"chargingProfileId\":2,\"stackLevel\":1,\"chargingProfilePurpose\":\"ChargePointMaxProfile\",\"chargingProfileKind\":\"Absolute\",\"chargingSchedule\":{\"chargingRateUnit\":\"A\",\"chargingSchedulePeriod\":[{\"startPeriod\":0,\"limit\":32.0},{\"startPeriod\":3600,\"limit\":16.0},{\"startPeriod\":7200,\"limit\":32.0}]}}}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| status | ChargingProfileStatus | Result of profile application |

**ChargingProfileStatus Values:**

| Status | Description |
|--------|-------------|
| Accepted | Profile accepted and applied |
| Rejected | Profile rejected |
| NotSupported | Smart charging not supported |

---

### ClearChargingProfile

Remove charging profiles from the charge point.

**Feature Name:** `ClearChargingProfile`

**Direction:** Central System -> Charge Point

#### Request

All fields are optional. If no fields specified, all profiles are cleared.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | integer | No | Specific profile ID to clear |
| connectorId | integer | No | Clear profiles on specific connector |
| chargingProfilePurpose | ChargingProfilePurposeType | No | Clear profiles with specific purpose |
| stackLevel | integer | No | Clear profiles at specific stack level |

**Clear All Profiles:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "ClearChargingProfile",
  "payload": "{}"
}
```

**Clear Specific Profile:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "ClearChargingProfile",
  "payload": "{\"id\":1}"
}
```

**Clear by Purpose:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "ClearChargingProfile",
  "payload": "{\"chargingProfilePurpose\":\"TxDefaultProfile\"}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| status | ClearChargingProfileStatus | Result of clear operation |

**ClearChargingProfileStatus Values:**

| Status | Description |
|--------|-------------|
| Accepted | Matching profiles cleared |
| Unknown | No matching profiles found |

---

### GetCompositeSchedule

Request the composite charging schedule for a connector.

**Feature Name:** `GetCompositeSchedule`

**Direction:** Central System -> Charge Point

#### Request

| Field | Type | Required | Constraints | Description |
|-------|------|----------|-------------|-------------|
| connectorId | integer | Yes | >= 0 | Connector (0 for charge point) |
| duration | integer | Yes | >= 0 | Duration in seconds to retrieve |
| chargingRateUnit | ChargingRateUnitType | No | A or W | Preferred unit for limits |

**Request Format:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 1,
  "feature_name": "GetCompositeSchedule",
  "payload": "{\"connectorId\":1,\"duration\":3600,\"chargingRateUnit\":\"A\"}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| status | GetCompositeScheduleStatus | Result of request |
| connectorId | integer | Connector for the schedule |
| scheduleStart | DateTime | Start time of schedule |
| chargingSchedule | ChargingSchedule | Calculated composite schedule |

**GetCompositeScheduleStatus Values:**

| Status | Description |
|--------|-------------|
| Accepted | Schedule returned successfully |
| Rejected | Request rejected |

**Example Response:**
```json
{
  "status": "Accepted",
  "connectorId": 1,
  "scheduleStart": "2024-01-15T10:00:00Z",
  "chargingSchedule": {
    "chargingRateUnit": "A",
    "chargingSchedulePeriod": [
      {"startPeriod": 0, "limit": 16.0},
      {"startPeriod": 1800, "limit": 32.0}
    ]
  }
}
```

---

## Remote Trigger Features

### TriggerMessage

Request the charge point to send a specific message.

**Feature Name:** `TriggerMessage`

**Direction:** Central System -> Charge Point

#### Request

| Field | Type | Required | Constraints | Description |
|-------|------|----------|-------------|-------------|
| requestedMessage | MessageTrigger | Yes | - | Type of message to trigger |
| connectorId | integer | No | > 0 | Connector for connector-specific messages |

**MessageTrigger Values:**

| Value | Description |
|-------|-------------|
| BootNotification | Trigger BootNotification message |
| DiagnosticsStatusNotification | Trigger diagnostics status |
| FirmwareStatusNotification | Trigger firmware status |
| Heartbeat | Trigger Heartbeat message |
| MeterValues | Trigger MeterValues for connector |
| StatusNotification | Trigger StatusNotification |

**Trigger Heartbeat:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "TriggerMessage",
  "payload": "{\"requestedMessage\":\"Heartbeat\"}"
}
```

**Trigger MeterValues for Connector:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 1,
  "feature_name": "TriggerMessage",
  "payload": "{\"requestedMessage\":\"MeterValues\",\"connectorId\":1}"
}
```

**Trigger StatusNotification:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 1,
  "feature_name": "TriggerMessage",
  "payload": "{\"requestedMessage\":\"StatusNotification\",\"connectorId\":1}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| status | TriggerMessageStatus | Result of trigger request |

**TriggerMessageStatus Values:**

| Status | Description |
|--------|-------------|
| Accepted | Trigger accepted, message will be sent |
| Rejected | Trigger rejected |
| NotImplemented | Requested message type not supported |

---

## Local Authorization Features

### SendLocalList

Update the local authorization list on the charge point.

**Feature Name:** `SendLocalList`

**Direction:** Central System -> Charge Point

#### Request

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| listVersion | integer | Yes | Version number of the list (>= 0) |
| localAuthorizationList | AuthorizationData[] | No | List of authorization entries |
| updateType | UpdateType | Yes | Type of update (Full or Differential) |

**AuthorizationData Structure:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| idTag | string | Yes | Authorization token (max 20 chars) |
| idTagInfo | IdTagInfo | No | Authorization status info |

**UpdateType Values:**

| Type | Description |
|------|-------------|
| Differential | Add/update only specified entries |
| Full | Replace entire list with provided entries |

**Full List Update:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "SendLocalList",
  "payload": "{\"listVersion\":1,\"updateType\":\"Full\",\"localAuthorizationList\":[{\"idTag\":\"USER001\",\"idTagInfo\":{\"status\":\"Accepted\"}},{\"idTag\":\"USER002\",\"idTagInfo\":{\"status\":\"Accepted\"}}]}"
}
```

**Differential Update:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "SendLocalList",
  "payload": "{\"listVersion\":2,\"updateType\":\"Differential\",\"localAuthorizationList\":[{\"idTag\":\"USER003\",\"idTagInfo\":{\"status\":\"Accepted\"}}]}"
}
```

**Clear List:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "SendLocalList",
  "payload": "{\"listVersion\":3,\"updateType\":\"Full\",\"localAuthorizationList\":[]}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| status | UpdateStatus | Result of list update |

**UpdateStatus Values:**

| Status | Description |
|--------|-------------|
| Accepted | List update accepted |
| Failed | List update failed |
| NotSupported | Local authorization list not supported |
| VersionMismatch | Provided version conflicts with stored version |

---

### GetLocalListVersion

Get the current version of the local authorization list.

**Feature Name:** `GetLocalListVersion`

**Direction:** Central System -> Charge Point

#### Request

No payload required.

```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "GetLocalListVersion",
  "payload": ""
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| listVersion | integer | Current list version (-1 if not supported) |

**Example Response:**
```json
{
  "listVersion": 2
}
```

---

## Firmware Management Features

### GetDiagnostics

Request the charge point to upload diagnostics.

**Feature Name:** `GetDiagnostics`

**Direction:** Central System -> Charge Point

#### Request

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| location | string | Yes | URI where diagnostics should be uploaded |
| retries | integer | No | Number of upload retries (>= 0) |
| retryInterval | integer | No | Seconds between retries (>= 0) |
| startTime | DateTime | No | Start of log period to retrieve |
| stopTime | DateTime | No | End of log period to retrieve |

**Basic Request:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "GetDiagnostics",
  "payload": "{\"location\":\"ftp://server.example.com/diagnostics/\"}"
}
```

**Request with Time Range:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "GetDiagnostics",
  "payload": "{\"location\":\"ftp://server.example.com/diagnostics/\",\"startTime\":\"2024-01-01T00:00:00Z\",\"stopTime\":\"2024-01-15T00:00:00Z\",\"retries\":3,\"retryInterval\":60}"
}
```

#### Response

| Field | Type | Description |
|-------|------|-------------|
| fileName | string | Name of the diagnostics file (empty if not available) |

**Example Response:**
```json
{
  "fileName": "CP001_diag_20240115.zip"
}
```

#### Notes

- The charge point will report upload progress via `DiagnosticsStatusNotification`
- Supported URI schemes depend on charge point implementation (FTP, FTPS, HTTP, HTTPS)

---

### UpdateFirmware

Request the charge point to download and install firmware.

**Feature Name:** `UpdateFirmware`

**Direction:** Central System -> Charge Point

#### Request

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| location | string | Yes | URI of the firmware file |
| retrieveDate | DateTime | Yes | When to start download |
| retries | integer | No | Number of download retries |
| retryInterval | integer | No | Seconds between retries |

**Request Format:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "UpdateFirmware",
  "payload": "{\"location\":\"https://firmware.example.com/cp001_v2.0.bin\",\"retrieveDate\":\"2024-01-15T02:00:00Z\",\"retries\":3,\"retryInterval\":300}"
}
```

#### Response

No response payload. HTTP 204 indicates command accepted.

#### Notes

- The charge point will report progress via `FirmwareStatusNotification`
- Schedule updates during low-usage periods
- Ensure firmware URI is accessible from the charge point network

---

## Incoming Messages (Charge Point -> Central System)

These messages are sent by charge points to the central system. They are documented here for reference when interpreting system events.

### BootNotification

Sent when a charge point boots or reconnects.

| Field | Type | Description |
|-------|------|-------------|
| chargePointVendor | string | Manufacturer name |
| chargePointModel | string | Model designation |
| chargePointSerialNumber | string | Serial number |
| chargeBoxSerialNumber | string | Charge box serial |
| firmwareVersion | string | Current firmware version |
| iccid | string | SIM card identifier |
| imsi | string | Mobile subscriber identity |
| meterType | string | Main meter type |
| meterSerialNumber | string | Main meter serial |

### Authorize

Request authorization for an ID tag.

| Field | Type | Description |
|-------|------|-------------|
| idTag | string | Authorization token (max 20 chars) |

### StartTransaction

Notification that a charging session has started.

| Field | Type | Description |
|-------|------|-------------|
| connectorId | integer | Connector used (> 0) |
| idTag | string | Authorization token |
| meterStart | integer | Starting meter value (Wh) |
| reservationId | integer | Reservation ID if applicable |
| timestamp | DateTime | Session start time |

### StopTransaction

Notification that a charging session has ended.

| Field | Type | Description |
|-------|------|-------------|
| idTag | string | Authorization token |
| meterStop | integer | Final meter value (Wh) |
| timestamp | DateTime | Session end time |
| transactionId | integer | Transaction identifier |
| reason | Reason | Why the transaction stopped |
| transactionData | MeterValue[] | Final meter readings |

**Reason Values:** DeAuthorized, EmergencyStop, EVDisconnected, HardReset, Local, Other, PowerLoss, Reboot, Remote, SoftReset, UnlockCommand

### StatusNotification

Connector status update.

| Field | Type | Description |
|-------|------|-------------|
| connectorId | integer | Connector (0 = charge point) |
| errorCode | ChargePointErrorCode | Error condition |
| info | string | Additional info |
| status | ChargePointStatus | Current status |
| timestamp | DateTime | Status change time |
| vendorId | string | Vendor identifier |
| vendorErrorCode | string | Vendor-specific error |

**ChargePointStatus Values:** Available, Preparing, Charging, SuspendedEVSE, SuspendedEV, Finishing, Reserved, Unavailable, Faulted

**ChargePointErrorCode Values:** NoError, ConnectorLockFailure, EVCommunicationError, GroundFailure, HighTemperature, InternalError, LocalListConflict, OtherError, OverCurrentFailure, OverVoltage, PowerMeterFailure, PowerSwitchFailure, ReaderFailure, ResetFailure, UnderVoltage, WeakSignal

### MeterValues

Periodic or triggered meter value samples.

| Field | Type | Description |
|-------|------|-------------|
| connectorId | integer | Connector (>= 0) |
| transactionId | integer | Associated transaction |
| meterValue | MeterValue[] | Sampled values |

### Heartbeat

Periodic keepalive message.

No payload. Response contains current server time.

---

## Common Types

### DateTime

ISO 8601 formatted timestamp: `2024-01-15T14:30:00Z`

### IdTagInfo

Authorization status information.

| Field | Type | Description |
|-------|------|-------------|
| expiryDate | DateTime | When authorization expires |
| parentIdTag | string | Parent group tag |
| status | AuthorizationStatus | Authorization result |

**AuthorizationStatus Values:**

| Status | Description |
|--------|-------------|
| Accepted | Tag authorized |
| Blocked | Tag blocked |
| Expired | Tag expired |
| Invalid | Tag invalid |
| ConcurrentTx | Tag already in use |

### MeterValue

Collection of sampled meter values.

| Field | Type | Description |
|-------|------|-------------|
| timestamp | DateTime | Sample time |
| sampledValue | SampledValue[] | Individual measurements |

### SampledValue

Individual meter measurement.

| Field | Type | Description |
|-------|------|-------------|
| value | string | Measured value |
| context | ReadingContext | Why reading was taken |
| format | ValueFormat | Raw or SignedData |
| measurand | Measurand | What was measured |
| phase | Phase | AC phase |
| location | Location | Measurement point |
| unit | UnitOfMeasure | Unit of measurement |

**Common Measurand Values:** Energy.Active.Import.Register, Power.Active.Import, Current.Import, Voltage, SoC
