package autonomousdatabase

import (
	"strings"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/database"

	core "flomation.app/automate/executor"
)

func sc(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: val}
}

// TestValidRegion guards the host-selecting region field: a real OCI region is a
// plain label, and anything that could redirect the SDK's host construction is
// rejected.
func TestValidRegion(t *testing.T) {
	for _, ok := range []string{"uk-london-1", "us-ashburn-1", "eu-frankfurt-1"} {
		if !validRegion.MatchString(ok) {
			t.Errorf("validRegion rejected a real region %q", ok)
		}
	}
	for _, bad := range []string{"attacker.com", "uk-london-1.evil.com", "host:1337", "a/b", "UK-LONDON-1", ""} {
		if validRegion.MatchString(bad) {
			t.Errorf("validRegion ACCEPTED a host-injection region %q", bad)
		}
	}
}

// TestGetAuthRejectsBadRegion confirms the SSRF guard fires in GetAuth before the
// region reaches the SDK's host builder.
func TestGetAuthRejectsBadRegion(t *testing.T) {
	in := []*core.Connection{
		sc("tenancy_ocid", "ocid1.tenancy.oc1..aaaa"),
		sc("user_ocid", "ocid1.user.oc1..aaaa"),
		sc("region", "attacker.com"),
		sc("fingerprint", "aa:bb:cc"),
		sc("private_key", "-----BEGIN RSA PRIVATE KEY-----\nx\n-----END RSA PRIVATE KEY-----"),
	}
	_, err := GetAuth(in)
	if err == nil || !strings.Contains(err.Error(), "region") {
		t.Fatalf("GetAuth should reject region \"attacker.com\", got %v", err)
	}
}

// TestGetAuthRedactsMalformedKey: a key that passes the non-empty check but fails
// to parse must not leak into the error OR the soft-failure envelope's tool_result.
func TestGetAuthRedactsMalformedKey(t *testing.T) {
	const secretBody = "SUPERSECRETKEYBODY0123456789abcdef"
	in := []*core.Connection{
		sc("tenancy_ocid", "ocid1.tenancy.oc1..aaaa"),
		sc("user_ocid", "ocid1.user.oc1..aaaa"),
		sc("region", "uk-london-1"),
		sc("fingerprint", "aa:bb:cc"),
		sc("private_key", "-----BEGIN RSA PRIVATE KEY-----\n"+secretBody+"\n-----END RSA PRIVATE KEY-----"),
	}
	_, err := GetAuth(in)
	if err == nil {
		t.Fatal("GetAuth accepted a malformed private key")
	}
	if strings.Contains(err.Error(), secretBody) {
		t.Errorf("GetAuth leaked key material: %q", err)
	}
	res := ErrorResult(err.Error())
	for _, k := range []string{"error", "tool_result"} {
		if s, _ := res[k].(string); strings.Contains(s, secretBody) {
			t.Errorf("ErrorResult[%q] leaked key material", k)
		}
	}
}

func TestServiceErrorCode(t *testing.T) {
	if code, status := ServiceErrorCode(nil); code != "" || status != 0 {
		t.Errorf("ServiceErrorCode(nil) = (%q,%d), want (\"\",0)", code, status)
	}
}

// TestOptionalFloat32 covers the compute-count parsing used by create/scale.
func TestOptionalFloat32(t *testing.T) {
	if _, ok, err := OptionalFloat32("cpu_count", []*core.Connection{sc("cpu_count", "")}); ok || err != nil {
		t.Errorf("blank should be (_,false,nil)")
	}
	if v, ok, err := OptionalFloat32("cpu_count", []*core.Connection{sc("cpu_count", "2")}); !ok || err != nil || v != 2 {
		t.Errorf("\"2\" should parse to 2, got (%v,%v,%v)", v, ok, err)
	}
	if _, _, err := OptionalFloat32("cpu_count", []*core.Connection{sc("cpu_count", "x")}); err == nil {
		t.Error("non-numeric should error")
	}
}

// TestBackupSummarisersAgree is the review fix: the Get/Create (full) and List
// (summary) backup summarisers MUST emit the same keys with the same presence
// rules, so the "backup" output shape is identical whichever action produced it.
func TestBackupSummarisersAgree(t *testing.T) {
	full := SummariseBackup(&database.AutonomousDatabaseBackup{})
	sum := SummariseBackupSummary(&database.AutonomousDatabaseBackupSummary{})
	if len(full) != len(sum) {
		t.Fatalf("key counts differ: full=%d summary=%d", len(full), len(sum))
	}
	for k := range full {
		if _, ok := sum[k]; !ok {
			t.Errorf("key %q present in SummariseBackup but not SummariseBackupSummary", k)
		}
	}
	// is_long_term_backup must be present in both (the finding's core gap).
	if _, ok := full["is_long_term_backup"]; !ok {
		t.Error("is_long_term_backup missing from SummariseBackup")
	}
	// And it must reflect the LONGTERM type.
	lt := SummariseBackup(&database.AutonomousDatabaseBackup{Type: database.AutonomousDatabaseBackupTypeLongterm})
	if lt["is_long_term_backup"] != true {
		t.Errorf("LONGTERM backup should have is_long_term_backup=true, got %v", lt["is_long_term_backup"])
	}
}
