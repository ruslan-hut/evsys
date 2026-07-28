package internal

import (
	"evsys/entity"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

// Integration tests for the webhook outbox. They need a real MongoDB: the ordering
// guarantee lives in the head-of-queue query, and the migration creates indexes a
// mock would not enforce. Same harness as the sweep tests (see sweep_test.go).

func seedDelivery(t *testing.T, db *MongoDB, delivery entity.WebhookDelivery) {
	t.Helper()
	withCollection(t, db, collectionWebhookOutbox, func(c *mongo.Collection) {
		if _, err := c.InsertOne(db.ctx, delivery); err != nil {
			t.Fatalf("seed delivery: %v", err)
		}
	})
}

func seedSubscriber(t *testing.T, db *MongoDB, subscriber entity.WebhookSubscriber) {
	t.Helper()
	withCollection(t, db, collectionWebhookSubscribers, func(c *mongo.Collection) {
		if _, err := c.InsertOne(db.ctx, subscriber); err != nil {
			t.Fatalf("seed subscriber: %v", err)
		}
	})
}

func pendingDelivery(t *testing.T, subscriber string, sequence int64, nextAttempt time.Time) entity.WebhookDelivery {
	t.Helper()
	return entity.WebhookDelivery{
		EventId:     "event-" + subscriber + "-" + time.Now().Format("150405.000000000") + string(rune('a'+sequence%26)),
		Subscriber:  subscriber,
		Type:        "transaction.start",
		Sequence:    sequence,
		Payload:     []byte(`{"id":"x"}`),
		Status:      entity.WebhookPending,
		NextAttempt: nextAttempt,
		CreatedAt:   time.Now().UTC(),
	}
}

func TestGetWebhookSubscribersReturnsOnlyEnabled(t *testing.T) {
	db := testClient(t)
	seedSubscriber(t, db, entity.WebhookSubscriber{Name: "on", URL: "http://a", Events: []string{"*"}, IsEnabled: true})
	seedSubscriber(t, db, entity.WebhookSubscriber{Name: "off", URL: "http://b", Events: []string{"*"}, IsEnabled: false})

	subscribers, err := db.GetWebhookSubscribers()
	if err != nil {
		t.Fatalf("GetWebhookSubscribers: %v", err)
	}
	if len(subscribers) != 1 || subscribers[0].Name != "on" {
		t.Fatalf("expected only the enabled subscriber, got %+v", subscribers)
	}
}

// TestGetPendingWebhookDeliveryOrdering pins the two properties the dispatcher relies
// on: the oldest pending delivery comes first regardless of insertion order, and a
// head that is not yet due blocks the queue even when newer deliveries are due.
func TestGetPendingWebhookDeliveryOrdering(t *testing.T) {
	db := testClient(t)
	now := time.Now().UTC()

	// inserted newest-first, so a query without the sort would return sequence 3
	seedDelivery(t, db, pendingDelivery(t, "sub", 3, now.Add(-time.Minute)))
	seedDelivery(t, db, pendingDelivery(t, "sub", 1, now.Add(-time.Minute)))
	seedDelivery(t, db, pendingDelivery(t, "sub", 2, now.Add(-time.Minute)))

	delivery, err := db.GetPendingWebhookDelivery("sub", now)
	if err != nil {
		t.Fatalf("GetPendingWebhookDelivery: %v", err)
	}
	if delivery == nil || delivery.Sequence != 1 {
		t.Fatalf("expected head of queue sequence 1, got %+v", delivery)
	}
}

func TestGetPendingWebhookDeliveryHeadNotDueBlocksQueue(t *testing.T) {
	db := testClient(t)
	now := time.Now().UTC()

	// head is scheduled for a future retry; the newer delivery is due now
	seedDelivery(t, db, pendingDelivery(t, "sub", 1, now.Add(time.Hour)))
	seedDelivery(t, db, pendingDelivery(t, "sub", 2, now.Add(-time.Minute)))

	delivery, err := db.GetPendingWebhookDelivery("sub", now)
	if err != nil {
		t.Fatalf("GetPendingWebhookDelivery: %v", err)
	}
	if delivery != nil {
		t.Fatalf("expected nothing due while the head waits for its retry, got sequence %d", delivery.Sequence)
	}
}

func TestGetPendingWebhookDeliverySkipsOtherSubscribersAndStatuses(t *testing.T) {
	db := testClient(t)
	now := time.Now().UTC()

	delivered := pendingDelivery(t, "sub", 1, now.Add(-time.Minute))
	delivered.Status = entity.WebhookDelivered
	seedDelivery(t, db, delivered)
	failed := pendingDelivery(t, "sub", 2, now.Add(-time.Minute))
	failed.Status = entity.WebhookFailed
	seedDelivery(t, db, failed)
	seedDelivery(t, db, pendingDelivery(t, "other", 3, now.Add(-time.Minute)))
	seedDelivery(t, db, pendingDelivery(t, "sub", 4, now.Add(-time.Minute)))

	delivery, err := db.GetPendingWebhookDelivery("sub", now)
	if err != nil {
		t.Fatalf("GetPendingWebhookDelivery: %v", err)
	}
	if delivery == nil || delivery.Sequence != 4 {
		t.Fatalf("expected the pending delivery of the right subscriber (sequence 4), got %+v", delivery)
	}
}

func TestMarkWebhookDeliveredAndFailedTransitions(t *testing.T) {
	db := testClient(t)
	now := time.Now().UTC()

	first := pendingDelivery(t, "sub", 1, now)
	second := pendingDelivery(t, "sub", 2, now)
	seedDelivery(t, db, first)
	seedDelivery(t, db, second)

	if err := db.MarkWebhookDelivered(first.EventId, "sub"); err != nil {
		t.Fatalf("MarkWebhookDelivered: %v", err)
	}
	retryAt := now.Add(30 * time.Second)
	if err := db.MarkWebhookFailed(second.EventId, "sub", "connection refused", retryAt, false); err != nil {
		t.Fatalf("MarkWebhookFailed: %v", err)
	}

	delivery, err := db.GetPendingWebhookDelivery("sub", now)
	if err != nil {
		t.Fatalf("GetPendingWebhookDelivery: %v", err)
	}
	if delivery != nil {
		t.Fatalf("expected no delivery due (one delivered, one waiting for retry), got %+v", delivery)
	}

	delivery, err = db.GetPendingWebhookDelivery("sub", retryAt.Add(time.Second))
	if err != nil {
		t.Fatalf("GetPendingWebhookDelivery after retry window: %v", err)
	}
	if delivery == nil || delivery.Sequence != 2 {
		t.Fatalf("expected the failed delivery due again after nextAttempt, got %+v", delivery)
	}
	if delivery.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", delivery.Attempts)
	}
	if delivery.LastError != "connection refused" {
		t.Errorf("last_error = %q, want the recorded error", delivery.LastError)
	}

	// a second failure marked terminal removes it from the queue for good
	if err = db.MarkWebhookFailed(second.EventId, "sub", "still down", retryAt, true); err != nil {
		t.Fatalf("MarkWebhookFailed terminal: %v", err)
	}
	delivery, err = db.GetPendingWebhookDelivery("sub", retryAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetPendingWebhookDelivery after terminal failure: %v", err)
	}
	if delivery != nil {
		t.Fatalf("terminally failed delivery is still served: %+v", delivery)
	}
}

