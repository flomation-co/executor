package azure_tables_table_get_all

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

func TestExecuteListsTables(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"value":[{"TableName":"Orders"},{"TableName":"Customers"}]}`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL))
	mustSucceed(t, out, err)

	if gotPath != "/devstoreaccount1/Tables" {
		t.Errorf("path = %s", gotPath)
	}
	if !strings.Contains(gotQuery, "top=50") {
		t.Errorf("query = %s, want the default page limit", gotQuery)
	}
	if out["count"] != 2 {
		t.Fatalf("count = %v", out["count"])
	}
	first := out["results"].([]interface{})[0].(map[string]interface{})
	if first["name"] != "Orders" {
		t.Errorf("first = %v", first)
	}
}

func TestExecuteReturnAllFollowsTheContinuation(t *testing.T) {
	pages := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		w.Header().Set("Content-Type", "application/json")
		if pages == 1 {
			w.Header().Set("x-ms-continuation-NextTableName", "Customers")
			_, _ = w.Write([]byte(`{"value":[{"TableName":"Orders"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"value":[{"TableName":"Customers"}]}`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL, flag("return_all", true)))
	mustSucceed(t, out, err)

	if pages != 2 {
		t.Errorf("fetched %d page(s), want 2", pages)
	}
	if out["count"] != 2 {
		t.Errorf("count = %v, want both pages", out["count"])
	}
}

func TestExecuteSinglePageStopsAtOneRequest(t *testing.T) {
	// Without return_all the continuation must be ignored, not followed.
	pages := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-ms-continuation-NextTableName", "More")
		_, _ = w.Write([]byte(`{"value":[{"TableName":"Orders"}]}`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL))
	mustSucceed(t, out, err)
	if pages != 1 {
		t.Errorf("fetched %d page(s), want exactly 1", pages)
	}
}

func TestExecuteForbiddenIsSoftError(t *testing.T) {
	srv := errorServer(http.StatusForbidden, "AuthorizationPermissionMismatch")
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL))
	mustSoftFail(t, out, err, "Storage Table Data Contributor")
}
