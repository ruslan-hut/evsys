# EVSYS Event Webhooks

EVSYS can push system events (charging sessions, connector status, alerts) to
external systems over HTTP. This document is the complete protocol description
for teams implementing a receiving side.

Delivery is **at-least-once**, **ordered per subscriber**, and survives evsys
restarts: every event is persisted to an outbox before delivery and retried
until acknowledged or expired.

## Registration

Subscribers are configured in the `webhook_subscribers` MongoDB collection (a
management UI is planned; until then documents are inserted directly):

```json
{
  "name": "nomadus",
  "url": "https://example.com/webhooks/evsys",
  "token": "shared-secret",
  "events": ["transaction.*", "status"],
  "is_enabled": true,
  "updated_at": {"$date": "2026-07-30T00:00:00Z"}
}
```

| Field | Description |
|---|---|
| `name` | Unique subscriber id; used as the delivery queue key |
| `url` | Endpoint that receives `POST` requests |
| `token` | Sent back verbatim as `Authorization: Token <token>` |
| `events` | Event type filter, see below |
| `is_enabled` | Disabled subscribers receive nothing and accumulate nothing |

An `events` entry is an exact type (`transaction.start`), a prefix wildcard
(`transaction.*`), or the catch-all `*`. Configuration changes are picked up
within one minute; no restart needed. The subsystem itself is gated by
`webhooks.enabled` in the evsys config and requires MongoDB.

## Transport

Each event is one HTTP request:

```
POST <url>
Content-Type: application/json
Authorization: Token <token>
```

- The request body is the envelope described below.
- The request times out after **15 seconds**.
- **Any 2xx status acknowledges the event.** The response body is ignored.
- Any other status, a timeout, or a connection error counts as a failed
  attempt and the event will be redelivered.

Consequence for receivers: return 2xx only after the event is durably accepted
(persisted or safely processed). Returning 2xx and then crashing loses the
event; returning 5xx keeps it queued.

## Delivery semantics

**At-least-once.** An event can arrive more than once (for example when the
receiver processed it but the acknowledgement was lost). Receivers must
deduplicate by envelope `id`.

**Ordered per subscriber.** Events are delivered strictly in `sequence` order.
While the oldest undelivered event is failing, newer events wait — a
`transaction.stop` is never delivered before its `transaction.start`. Ordering
holds only per subscriber; there is no cross-subscriber coordination.

**Retry schedule.** After a failed attempt the delivery is retried after
30 s, 1 min, 5 min, 15 min, 1 h, then hourly. After **24 hours** of failures
the event is abandoned (marked `failed` in the outbox, logged on the evsys
side) and the queue moves on to the next event. An abandoned event is *not*
redelivered — after an outage longer than 24 h the receiver must reconcile
state by other means (e.g. re-read via API or database).

**Blocking is per subscriber.** One unreachable receiver delays only its own
queue; other subscribers are unaffected.

## Envelope

```json
{
  "id": "0d9df9a4-63f2-49f5-8d4e-6f8f0a2a9b7e",
  "type": "transaction.start",
  "source": "evsys",
  "time": "2026-07-30T10:15:03.412Z",
  "sequence": 1287,
  "data": { ... }
}
```

