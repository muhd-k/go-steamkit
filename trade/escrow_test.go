package trade

import "testing"

func TestParseEscrowDuration(t *testing.T) {
	duration, err := parseEscrowDuration([]byte(`
		<script>
			var g_daysMyEscrow = 0;
			var g_daysTheirEscrow = 7;
		</script>
	`))
	if err != nil {
		t.Fatalf("parseEscrowDuration() error: %v", err)
	}
	if duration.DaysMyEscrow != 0 || duration.DaysTheirEscrow != 7 {
		t.Fatalf("duration = %#v", duration)
	}
}

func TestParseEscrowDuration_NotFriends(t *testing.T) {
	_, err := parseEscrowDuration([]byte(`>You are not friends with this user<`))
	if err == nil {
		t.Fatal("expected error")
	}
}
