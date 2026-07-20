package azure_tables_table_get_access_policy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

// devKey is Azurite's well-known development account key — published in
// Microsoft's own documentation, not a secret.
const devKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

// baseInputs is the Azurite-shaped credential block: the account in the URL
// path rather than the host, which is the endpoint style these tests and the
// emulator share.
func baseInputs(endpoint string, extra ...*core.Connection) []*core.Connection {
	inputs := []*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "devstoreaccount1"},
		{Name: "account_key", Type: core.ConnectionTypeSecret, Value: devKey},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint + "/devstoreaccount1"},
	}
	return append(inputs, extra...)
}

func str(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: v}
}

func obj(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: v}
}

func flag(name string, v bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: v}
}

// errorServer answers every request with one Table service error. The
// x-ms-error-code header is what azcore reads the code from, exactly as the
// real service and Azurite send it.
func errorServer(status int, code string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ms-error-code", code)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"odata.error":{"code":"` + code + `","message":{"value":"the service said no"}}}`))
	}))
}

func mustSoftFail(t *testing.T, out map[string]interface{}, err error, want string) {
	t.Helper()
	if err != nil {
		t.Fatalf("must be a soft failure, got hard error: %v", err)
	}
	if out["success"] != false {
		t.Fatalf("expected failure, got %v", out)
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, want) {
		t.Errorf("error = %q, want it to mention %q", msg, want)
	}
}

func mustSucceed(t *testing.T, out map[string]interface{}, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("expected success, got error %v", out["error"])
	}
}

func TestExecuteReadsAccessPolicies(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<SignedIdentifiers>
  <SignedIdentifier>
    <Id>readonly</Id>
    <AccessPolicy>
      <Start>2026-07-17T10:00:00.0000000Z</Start>
      <Expiry>2027-01-01T00:00:00.0000000Z</Expiry>
      <Permission>r</Permission>
    </AccessPolicy>
  </SignedIdentifier>
</SignedIdentifiers>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL, str("table", "Orders")))
	mustSucceed(t, out, err)

	if gotPath != "/devstoreaccount1/Orders" || !strings.Contains(gotQuery, "comp=acl") {
		t.Errorf("request = %s?%s", gotPath, gotQuery)
	}
	if out["count"] != 1 {
		t.Fatalf("count = %v", out["count"])
	}
	policy := out["results"].([]interface{})[0].(map[string]interface{})
	if policy["id"] != "readonly" || policy["permissions"] != "r" {
		t.Errorf("policy = %v", policy)
	}
	// The shape must round-trip back into Set Access Policies unchanged.
	if policy["expiry"] != "2027-01-01T00:00:00Z" {
		t.Errorf("expiry = %v, want RFC3339", policy["expiry"])
	}
}

func TestExecuteEmptyPolicySetIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><SignedIdentifiers />`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL, str("table", "Orders")))
	mustSucceed(t, out, err)
	if out["count"] != 0 {
		t.Errorf("count = %v", out["count"])
	}
	// A nil slice would serialise as null and break a downstream Loop.
	if results, ok := out["results"].([]interface{}); !ok || results == nil {
		t.Errorf("results must be an empty array, not null: %#v", out["results"])
	}
}

func TestExecuteMissingTableIsSoftError(t *testing.T) {
	srv := errorServer(http.StatusNotFound, "TableNotFound")
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL, str("table", "Orders")))
	mustSoftFail(t, out, err, "no table with that name")
}
