package azure_tables_entity_upsert

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

func TestExecuteUpsertMergesByDefault(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set("ETag", `W/"x"`)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"),
		obj("entity", `{"PartitionKey":"uk","RowKey":"1001","Total":42}`)))
	mustSucceed(t, out, err)

	// Merge is PATCH; replace is PUT. An untouched dropdown must MERGE —
	// replace deletes every property the entity does not mention.
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %s, want PATCH (merge) for an unset update_mode", gotMethod)
	}
	if gotPath != "/devstoreaccount1/Orders(PartitionKey='uk',RowKey='1001')" {
		t.Errorf("path = %s", gotPath)
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "left alone") {
		t.Errorf("the summary must say what merge did, got %q", summary)
	}
}

func TestExecuteUpsertReplaceUsesPut(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Header().Set("ETag", `W/"x"`)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"),
		obj("entity", `{"PartitionKey":"uk","RowKey":"1001","Total":42}`),
		str("update_mode", "replace")))
	mustSucceed(t, out, err)

	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT for replace", gotMethod)
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "removed") {
		t.Errorf("the summary must warn that replace removed fields, got %q", summary)
	}
}

// The same unchecked type assertion as entity_insert's guard, on the other SDK
// entry point: UpsertEntity panics on a non-string PartitionKey.
func TestExecuteMissingPartitionKeyDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a missing PartitionKey panicked instead of soft-failing: %v", r)
		}
	}()

	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("table", "Orders"), obj("entity", `{"RowKey":"1001","Total":42}`)))
	mustSoftFail(t, out, err, "no PartitionKey")
}

func TestExecuteRejectsBadUpdateMode(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("table", "Orders"),
		obj("entity", `{"PartitionKey":"uk","RowKey":"1001"}`),
		str("update_mode", "clobber")))
	mustSoftFail(t, out, err, "merge or replace")
}
