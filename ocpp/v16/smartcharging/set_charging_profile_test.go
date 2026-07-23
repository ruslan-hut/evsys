package smartcharging

import "testing"

// OCPP 1.6 scopes chargingProfileId to the charge point, not to the connector.
// Two connectors on the same charge point sharing an id lets the second
// SetChargingProfile replace the profile already installed for the first, so the
// first session silently loses its limit.
func TestTransactionProfileIdIsUniquePerConnector(t *testing.T) {
	first := NewTransactionChargingProfile(1, 100, 330)
	second := NewTransactionChargingProfile(2, 101, 195)

	if first.ChargingProfileId == second.ChargingProfileId {
		t.Fatalf("connectors 1 and 2 share chargingProfileId %d", first.ChargingProfileId)
	}

	// and it must not collide with the default profile installed at boot
	def := NewDefaultChargingProfile(85)
	for _, profile := range []struct {
		name string
		id   int
	}{{"connector 1", first.ChargingProfileId}, {"connector 2", second.ChargingProfileId}} {
		if profile.id == def.ChargingProfileId {
			t.Fatalf("%s collides with the default profile id %d", profile.name, def.ChargingProfileId)
		}
	}
}

// The same connector must keep the same id across updates, otherwise raising a
// session's limit installs a second profile instead of replacing the first.
func TestTransactionProfileIdIsStablePerConnector(t *testing.T) {
	before := NewTransactionChargingProfile(2, 100, 115)
	after := NewTransactionChargingProfile(2, 100, 330)

	if before.ChargingProfileId != after.ChargingProfileId {
		t.Fatalf("connector 2 changed chargingProfileId from %d to %d",
			before.ChargingProfileId, after.ChargingProfileId)
	}
	if after.ChargingSchedule.ChargingSchedulePeriod[0].Limit != 330 {
		t.Fatalf("limit = %v, want 330", after.ChargingSchedule.ChargingSchedulePeriod[0].Limit)
	}
}
