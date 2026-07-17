package azure_tables_entity_insert

import (
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

func TestExecuteInsertsRow(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("ETag", `W/"datetime'2026-07-17T10%3A00%3A00.0000000Z'"`)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"),
		obj("entity", `{"PartitionKey":"uk","RowKey":"1001","Total":42}`)))
	mustSucceed(t, out, err)

	if gotMethod != http.MethodPost || gotPath != "/devstoreaccount1/Orders" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if !strings.Contains(gotBody, `"RowKey":"1001"`) {
		t.Errorf("body = %s", gotBody)
	}
	if out["id"] != "uk/1001" {
		t.Errorf("id = %v, want the composite identity", out["id"])
	}
	result := out["result"].(map[string]interface{})
	if result["etag"] == nil || result["etag"] == "" {
		t.Errorf("the etag must reach the result so a later update can use it: %v", result)
	}
}

// TestExecuteNumericPartitionKeyDoesNotPanic is the guard that matters most in
// this package. aztables asserts entity["PartitionKey"].(string) with no
// comma-ok, so an entity carrying a number here reaches the SDK and takes the
// whole executor process down. Our entities come from operator input and flow
// variables, so this is reachable from a flow, not theoretical.
func TestExecuteNumericPartitionKeyDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a numeric PartitionKey panicked instead of soft-failing: %v", r)
		}
	}()

	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("table", "Orders"),
		obj("entity", `{"PartitionKey":42,"RowKey":"1001"}`)))
	mustSoftFail(t, out, err, "must be a string")
}

func TestExecuteMissingRowKeyIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("table", "Orders"),
		obj("entity", `{"PartitionKey":"uk","Total":42}`)))
	mustSoftFail(t, out, err, "no RowKey")
}

func TestExecuteDuplicateIsSoftError(t *testing.T) {
	srv := errorServer(http.StatusConflict, "EntityAlreadyExists")
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"),
		obj("entity", `{"PartitionKey":"uk","RowKey":"1001"}`)))
	mustSoftFail(t, out, err, "Upsert Row")
}
