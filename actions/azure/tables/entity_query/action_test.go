package azure_tables_entity_query

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

func TestExecuteQueriesRows(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.Query().Encode()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"value":[{"odata.etag":"W/\"a\"","PartitionKey":"uk","RowKey":"1001","Total":42}]}`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"),
		str("filter", "PartitionKey eq 'uk' and Total gt 10"),
		str("select", "Total")))
	mustSucceed(t, out, err)

	if gotPath != "/devstoreaccount1/Orders()" {
		t.Errorf("path = %s", gotPath)
	}
	if !strings.Contains(gotQuery, "filter=PartitionKey+eq+%27uk%27") || !strings.Contains(gotQuery, "select=Total") {
		t.Errorf("query = %s", gotQuery)
	}
	row := out["results"].([]interface{})[0].(map[string]interface{})
	// On a query the per-row etag exists only in the body; losing it would
	// leave the operator unable to do a concurrency-checked update.
	if row["etag"] != `W/"a"` {
		t.Errorf("the per-row etag must be lifted out of odata.etag: %v", row)
	}
}

func TestExecuteReturnAllFollowsBothContinuationHeaders(t *testing.T) {
	// Paging round-trips NextPartitionKey AND NextRowKey — the walk is only
	// over when BOTH are absent, unlike Blob's single NextMarker.
	pages := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		w.Header().Set("Content-Type", "application/json")
		if pages == 1 {
			w.Header().Set("x-ms-continuation-NextPartitionKey", "uk")
			w.Header().Set("x-ms-continuation-NextRowKey", "1002")
			_, _ = w.Write([]byte(`{"value":[{"PartitionKey":"uk","RowKey":"1001"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"value":[{"PartitionKey":"uk","RowKey":"1002"}]}`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"), flag("return_all", true)))
	mustSucceed(t, out, err)

	if pages != 2 || out["count"] != 2 {
		t.Errorf("pages = %d, count = %v — want a two-page walk", pages, out["count"])
	}
}

// TestExecuteReturnAllSurvivesAnEmptyPage pins the scan behaviour that breaks
// naive pagers: a filtered scan can return an EMPTY page WITH a continuation
// ("nothing matched in the range I scanned, keep going"). Stopping there would
// silently drop every later match.
func TestExecuteReturnAllSurvivesAnEmptyPage(t *testing.T) {
	pages := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		w.Header().Set("Content-Type", "application/json")
		if pages == 1 {
			w.Header().Set("x-ms-continuation-NextPartitionKey", "uk")
			w.Header().Set("x-ms-continuation-NextRowKey", "5000")
			_, _ = w.Write([]byte(`{"value":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"value":[{"PartitionKey":"uk","RowKey":"9999"}]}`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"), str("filter", "Total gt 1000000"), flag("return_all", true)))
	mustSucceed(t, out, err)

	if out["count"] != 1 {
		t.Errorf("count = %v — the walk stopped on the empty page and lost the match", out["count"])
	}
}

func TestExecuteBadFilterIsSoftError(t *testing.T) {
	srv := errorServer(http.StatusBadRequest, "InvalidInput")
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "Orders"), str("filter", "not an odata filter")))
	mustSoftFail(t, out, err, "InvalidInput")
}
