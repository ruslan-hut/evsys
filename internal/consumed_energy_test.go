package internal

import (
	"testing"
	"time"

	"evsys/entity"

	"go.mongodb.org/mongo-driver/mongo"
)

func seedChargePoint(t *testing.T, db *MongoDB, id, locationId string) {
	t.Helper()
	withCollection(t, db, collectionChargePoints, func(c *mongo.Collection) {
		if _, err := c.InsertOne(db.ctx, &entity.ChargePoint{Id: id, LocationId: locationId}); err != nil {
			t.Fatalf("seed charge point: %v", err)
		}
	})
}

func TestGetTodayConsumedEnergy(t *testing.T) {
	db := testClient(t)
	now := time.Now().UTC()

	seedChargePoint(t, db, "CP1", "loc1")
	seedChargePoint(t, db, "CP2", "loc2")

	// two sessions on CP1 today: 1000 + 500
	seedTransaction(t, db, &entity.Transaction{
		Id: 1, ChargePointId: "CP1", IsFinished: true,
		MeterStart: 0, MeterStop: 1000, TimeStop: now.Add(-time.Hour),
	})
	seedTransaction(t, db, &entity.Transaction{
		Id: 2, ChargePointId: "CP1", IsFinished: true,
		MeterStart: 200, MeterStop: 700, TimeStop: now.Add(-2 * time.Hour),
	})
	// yesterday's session is outside the window
	seedTransaction(t, db, &entity.Transaction{
		Id: 3, ChargePointId: "CP1", IsFinished: true,
		MeterStart: 0, MeterStop: 9999, TimeStop: now.Add(-48 * time.Hour),
	})
	// an open session has no final reading yet
	seedTransaction(t, db, &entity.Transaction{
		Id: 4, ChargePointId: "CP2", IsFinished: false,
		MeterStart: 0, MeterStop: 0, TimeStop: now.Add(-time.Hour),
	})
	// a meter that went backwards contributes zero, not a negative value
	seedTransaction(t, db, &entity.Transaction{
		Id: 5, ChargePointId: "CP2", IsFinished: true,
		MeterStart: 5000, MeterStop: 100, TimeStop: now.Add(-time.Hour),
	})
	// a charge point with no document lands in the empty-location group
	seedTransaction(t, db, &entity.Transaction{
		Id: 6, ChargePointId: "CPX", IsFinished: true,
		MeterStart: 0, MeterStop: 300, TimeStop: now.Add(-time.Hour),
	})

	got, err := db.GetTodayConsumedEnergy()
	if err != nil {
		t.Fatalf("GetTodayConsumedEnergy: %v", err)
	}

	byChargePoint := map[string]*entity.ConsumedEnergy{}
	for _, c := range got {
		byChargePoint[c.ID.ChargePointID] = c
	}

	cp1, ok := byChargePoint["CP1"]
	if !ok {
		t.Fatal("CP1 missing from the aggregation")
	}
	if cp1.ID.Location != "loc1" || cp1.Consumed != 1500 {
		t.Errorf("CP1 = location %q consumed %d, want loc1 / 1500", cp1.ID.Location, cp1.Consumed)
	}
	if cp1.Count != 2 {
		t.Errorf("CP1 count = %d, want 2: the count must cover the same rows as the energy sum", cp1.Count)
	}

	cp2, ok := byChargePoint["CP2"]
	if !ok {
		t.Fatal("CP2 missing from the aggregation")
	}
	if cp2.Consumed != 0 {
		t.Errorf("a backwards meter should contribute 0, got %d", cp2.Consumed)
	}
	if cp2.Count != 1 {
		t.Errorf("CP2 count = %d, want 1: a finished session counts even when its meter went backwards", cp2.Count)
	}

	cpx, ok := byChargePoint["CPX"]
	if !ok {
		t.Fatal("CPX missing from the aggregation")
	}
	if cpx.ID.Location != "" {
		t.Errorf("an unknown charge point should group under an empty location, got %q", cpx.ID.Location)
	}
}
