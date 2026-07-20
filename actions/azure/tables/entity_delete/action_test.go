package azure_tables_entity_delete

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

func TestExecuteDeletesRow(t *testing.T) {
	var gotMethod, gotPath, gotIfMatch string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotIfMatch = r.Method, r.URL.Path, r.Header.Get("If-Match")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"), str("partition_key", "uk"), str("row_key", "1001")))
	mustSucceed(t, out, err)

	if gotMethod != http.MethodDelete || gotPath != "/devstoreaccount1/Orders(PartitionKey='uk',RowKey='1001')" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if gotIfMatch != "*" {
		t.Errorf("If-Match = %q, want * for a blank etag", gotIfMatch)
	}
	if out["id"] != "uk/1001" {
		t.Errorf("id = %v", out["id"])
	}
}

func TestExecuteStaleETagIsCleanSoftError(t *testing.T) {
	srv := errorServer(http.StatusPreconditionFailed, "UpdateConditionNotSatisfied")
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"), str("partition_key", "uk"), str("row_key", "1001"),
		str("etag", `W/"stale"`)))
	mustSoftFail(t, out, err, "modified since it was read")
}

func TestExecuteIgnoreIfMissingSucceeds(t *testing.T) {
	srv := errorServer(http.StatusNotFound, "ResourceNotFound")
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"), str("partition_key", "uk"), str("row_key", "gone"),
		flag("ignore_if_missing", true)))
	mustSucceed(t, out, err)
	if result := out["result"].(map[string]interface{}); result["deleted"] != false {
		t.Errorf("an ignored 404 must report deleted=false, got %v", result)
	}
}

func TestExecuteMissingRowIsSoftErrorByDefault(t *testing.T) {
	srv := errorServer(http.StatusNotFound, "ResourceNotFound")
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"), str("partition_key", "uk"), str("row_key", "gone")))
	mustSoftFail(t, out, err, "no entity with that PartitionKey and RowKey")
}
