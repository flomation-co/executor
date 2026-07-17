package azure_tables_entity_get

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

func TestExecuteGetsRow(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", `W/"datetime'2026-07-17T10%3A00%3A00.0000000Z'"`)
		_, _ = w.Write([]byte(`{"odata.metadata":"http://127.0.0.1:10002/devstoreaccount1/$metadata#Orders/@Element","odata.etag":"W/\"x\"","PartitionKey":"uk","RowKey":"1001","Total":42,"Timestamp":"2026-07-17T10:00:00Z"}`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"), str("partition_key", "uk"), str("row_key", "1001")))
	mustSucceed(t, out, err)

	if gotPath != "/devstoreaccount1/Orders(PartitionKey='uk',RowKey='1001')" {
		t.Errorf("path = %s", gotPath)
	}
	result := out["result"].(map[string]interface{})
	// odata.metadata echoes the endpoint URL, and the result is persisted in
	// the run record and forwarded downstream — it must not survive shaping.
	if _, present := result["odata.metadata"]; present {
		t.Errorf("odata.metadata leaked into the result: %v", result)
	}
	if _, present := result["odata.etag"]; present {
		t.Errorf("odata.etag must be lifted to etag, not left in place: %v", result)
	}
	if result["etag"] == nil {
		t.Errorf("etag missing from the result: %v", result)
	}
	if result["Timestamp"] == nil {
		t.Errorf("Timestamp is the operator's, not noise — it must survive: %v", result)
	}
	if result["Total"] != float64(42) {
		t.Errorf("Total = %v", result["Total"])
	}
}

func TestExecuteSelectKeepsTheIdentity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", `W/"x"`)
		_, _ = w.Write([]byte(`{"PartitionKey":"uk","RowKey":"1001","Total":42,"Customer":"Acme"}`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"), str("partition_key", "uk"), str("row_key", "1001"),
		str("select", "Total")))
	mustSucceed(t, out, err)

	result := out["result"].(map[string]interface{})
	if result["Customer"] != nil {
		t.Errorf("Customer was not selected: %v", result)
	}
	// A projected row that cannot be fed back into Update Row is a trap.
	if result["PartitionKey"] != "uk" || result["RowKey"] != "1001" || result["etag"] == nil {
		t.Errorf("the identity and etag must survive a projection: %v", result)
	}
}

func TestExecuteMissingRowIsSoftError(t *testing.T) {
	srv := errorServer(http.StatusNotFound, "ResourceNotFound")
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"), str("partition_key", "uk"), str("row_key", "nope")))
	mustSoftFail(t, out, err, "no entity with that PartitionKey and RowKey")
}

func TestExecuteRejectsKeyWithReservedCharacter(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		str("table", "Orders"), str("partition_key", "uk/south"), str("row_key", "1001")))
	mustSoftFail(t, out, err, "must not contain")
}
