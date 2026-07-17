package azure_tables_entity_update

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

func TestExecuteUpdatesRow(t *testing.T) {
	var gotMethod, gotIfMatch string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotIfMatch = r.Method, r.Header.Get("If-Match")
		w.Header().Set("ETag", `W/"new"`)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"),
		obj("entity", `{"PartitionKey":"uk","RowKey":"1001","Total":43}`)))
	mustSucceed(t, out, err)

	if gotMethod != http.MethodPatch {
		t.Errorf("method = %s, want PATCH (merge) by default", gotMethod)
	}
	// A blank etag means the SDK's "*": overwrite whatever is there. That is
	// last-write-wins, and it is deliberate — the alternative is failing every
	// update by an operator who never read the row first.
	if gotIfMatch != "*" {
		t.Errorf("If-Match = %q, want * for a blank etag", gotIfMatch)
	}
}

func TestExecuteSuppliedETagIsSent(t *testing.T) {
	var gotIfMatch string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIfMatch = r.Header.Get("If-Match")
		w.Header().Set("ETag", `W/"new"`)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"),
		obj("entity", `{"PartitionKey":"uk","RowKey":"1001","Total":43}`),
		str("etag", `W/"datetime'2026-07-17T10%3A00%3A00.0000000Z'"`)))
	mustSucceed(t, out, err)

	if gotIfMatch != `W/"datetime'2026-07-17T10%3A00%3A00.0000000Z'"` {
		t.Errorf("If-Match = %q, want the supplied etag verbatim", gotIfMatch)
	}
}

// A 412 is the whole point of the etag field: someone else wrote the row
// between the read and the write. It must read as that, not as a raw code.
func TestExecuteStaleETagIsCleanSoftError(t *testing.T) {
	srv := errorServer(http.StatusPreconditionFailed, "UpdateConditionNotSatisfied")
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"),
		obj("entity", `{"PartitionKey":"uk","RowKey":"1001","Total":43}`),
		str("etag", `W/"stale"`)))
	mustSoftFail(t, out, err, "modified since it was read")
}

func TestExecuteNumericRowKeyDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a numeric RowKey panicked instead of soft-failing: %v", r)
		}
	}()

	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("table", "Orders"), obj("entity", `{"PartitionKey":"uk","RowKey":1001}`)))
	mustSoftFail(t, out, err, "must be a string")
}
