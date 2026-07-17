package azure_storage_blob_generate_sas

import (
	"net/url"
	"strings"
	"testing"
	"time"

	core "flomation.app/automate/executor"
)

const testKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

// endpoint is fixed: this action signs locally and makes no HTTP call, so there
// is no server to stand up.
const endpoint = "https://devstoreaccount1.blob.core.windows.net"

func baseInputs(extra ...*core.Connection) []*core.Connection {
	inputs := []*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "devstoreaccount1"},
		{Name: "account_key", Type: core.ConnectionTypeSecret, Value: testKey},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint},
	}
	return append(inputs, extra...)
}

// sasQuery splits a sas_url into its resource URL and parsed token.
func sasQuery(t *testing.T, sasURL string) (string, url.Values) {
	t.Helper()
	base, query, found := strings.Cut(sasURL, "?")
	if !found {
		t.Fatalf("sas_url %q carries no token", sasURL)
	}
	q, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("sas_url query %q: %v", query, err)
	}
	return base, q
}

// TestExecuteGeneratesBlobSAS pins the URL shape and every token field the
// service reads. (The string-to-sign itself has fixed vectors in the package's
// common_test.go — not repeated here.)
func TestExecuteGeneratesBlobSAS(t *testing.T) {
	before := time.Now()
	out, err := Execute(&core.Flow{}, nil, baseInputs(
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "reports/summary final.pdf"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}

	// The URL is the escaped resource path with the token as its query string.
	base, q := sasQuery(t, out["sas_url"].(string))
	if base != endpoint+"/my-container/reports/summary%20final.pdf" {
		t.Errorf("sas_url resource = %q", base)
	}
	if q.Get("sv") != "2023-11-03" {
		t.Errorf("sv = %q, want the pinned service version", q.Get("sv"))
	}
	if q.Get("sr") != "b" {
		t.Errorf("sr = %q, want b for a blob SAS", q.Get("sr"))
	}
	if q.Get("sp") != "r" {
		t.Errorf("sp = %q, want the read-only default", q.Get("sp"))
	}
	if q.Get("sig") == "" {
		t.Errorf("token carries no signature: %v", q)
	}
	// Slots the operator didn't fill must not appear at all.
	for _, unset := range []string{"st", "sip", "rscd"} {
		if q.Has(unset) {
			t.Errorf("%s = %q, want omitted", unset, q.Get(unset))
		}
	}

	// sas_token is the same query string, without the "?".
	if out["sas_token"] != strings.TrimPrefix(out["sas_url"].(string), base+"?") {
		t.Errorf("sas_token = %v does not match the sas_url query", out["sas_token"])
	}

	// The default lifetime is 24h from now.
	expiresAt, err := time.Parse(time.RFC3339, out["expires_at"].(string))
	if err != nil {
		t.Fatalf("expires_at %v: %v", out["expires_at"], err)
	}
	wantLow, wantHigh := before.Add(24*time.Hour-time.Minute), time.Now().Add(24*time.Hour+time.Minute)
	if expiresAt.Before(wantLow) || expiresAt.After(wantHigh) {
		t.Errorf("expires_at = %v, want ~24h out", expiresAt)
	}
	// se in the token is the same instant, in the SAS's own format.
	if q.Get("se") != expiresAt.UTC().Format("2006-01-02T15:04:05Z") {
		t.Errorf("se = %q, expires_at = %v", q.Get("se"), expiresAt)
	}

	result := out["result"].(map[string]interface{})
	if result["resource"] != "blob" || result["permissions"] != "r" {
		t.Errorf("result = %#v", result)
	}
	if !strings.Contains(out["tool_result"].(string), "SAS link") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

// TestExecuteGeneratesContainerSAS — a container SAS signs sr=c and needs no
// blob name.
func TestExecuteGeneratesContainerSAS(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs(
		&core.Connection{Name: "resource", Type: core.ConnectionTypeString, Value: "container"},
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "permissions", Type: core.ConnectionTypeString, Value: "rl"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	base, q := sasQuery(t, out["sas_url"].(string))
	if base != endpoint+"/my-container" {
		t.Errorf("sas_url resource = %q, want the container path", base)
	}
	if q.Get("sr") != "c" || q.Get("sp") != "rl" {
		t.Errorf("sr = %q sp = %q", q.Get("sr"), q.Get("sp"))
	}
	if out["id"] != "my-container" {
		t.Errorf("id = %v", out["id"])
	}
}

// TestExecuteOptionalSlots — start, IP range and content disposition reach the
// token when set.
func TestExecuteOptionalSlots(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs(
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "report.pdf"},
		&core.Connection{Name: "permissions", Type: core.ConnectionTypeString, Value: "rw"},
		&core.Connection{Name: "start", Type: core.ConnectionTypeDateTime, Value: "2026-07-17T09:00:00Z"},
		&core.Connection{Name: "expiry", Type: core.ConnectionTypeDateTime, Value: "2026-07-18T09:00:00Z"},
		&core.Connection{Name: "ip_range", Type: core.ConnectionTypeString, Value: "168.1.5.60-168.1.5.70"},
		&core.Connection{Name: "content_disposition", Type: core.ConnectionTypeString, Value: `attachment; filename="report.pdf"`},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	_, q := sasQuery(t, out["sas_url"].(string))
	if q.Get("st") != "2026-07-17T09:00:00Z" {
		t.Errorf("st = %q", q.Get("st"))
	}
	if q.Get("se") != "2026-07-18T09:00:00Z" {
		t.Errorf("se = %q", q.Get("se"))
	}
	if q.Get("sip") != "168.1.5.60-168.1.5.70" {
		t.Errorf("sip = %q", q.Get("sip"))
	}
	if q.Get("rscd") != `attachment; filename="report.pdf"` {
		t.Errorf("rscd = %q", q.Get("rscd"))
	}
	result := out["result"].(map[string]interface{})
	if result["startsAt"] != "2026-07-17T09:00:00Z" {
		t.Errorf("result = %#v", result)
	}
}

// TestExecuteExplicitExpiryWinsOverHours — the two expiry inputs have a defined
// precedence.
func TestExecuteExplicitExpiryWinsOverHours(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs(
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "report.pdf"},
		&core.Connection{Name: "expiry_hours", Type: core.ConnectionTypeInteger, Value: 1},
		&core.Connection{Name: "expiry", Type: core.ConnectionTypeDateTime, Value: "2026-12-31T23:59:59Z"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if out["expires_at"] != "2026-12-31T23:59:59Z" {
		t.Errorf("expires_at = %v, want the explicit expiry to win", out["expires_at"])
	}
}

func TestExecuteExpiryHours(t *testing.T) {
	before := time.Now()
	out, err := Execute(&core.Flow{}, nil, baseInputs(
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "report.pdf"},
		&core.Connection{Name: "expiry_hours", Type: core.ConnectionTypeInteger, Value: 2},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	expiresAt, err := time.Parse(time.RFC3339, out["expires_at"].(string))
	if err != nil {
		t.Fatalf("expires_at: %v", err)
	}
	if expiresAt.Before(before.Add(2*time.Hour-time.Minute)) || expiresAt.After(time.Now().Add(2*time.Hour+time.Minute)) {
		t.Errorf("expires_at = %v, want ~2h out", expiresAt)
	}
}

// TestExecuteRefusesEntraAuth — a service SAS is signed with the account key;
// a service principal has none, so this must fail cleanly rather than emit an
// unsigned or bogus link.
func TestExecuteRefusesEntraAuth(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "devstoreaccount1"},
		{Name: "auth_method", Type: core.ConnectionTypeString, Value: "entra"},
		{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Value: "11111111-1111-1111-1111-111111111111"},
		{Name: "azure_client_id", Type: core.ConnectionTypeString, Value: "22222222-2222-2222-2222-222222222222"},
		{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Value: "super-secret-value"},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint},
		{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		{Name: "blob_name", Type: core.ConnectionTypeString, Value: "report.pdf"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	msg := out["error"].(string)
	if out["success"] != false || msg != "SAS generation requires Shared Key auth" {
		t.Errorf("out = %v", out)
	}
	if _, leaked := out["sas_url"]; leaked {
		t.Errorf("a refused SAS still emitted a URL: %v", out["sas_url"])
	}
	if strings.Contains(msg, "super-secret-value") {
		t.Errorf("error leaked the client secret: %q", msg)
	}
}

func TestExecuteValidatesInputs(t *testing.T) {
	cases := []struct {
		name  string
		extra []*core.Connection
		want  string
	}{
		{
			name:  "blob resource without a name",
			extra: []*core.Connection{{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"}},
			want:  "blob_name is required",
		},
		{
			name: "permissions out of canonical order",
			extra: []*core.Connection{
				{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
				{Name: "blob_name", Type: core.ConnectionTypeString, Value: "r.pdf"},
				{Name: "permissions", Type: core.ConnectionTypeString, Value: "wr"},
			},
			want: "out of order",
		},
		{
			name: "unknown permission character",
			extra: []*core.Connection{
				{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
				{Name: "blob_name", Type: core.ConnectionTypeString, Value: "r.pdf"},
				{Name: "permissions", Type: core.ConnectionTypeString, Value: "rz"},
			},
			want: "is not valid",
		},
		{
			name: "unknown resource",
			extra: []*core.Connection{
				{Name: "resource", Type: core.ConnectionTypeString, Value: "queue"},
				{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
			},
			want: `resource "queue" is not valid`,
		},
		{
			name: "expiry in the past",
			extra: []*core.Connection{
				{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
				{Name: "blob_name", Type: core.ConnectionTypeString, Value: "r.pdf"},
				{Name: "expiry", Type: core.ConnectionTypeDateTime, Value: "2020-01-01T00:00:00Z"},
			},
			want: "expiry is in the past",
		},
		{
			name: "expiry before start",
			extra: []*core.Connection{
				{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
				{Name: "blob_name", Type: core.ConnectionTypeString, Value: "r.pdf"},
				{Name: "start", Type: core.ConnectionTypeDateTime, Value: "2030-01-02T00:00:00Z"},
				{Name: "expiry", Type: core.ConnectionTypeDateTime, Value: "2030-01-01T00:00:00Z"},
			},
			want: "expiry must be after start",
		},
		{
			name: "unparseable expiry",
			extra: []*core.Connection{
				{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
				{Name: "blob_name", Type: core.ConnectionTypeString, Value: "r.pdf"},
				{Name: "expiry", Type: core.ConnectionTypeDateTime, Value: "next tuesday"},
			},
			want: "is not a recognised timestamp",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Execute(&core.Flow{}, nil, baseInputs(tc.extra...))
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out["success"] != false || !strings.Contains(out["error"].(string), tc.want) {
				t.Errorf("out = %v, want an error containing %q", out, tc.want)
			}
		})
	}
}

// TestExecuteNeverEchoesTheAccountKey — the token is derived from the key; the
// key itself must never appear in any output.
func TestExecuteNeverEchoesTheAccountKey(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs(
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "blob_name", Type: core.ConnectionTypeString, Value: "report.pdf"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for k, v := range out {
		if s, ok := v.(string); ok && strings.Contains(s, testKey) {
			t.Errorf("output %q leaked the account key: %q", k, s)
		}
	}
}