func TestGetMaxWebhookSequence(t *testing.T) {
	db := testClient(t)

	sequence, err := db.GetMaxWebhookSequence()
	if err != nil {
		t.Fatalf("GetMaxWebhookSequence on empty outbox: %v", err)
	}
	if sequence != 0 {
		t.Fatalf("empty outbox sequence = %d, want 0", sequence)
	}

	now := time.Now().UTC()
	seedDelivery(t, db, pendingDelivery(t, "a", 7, now))
	delivered := pendingDelivery(t, "b", 12, now)
	delivered.Status = entity.WebhookDelivered
	seedDelivery(t, db, delivered)

	sequence, err = db.GetMaxWebhookSequence()
	if err != nil {
		t.Fatalf("GetMaxWebhookSequence: %v", err)
	}
	if sequence != 12 {
		t.Fatalf("sequence = %d, want 12 (max across all statuses)", sequence)
	}
}

func TestMigrationWebhooks(t *testing.T) {
	db := testClient(t)

	connection, err := db.connect()
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.disconnect(connection)
	database := connection.Database(db.database)

	if err = migrationWebhooksUp(db.ctx, database); err != nil {
		t.Fatalf("migrationWebhooksUp: %v", err)
	}

	// the unique index must reject a duplicate subscriber name
	subscribers := database.Collection(collectionWebhookSubscribers)
	if _, err = subscribers.InsertOne(db.ctx, entity.WebhookSubscriber{Name: "dup", URL: "http://a"}); err != nil {
		t.Fatalf("insert first subscriber: %v", err)
	}
	if _, err = subscribers.InsertOne(db.ctx, entity.WebhookSubscriber{Name: "dup", URL: "http://b"}); err == nil {
		t.Fatal("duplicate subscriber name was accepted; unique index missing")
	}

	indexNames := func(collection string) map[string]bool {
		cursor, e := database.Collection(collection).Indexes().List(db.ctx)
		if e != nil {
			t.Fatalf("list indexes of %s: %v", collection, e)
		}
		var specs []struct {
			Name string `bson:"name"`
		}
		if e = cursor.All(db.ctx, &specs); e != nil {
			t.Fatalf("decode indexes of %s: %v", collection, e)
		}
		names := map[string]bool{}
		for _, spec := range specs {
			names[spec.Name] = true
		}
		return names
	}

	outboxIndexes := indexNames(collectionWebhookOutbox)
	if !outboxIndexes["subscriber_status_sequence"] {
		t.Error("outbox dispatch index missing")
	}
	if !outboxIndexes["delivered_at_ttl"] {
		t.Error("outbox ttl index missing")
	}

	if err = migrationWebhooksDown(db.ctx, database); err != nil {
		t.Fatalf("migrationWebhooksDown: %v", err)
	}
	outboxIndexes = indexNames(collectionWebhookOutbox)
	if outboxIndexes["subscriber_status_sequence"] || outboxIndexes["delivered_at_ttl"] {
		t.Error("rollback left outbox indexes in place")
	}
	if indexNames(collectionWebhookSubscribers)["name_unique"] {
		t.Error("rollback left the subscriber name index in place")
	}
}
