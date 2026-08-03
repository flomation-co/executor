package linear_common

import "testing"

func TestLooksLikeUUID(t *testing.T) {
	cases := map[string]bool{
		"3e5e5e5e-1111-2222-3333-444455556666": true,
		"Enriched":                             false,
		"FLO-123":                              false,
		"":                                     false,
		"not-a-uuid":                           false,
	}
	for in, want := range cases {
		if got := LooksLikeUUID(in); got != want {
			t.Errorf("LooksLikeUUID(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSplitCSV(t *testing.T) {
	got := SplitCSV(" Enriched , Priority ,, Hot ")
	want := []string{"Enriched", "Priority", "Hot"}
	if len(got) != len(want) {
		t.Fatalf("SplitCSV len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("SplitCSV[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if len(SplitCSV("   ")) != 0 {
		t.Error("SplitCSV of blanks should be empty")
	}
}

// When every value is already a UUID, ResolveLabelIDs must NOT hit the network
// — it passes them through unchanged. This is the offline-testable contract; the
// name-lookup path is exercised by integration tests against a live key.
func TestResolveLabelIDs_UUIDPassthrough(t *testing.T) {
	uuids := []string{"3e5e5e5e-1111-2222-3333-444455556666", "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}
	ids, unresolved, err := ResolveLabelIDs("unused-key", uuids)
	if err != nil {
		t.Fatalf("passthrough must not error (no network call): %v", err)
	}
	if len(unresolved) != 0 {
		t.Errorf("UUIDs should never be unresolved, got %v", unresolved)
	}
	if len(ids) != 2 || ids[0] != uuids[0] || ids[1] != uuids[1] {
		t.Errorf("passthrough ids = %v, want %v", ids, uuids)
	}
}

func TestResolveUserID_UUIDPassthrough(t *testing.T) {
	u := "3e5e5e5e-1111-2222-3333-444455556666"
	got, err := ResolveUserID("unused-key", u)
	if err != nil || got != u {
		t.Fatalf("ResolveUserID passthrough = %q, %v; want %q", got, err, u)
	}
	if _, err := ResolveUserID("unused-key", ""); err == nil {
		t.Error("empty user should error")
	}
}