| Field | Type | Description |
|---|---|---|
| `id` | string (UUID v4) | Unique per event. **Deduplication key.** All subscribers receiving the same event see the same `id`. |
| `type` | string | Event type, dotted (see next section) |
| `source` | string | Always `"evsys"` |
| `time` | string (RFC 3339) | When evsys emitted the event |
| `sequence` | int64 | Monotonically increasing across all events and restarts. Gaps are normal (other subscribers' events, filtered types). Use for last-writer-wins on state updates: ignore an update whose `sequence` is lower than the last one applied for the same entity. |
| `data` | object | Event payload, see below |

The envelope for a given `id` is immutable: retries and different subscribers
always receive byte-identical payloads.

## Event types

| Type | Emitted when |
|---|---|
| `transaction.start` | A charging session started (OCPP StartTransaction accepted) |
| `transaction.stop` | A charging session finished (normal stop, or closed by the abandoned-session sweeper) |
| `status` | A connector status changed (StatusNotification), including synthetic per-connector notifications when a charge point comes online/offline |
| `authorize` | An authorization attempt was processed (RFID tag / remote start) |
| `alert` | An operational problem: busy connector on start, billing failure, late or duplicate stop, charge point online/offline transition, session closed by the sweeper |
| `info` | Informational system messages (startup summary etc.) |
| `transaction.event` | Reserved, not currently emitted |

## `data` payload

`data` is the same structure for every event type; fields not relevant to the
event are **omitted** (JSON `omitempty` — this also means zero values are
omitted: a `consumed` of `0` or a `connector_id` of `0` is absent, not `0`).

| Field | Type | Description |
|---|---|---|
| `type` | string | Internal event name (`TransactionStart`, `Alert`, ...); redundant with the envelope `type` |
| `charge_point_id` | string | OCPP charge point identity |
| `connector_id` | int | Connector number within the charge point (1-based; 0/absent when not connector-specific) |
| `location_id` | string | Location (site) id |
| `evse` | string | EVSE id used for OCPI roaming |
| `time` | string (RFC 3339) | Event-specific time, see notes below |
| `username` | string | Resolved user name, when known |
| `id_tag` | string | Authorization tag; for `authorize` events prefixed with the source, e.g. `"remote ABC123"` |
| `transaction_id` | int | Session id; correlates `transaction.start` and `transaction.stop` |
| `consumed` | int | Energy consumed, **Wh** (present on `transaction.stop`) |
| `status` | string | Connector status (`Available`, `Charging`, `Faulted`, ...) or authorization result |
| `info` | string | Human-readable note (e.g. `"consumed 12.4 kW; 3.10 €"` on stop) |
| `payload` | object | Raw OCPP request that triggered the event. Diagnostic only — structure depends on charger protocol version and may change; do not build logic on it |

Field presence by type (guaranteed unless noted):

- **`transaction.start`** — `charge_point_id`, `connector_id`, `location_id`,
  `evse`, `time` (= session start), `username`, `id_tag`, `transaction_id`,
  `status`.
- **`transaction.stop`** — same identifiers plus `consumed` (Wh) and a
  price/energy summary in `info`. Note: `time` currently carries the session
  *start* time; use the envelope `time` for the stop moment.
- **`status`** — `charge_point_id`, `connector_id`, `location_id`, `evse`,
  `status`; `transaction_id` when a session is active on the connector.
  Synthetic online/offline notifications carry only `location_id`, `evse`,
  `status`.
- **`authorize`** — `charge_point_id`, `id_tag`, `status`, `username`/`info`
  when known.
- **`alert` / `info`** — always `info`; other fields as applicable.

Charge points speaking OCPP 2.0.1 currently produce sparser payloads:
`transaction.start`/`transaction.stop` may lack `location_id`, `evse`,
`status` and `consumed`. Treat every field except the ones needed for
correlation (`transaction_id`, `charge_point_id`) as optional.

## Example

`transaction.stop` as received:

```json
{
  "id": "8b1f3c9e-2a44-4a6e-9d3f-5e7c1b0a9f21",
  "type": "transaction.stop",
  "source": "evsys",
  "time": "2026-07-30T12:41:07.902Z",
  "sequence": 1302,
  "data": {
    "type": "TransactionStop",
    "charge_point_id": "Wallbox3",
    "connector_id": 1,
    "location_id": "loc-madrid-01",
    "evse": "ES*EVS*E003*1",
    "time": "2026-07-30T11:02:44Z",
    "username": "j.garcia",
    "id_tag": "04A2B3C4D5",
    "transaction_id": 4711,
    "consumed": 12400,
    "status": "Charging",
    "info": "consumed 12.4 kW; 3.10 €",
    "payload": { "transactionId": 4711, "meterStop": 1012400, "...": "..." }
  }
}
```

## Receiver implementation checklist

1. Expose an HTTPS `POST` endpoint; verify `Authorization: Token <token>`
   with a constant-time comparison.
2. Parse the envelope; ignore unknown fields and unknown `type` values
   (new types may be added — subscribe narrowly via the `events` filter).
3. Deduplicate by `id` (persist processed ids; a TTL of a few days is enough
   given the 24 h redelivery window).
4. For state updates keyed by an entity (e.g. connector status), keep the
   highest applied `sequence` and drop older ones.
5. Acknowledge with 2xx only after the event is durably accepted; respond
   within 15 s (do heavy processing asynchronously).
6. Return 5xx on internal failure to get a redelivery; never return 2xx for
   an event you could not store.
7. Handle the >24 h outage case: after downtime, reconcile state instead of
   relying on replay.
