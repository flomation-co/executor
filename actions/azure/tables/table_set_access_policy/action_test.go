package azure_tables_table_set_access_policy

import (
	"fmt"
	"io"
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

func TestExecuteWritesAccessPolicies(t *testing.T) {
	var gotMethod, gotQuery, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotQuery = r.Method, r.URL.RawQuery
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"),
		obj("policies", `[{"id":"readonly","permissions":"r","expiry":"2027-01-01T00:00:00Z"}]`)))
	mustSucceed(t, out, err)

	if gotMethod != http.MethodPut || !strings.Contains(gotQuery, "comp=acl") {
		t.Errorf("request = %s ?%s", gotMethod, gotQuery)
	}
	if !strings.Contains(gotBody, "<Id>readonly</Id>") || !strings.Contains(gotBody, "<Permission>r</Permission>") {
		t.Errorf("body = %s", gotBody)
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "has been removed") {
		t.Errorf("the summary must warn that unlisted policies are gone, got %q", summary)
	}
}

// An empty array is the ONLY way to clear the policies, so it must be a valid
// input rather than read as "nothing supplied".
func TestExecuteEmptyListClearsPolicies(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"), obj("policies", `[]`)))
	mustSucceed(t, out, err)
	if strings.Contains(gotBody, "<SignedIdentifier>") {
		t.Errorf("body = %s, want no identifiers", gotBody)
	}
}

func TestExecuteRejectsSixPolicies(t *testing.T) {
	var policies []string
	for i := 0; i < 6; i++ {
		policies = append(policies, fmt.Sprintf(`{"id":"p%d","permissions":"r"}`, i))
	}
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("table", "Orders"),
		obj("policies", "["+strings.Join(policies, ",")+"]")))
	mustSoftFail(t, out, err, "at most 5")
}

func TestExecuteRejectsBadPermissions(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("table", "Orders"),
		obj("policies", `[{"id":"bad","permissions":"rwx"}]`)))
	mustSoftFail(t, out, err, "raud")
}

func TestExecuteRejectsPolicyWithoutID(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("table", "Orders"),
		obj("policies", `[{"permissions":"r"}]`)))
	mustSoftFail(t, out, err, "has no id")
}
