package compute

import (
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

func sc(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: val}
}

// TestValidRegion guards the host-selecting region field: a legitimate OCI region
// is a plain label, and anything that could redirect the SDK's host construction
// (dots, slashes, colons, whitespace, uppercase-with-dot) is rejected.
func TestValidRegion(t *testing.T) {
	for _, ok := range []string{"uk-london-1", "us-ashburn-1", "eu-frankfurt-1", "ap-tokyo-1"} {
		if !validRegion.MatchString(ok) {
			t.Errorf("validRegion rejected a real region %q", ok)
		}
	}
	for _, bad := range []string{"attacker.com", "iaas.attacker.com", "uk-london-1.evil.com", "host:1337", "a/b", "uk london 1", "UK-LONDON-1", ""} {
		if validRegion.MatchString(bad) {
			t.Errorf("validRegion ACCEPTED a host-injection region %q", bad)
		}
	}
}

// TestGetAuthRejectsBadRegion confirms the region guard fires in GetAuth (before
// the region ever reaches the SDK's host builder). A dotted region is the
// concrete SSRF vector.
func TestGetAuthRejectsBadRegion(t *testing.T) {
	in := []*core.Connection{
		sc("tenancy_ocid", "ocid1.tenancy.oc1..aaaa"),
		sc("user_ocid", "ocid1.user.oc1..aaaa"),
		sc("region", "attacker.com"),
		sc("fingerprint", "aa:bb:cc"),
		sc("private_key", "-----BEGIN RSA PRIVATE KEY-----\nx\n-----END RSA PRIVATE KEY-----"),
	}
	_, err := GetAuth(in)
	if err == nil {
		t.Fatal("GetAuth accepted region \"attacker.com\" — the SSRF guard did not fire")
	}
	if !strings.Contains(err.Error(), "region") {
		t.Errorf("GetAuth error = %q, want it to name the region", err)
	}
	// The region is validated before the private key, so no key material can be in
	// the message.
	if strings.Contains(err.Error(), "BEGIN RSA") {
		t.Error("GetAuth region error leaked key material")
	}
}

// TestGetAuthRedactsMalformedKey is the malformed-key echo path Dan flagged: a
// key that passes the non-empty check but fails to parse. The resulting error
// flows into ErrorResult's error AND tool_result, so neither may contain key
// material.
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
		t.Errorf("GetAuth leaked key material in the error: %q", err)
	}
	// The soft-failure envelope built from that error must also be clean on BOTH
	// the error and tool_result fields.
	res := ErrorResult(err.Error())
	for _, k := range []string{"error", "tool_result"} {
		if s, _ := res[k].(string); strings.Contains(s, secretBody) {
			t.Errorf("ErrorResult[%q] leaked key material", k)
		}
	}
}

// TestFieldLabel: *_ocid fields render with an upper-cased OCID token, matching
// RequiredCompartment's wording.
func TestFieldLabel(t *testing.T) {
	for in, want := range map[string]string{
		"instance_ocid": "instance OCID",
		"vcn_ocid":      "vcn OCID",
		"subnet_ocid":   "subnet OCID",
		"display_name":  "display name",
	} {
		if got := fieldLabel(in); got != want {
			t.Errorf("fieldLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFreeformTags covers the tag_set fix: a blank input is "missing" (nil), an
// explicit {} is a valid empty map (clear-all), and malformed input errors.
func TestFreeformTags(t *testing.T) {
	in := func(v string) []*core.Connection { return []*core.Connection{sc("tags", v)} }

	if m, err := FreeformTags("tags", in("")); err != nil || m != nil {
		t.Errorf("FreeformTags(blank) = (%v, %v), want (nil, nil)", m, err)
	}
	// {} → non-nil empty map, so tag_set can distinguish "clear all" from "missing".
	m, err := FreeformTags("tags", in("{}"))
	if err != nil || m == nil || len(m) != 0 {
		t.Errorf("FreeformTags(\"{}\") = (%v, %v), want (non-nil empty map, nil)", m, err)
	}
	m, err = FreeformTags("tags", in(`{"env":"prod","owner":"ops"}`))
	if err != nil || len(m) != 2 || m["env"] != "prod" {
		t.Errorf("FreeformTags(valid) = (%v, %v)", m, err)
	}
	if _, err := FreeformTags("tags", in(`{"env":`)); err == nil {
		t.Error("FreeformTags(malformed) did not error")
	}
}

// TestOptionalBool covers the default handling used by the safety-critical
// delete_boot_volume / force flags.
func TestOptionalBool(t *testing.T) {
	none := []*core.Connection{}
	if OptionalBool("delete_boot_volume", none, false) != false {
		t.Error("OptionalBool(unset, def=false) should be false")
	}
	if OptionalBool("x", none, true) != true {
		t.Error("OptionalBool(unset, def=true) should return the default true")
	}
	set := []*core.Connection{{Name: "force", Type: core.ConnectionTypeBoolean, Value: true}}
	if OptionalBool("force", set, false) != true {
		t.Error("OptionalBool(set true, def=false) should be true")
	}
}
