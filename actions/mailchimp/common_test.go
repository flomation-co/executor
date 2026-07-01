package mailchimp_common

import (
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

func conn(name, typ string, value interface{}) *core.Connection {
	return &core.Connection{Name: name, Type: typ, Value: value}
}

func TestDatacenter(t *testing.T) {
	dc, err := datacenter("abc123def456-us21")
	if err != nil || dc != "us21" {
		t.Errorf("datacenter = %q, %v; want us21", dc, err)
	}
	if _, err := datacenter("nodashkey"); err == nil {
		t.Error("expected error for key without a datacenter suffix")
	}
	if _, err := datacenter("trailingdash-"); err == nil {
		t.Error("expected error for key with empty datacenter suffix")
	}
}

func TestBaseURL(t *testing.T) {
	u, err := baseURL("key-us6")
	if err != nil || u != "https://us6.api.mailchimp.com/3.0" {
		t.Errorf("baseURL = %q, %v", u, err)
	}
}

func TestSubscriberHash(t *testing.T) {
	// Canonical: MD5 of the lowercased, trimmed email — 32 lowercase hex chars.
	h := SubscriberHash("Test@Example.COM")
	if len(h) != 32 {
		t.Fatalf("hash length = %d, want 32 (%q)", len(h), h)
	}
	if h != strings.ToLower(h) {
		t.Errorf("hash should be lowercase hex: %q", h)
	}
	// Case- and whitespace-insensitive.
	if SubscriberHash("  a@b.com ") != SubscriberHash("A@B.COM") {
		t.Error("hash should normalise case and surrounding whitespace")
	}
	if SubscriberHash("a@b.com") == SubscriberHash("c@d.com") {
		t.Error("distinct emails must hash differently")
	}
}

func TestMemberPathHashesEmail(t *testing.T) {
	p := MemberPath("abc", "Ada@Example.com")
	if !strings.HasPrefix(p, "/lists/abc/members/") {
		t.Errorf("unexpected member path: %q", p)
	}
	if strings.Contains(p, "@") {
		t.Errorf("email must be hashed, not embedded raw: %q", p)
	}
}

func TestBuildMergeFields_KVAndJSON(t *testing.T) {
	inputs := []*core.Connection{
		conn("merge_fields", core.ConnectionTypeKeyValueArray, `[{"key":"FNAME","value":"Ada"},{"key":"LNAME","value":"Lovelace"}]`),
		conn("merge_fields_json", core.ConnectionTypeObject, `{"PHONE":"555-0100","LNAME":"Byron"}`),
	}
	mf, err := BuildMergeFields(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if mf["FNAME"] != "Ada" {
		t.Errorf("FNAME = %v", mf["FNAME"])
	}
	if mf["PHONE"] != "555-0100" {
		t.Errorf("PHONE = %v", mf["PHONE"])
	}
	if mf["LNAME"] != "Byron" {
		t.Errorf("JSON should overlay KV; LNAME = %v", mf["LNAME"])
	}
}

func TestBuildMergeFields_Empty(t *testing.T) {
	mf, err := BuildMergeFields(nil)
	if err != nil || len(mf) != 0 {
		t.Errorf("expected empty, got %v (%v)", mf, err)
	}
}

func TestParseJSONObject(t *testing.T) {
	got, err := ParseJSONObject("interests_json", []*core.Connection{
		conn("interests_json", core.ConnectionTypeObject, `{"abc":true}`),
	})
	if err != nil || got["abc"] != true {
		t.Errorf("got %v, err %v", got, err)
	}
	// Absent -> nil, no error.
	if got, err := ParseJSONObject("interests_json", nil); err != nil || got != nil {
		t.Errorf("absent should be nil/nil; got %v %v", got, err)
	}
	// Parsed-map value (engine pre-parsed).
	if got, err := ParseJSONObject("x", []*core.Connection{conn("x", core.ConnectionTypeObject, map[string]interface{}{"k": "v"})}); err != nil || got["k"] != "v" {
		t.Errorf("parsed-map value: got %v %v", got, err)
	}
	// Invalid JSON -> error.
	if _, err := ParseJSONObject("x", []*core.Connection{conn("x", core.ConnectionTypeObject, `{bad`)}); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCheckResponse(t *testing.T) {
	if err := CheckResponse(&APIResponse{StatusCode: 200, Body: []byte(`{}`)}); err != nil {
		t.Errorf("2xx should be nil: %v", err)
	}
	err := CheckResponse(&APIResponse{StatusCode: 404, Body: []byte(`{"type":"...","title":"Resource Not Found","detail":"list not found","status":404}`)})
	if err == nil || !strings.Contains(err.Error(), "Resource Not Found") || !strings.Contains(err.Error(), "list not found") {
		t.Errorf("structured error not surfaced: %v", err)
	}
	err = CheckResponse(&APIResponse{StatusCode: 400, Body: []byte(`{"title":"Bad","detail":"invalid","errors":[{"field":"email_address","message":"required"}]}`)})
	if err == nil || !strings.Contains(err.Error(), "email_address") {
		t.Errorf("field errors not surfaced: %v", err)
	}
	err = CheckResponse(&APIResponse{StatusCode: 500, Body: []byte(`gateway error`)})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("opaque error: %v", err)
	}
}

func TestCSVToList(t *testing.T) {
	got := CSVToList(" subscribe , , unsubscribe ,campaign ")
	if len(got) != 3 || got[0] != "subscribe" || got[2] != "campaign" {
		t.Errorf("CSVToList = %v", got)
	}
	if CSVToList("   ") != nil {
		t.Error("blank -> nil")
	}
}
