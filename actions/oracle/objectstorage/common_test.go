package objectstorage

import (
	"strings"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"

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

// TestGetAuthRejectsBadRegion confirms the region guard fires in GetAuth before
// the region ever reaches the SDK's host builder — the concrete SSRF vector. The
// PAR URL host is derived from that same validated region (via the client), so
// the guard protects the presigned-URL output too.
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
	if strings.Contains(err.Error(), "BEGIN RSA") {
		t.Error("GetAuth region error leaked key material")
	}
}

// TestGetAuthRedactsMalformedKey is the malformed-key echo path: a key that
// passes the non-empty check but fails to parse. The resulting error flows into
// ErrorResult's error AND tool_result, so neither may contain key material.
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
	res := ErrorResult(err.Error())
	for _, k := range []string{"error", "tool_result"} {
		if s, _ := res[k].(string); strings.Contains(s, secretBody) {
			t.Errorf("ErrorResult[%q] leaked key material", k)
		}
	}
}

// TestStringMap covers the JSON-object helper shared by tags (bucket_create) and
// custom metadata (object_put): blank is "missing" (nil), an explicit {} is a
// valid empty map, and malformed input errors with the field label.
func TestStringMap(t *testing.T) {
	in := func(v string) []*core.Connection { return []*core.Connection{sc("metadata", v)} }

	if m, err := StringMap("metadata", "metadata", in("")); err != nil || m != nil {
		t.Errorf("StringMap(blank) = (%v, %v), want (nil, nil)", m, err)
	}
	m, err := StringMap("metadata", "metadata", in("{}"))
	if err != nil || m == nil || len(m) != 0 {
		t.Errorf("StringMap(\"{}\") = (%v, %v), want (non-nil empty map, nil)", m, err)
	}
	m, err = StringMap("metadata", "metadata", in(`{"source":"crm","tier":"gold"}`))
	if err != nil || len(m) != 2 || m["source"] != "crm" {
		t.Errorf("StringMap(valid) = (%v, %v)", m, err)
	}
	_, err = StringMap("metadata", "metadata", in(`{"source":`))
	if err == nil || !strings.Contains(err.Error(), "metadata") {
		t.Errorf("StringMap(malformed) err = %v, want it to name the field", err)
	}
	// FreeformTags is StringMap with a "tags" label.
	if _, err := FreeformTags("tags", []*core.Connection{sc("tags", `{"env":`)}); err == nil || !strings.Contains(err.Error(), "tags") {
		t.Errorf("FreeformTags(malformed) err = %v, want it to name tags", err)
	}
}

// TestOptionalBool covers the default handling behind the fail-safe overwrite
// guard: an unset overwrite must read false (guard applied → won't clobber).
func TestOptionalBool(t *testing.T) {
	none := []*core.Connection{}
	if OptionalBool("overwrite", none, false) != false {
		t.Error("OptionalBool(unset overwrite, def=false) should be false so the guard applies")
	}
	if OptionalBool("x", none, true) != true {
		t.Error("OptionalBool(unset, def=true) should return the default true")
	}
	set := []*core.Connection{{Name: "overwrite", Type: core.ConnectionTypeBoolean, Value: true}}
	if OptionalBool("overwrite", set, false) != true {
		t.Error("OptionalBool(set true, def=false) should be true")
	}
	// A String-typed value reads back nil → falls to the default (matches how the
	// editor's Boolean-typed checkbox is the only thing that flips it).
	str := []*core.Connection{sc("overwrite", "true")}
	if OptionalBool("overwrite", str, false) != false {
		t.Error("OptionalBool(String \"true\") should fall to the default, not parse")
	}
}

// TestServiceErrorCode: a plain (non-OCI) error yields ("", 0) so the rename
// 412 special-case only fires on a real OCI service error.
func TestServiceErrorCode(t *testing.T) {
	if code, status := ServiceErrorCode(nil); code != "" || status != 0 {
		t.Errorf("ServiceErrorCode(nil) = (%q, %d), want (\"\", 0)", code, status)
	}
	if code, status := ServiceErrorCode(errPlain("boom")); code != "" || status != 0 {
		t.Errorf("ServiceErrorCode(plain) = (%q, %d), want (\"\", 0)", code, status)
	}
}

type errPlain string

func (e errPlain) Error() string { return string(e) }

// TestFormatTime pins the RFC3339 output shared across the list actions, so the
// same timestamp field is parseable whether it came from create or list.
func TestFormatTime(t *testing.T) {
	if FormatTime(nil) != "" {
		t.Errorf("FormatTime(nil) = %q, want empty", FormatTime(nil))
	}
	ts := time.Date(2026, 7, 21, 10, 30, 0, 0, time.FixedZone("BST", 3600))
	got := FormatTime(&common.SDKTime{Time: ts})
	if got != "2026-07-21T09:30:00Z" {
		t.Errorf("FormatTime = %q, want RFC3339 UTC 2026-07-21T09:30:00Z", got)
	}
}
