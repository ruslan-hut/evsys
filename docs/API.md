# EVSYS REST API Documentation

EVSYS provides a REST API for sending commands to connected charge points. The API acts as a bridge between external applications and OCPP-compliant charging stations.

## Table of Contents

- [Overview](#overview)
- [Authentication](#authentication)
- [Endpoint](#endpoint)
- [Request Format](#request-format)
- [Response Format](#response-format)
- [Error Handling](#error-handling)
- [Protocol Versions](#protocol-versions)
- [Feature Reference](#feature-reference)

## Overview

The API provides a unified interface for:
- Sending OCPP commands to charge points
- Querying charge point configuration
- Remote start/stop of charging sessions
- Managing charging profiles
- Retrieving server status

All communication uses JSON over HTTP/HTTPS.

## Authentication

Currently, the API does not implement authentication. When deployed in production, it is recommended to:
- Use TLS (configure `tls_enabled: true` in API settings)
- Place behind a reverse proxy with authentication
- Restrict access via firewall rules

## Endpoint

**URL:** `POST http://<server>:<port>/api`

Default port is `5001` (configurable via `api.port` in config.yml).

**Supported Methods:**
- `POST` - Send commands to charge points

**Content-Type:** `application/json`

## Request Format

All requests follow a unified structure:

```json
{
  "charge_point_id": "string",
  "connector_id": 0,
  "feature_name": "string",
  "payload": "string",
  "protocol_version": "string"
}
```

### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `charge_point_id` | string | Yes | Unique identifier of the target charge point |
| `connector_id` | integer | Yes | Connector number (0 for charge point level commands, >0 for specific connector) |
| `feature_name` | string | Yes | OCPP feature/action name (e.g., "GetConfiguration", "RemoteStartTransaction") |
| `payload` | string | Depends | JSON-encoded payload for the command (some commands require no payload) |
| `protocol_version` | string | No | OCPP protocol version: "ocpp1.6", "ocpp2.0.1", or "ocpp2.1" |

### Protocol Version Resolution

If `protocol_version` is not specified:
1. System checks if the charge point has an active connection
2. Uses the protocol version from the active connection
3. Falls back to OCPP 1.6 for backward compatibility

### Payload Encoding

The `payload` field contains JSON-encoded data specific to each feature. For simple string parameters (like configuration key names), the value can be passed directly as a string.

**Example - Simple payload:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 0,
  "feature_name": "GetConfiguration",
  "payload": "HeartbeatInterval"
}
```

**Example - Complex payload:**
```json
{
  "charge_point_id": "CP001",
  "connector_id": 1,
  "feature_name": "RemoteStartTransaction",
  "payload": "{\"idTag\":\"USER001\",\"connectorId\":1}"
}
```

## Response Format

### Success Response

On successful command execution, the API returns the OCPP response from the charge point:

**HTTP Status:** `200 OK`

```json
{
  "configurationKey": [
    {
      "key": "HeartbeatInterval",
      "readonly": false,
      "value": "300"
    }
  ],
  "unknownKey": []
}
```

### Success with No Content

Some commands complete successfully but return no data:

**HTTP Status:** `204 No Content`

### Error Response

**HTTP Status:** `4xx` or `5xx`

```json
{
  "status": "error",
  "error": "error description"
}
```

## Error Handling

### HTTP Status Codes

| Code | Description |
|------|-------------|
| 200 | Success - Response contains charge point data |
| 204 | Success - Command executed, no response data |
| 400 | Bad Request - Invalid JSON or missing required fields |
| 404 | Not Found - Invalid endpoint path |
| 405 | Method Not Allowed - Only POST is accepted |
| 500 | Internal Server Error - Processing error |

### Common Error Messages

| Error | Cause |
|-------|-------|
| `charge point not found` | No charge point with specified ID is connected |
| `timeout waiting for response` | Charge point did not respond within 10 seconds |
| `invalid feature name` | Unknown or unsupported OCPP feature |
| `invalid payload` | Payload does not match expected format |

### Timeout Behavior

The API uses synchronous request/response with a **10-second timeout**. If the charge point does not respond within this window, an error is returned. The original OCPP request may still be processed by the charge point.

## Protocol Versions

EVSYS supports multiple OCPP protocol versions:

| Version | Status | Notes |
|---------|--------|-------|
| OCPP 1.6J | Full Support | Default protocol, JSON over WebSocket |
| OCPP 2.0.1 | Partial Support | Core features implemented |
| OCPP 2.1 | Planned | Future support |

### Version-Specific Documentation

- [OCPP 1.6 Features](API_OCPP16.md) - Complete reference for OCPP 1.6J protocol
- [OCPP 2.0.1 Features](API_OCPP201.md) - Reference for OCPP 2.0.1 protocol

## Feature Reference

### Quick Reference - OCPP 1.6

| Feature Name | Direction | Description |
|--------------|-----------|-------------|
| `GetConfiguration` | CS -> CP | Retrieve charge point configuration |
| `ChangeConfiguration` | CS -> CP | Modify charge point configuration |
| `RemoteStartTransaction` | CS -> CP | Start charging session remotely |
| `RemoteStopTransaction` | CS -> CP | Stop charging session remotely |
| `Reset` | CS -> CP | Reset charge point (Soft/Hard) |
| `SetChargingProfile` | CS -> CP | Set charging power limits |
| `ClearChargingProfile` | CS -> CP | Remove charging profiles |
| `GetCompositeSchedule` | CS -> CP | Get calculated charging schedule |
| `TriggerMessage` | CS -> CP | Request charge point to send message |
| `GetDiagnostics` | CS -> CP | Request diagnostics upload |
| `SendLocalList` | CS -> CP | Update local authorization list |
| `UnlockConnector` | CS -> CP | Unlock charging connector |
| `GetServerStatus` | Server | List connected charge points (non-OCPP) |

### Quick Reference - OCPP 2.0.1

| Feature Name | Direction | Description |
|--------------|-----------|-------------|
| `GetVariables` | CS -> CP | Retrieve device model variables |
| `SetVariables` | CS -> CP | Modify device model variables |
| `RequestStartTransaction` | CS -> CP | Start charging session remotely |
| `RequestStopTransaction` | CS -> CP | Stop charging session remotely |
| `Reset` | CS -> CP | Reset charging station |

**Legend:**
- CS = Central System (EVSYS)
- CP = Charge Point (Charging Station)

## Server Status Command

The `GetServerStatus` command is a non-OCPP feature specific to EVSYS:

**Request:**
```json
{
  "charge_point_id": "",
  "connector_id": 0,
  "feature_name": "GetServerStatus",
  "payload": ""
}
```

**Response:**
```json
{
  "connected_clients": "CP001,CP002,CP003",
  "total_clients": 3
}
```

This command does not require a `charge_point_id` and returns information about all connected charge points.
