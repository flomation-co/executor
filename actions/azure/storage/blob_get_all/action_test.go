package azure_storage_blob_get_all

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

const testKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

func baseInputs(endpoint string, extra ...*core.Connection) []*core.Connection {
	inputs := []*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "devstoreaccount1"},
		{Name: "account_key", Type: core.ConnectionTypeSecret, Value: testKey},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint},
	}
	return append(inputs, extra...)
}

const pageOne = `<?xml version="1.0" encoding="utf-8"?>
<EnumerationResults ContainerName="https://devstoreaccount1.blob.core.windows.net/my-container">
  <Blobs>
    <Blob>
      <Name>reports/a.pdf</Name>
      <Properties>
        <Last-Modified>Mon, 27 Jul 2026 12:00:00 GMT</Last-Modified>
        <Content-Length>1024</Content-Length>
        <Content-Type>application/pdf</Content-Type>
        <BlobType>BlockBlob</BlobType>
        <AccessTier>Hot</AccessTier>
        <ServerEncrypted>true</ServerEncrypted>
      </Properties>
      <Metadata><owner>ops</owner></Metadata>
    </Blob>
  </Blobs>
  <NextMarker>2!MTk!MDAwMTk</NextMarker>
</EnumerationResults>`

const pageTwo = `<?xml version="1.0" encoding="utf-8"?>
<EnumerationResults>
  <Blobs>
    <Blob>
      <Name>reports/b.pdf</Name>
      <Properties><Content-Length>7</Content-Length></Properties>
    </Blob>
  </Blobs>
  <NextMarker/>
</EnumerationResults>`

// TestExecutePaginatesWithReturnAll walks the marker ↔ NextMarker cursor and
// pins the list query the container-scoped List Blobs call needs.
func TestExecutePaginatesWithReturnAll(t *testing.T) {
	var requests int
	var firstQuery, secondMarker string
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		q := r.URL.Query()
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/xml")
		if q.Get("marker") == "" {
			firstQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(pageOne))
			return
		}
		secondMarker = q.Get("marker")
		_, _ = w.Write([]byte(pageTwo))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "prefix", Type: core.ConnectionTypeString, Value: "reports/"},
		&core.Connection{Name: "include", Type: core.ConnectionTypeComboBox, Value: "metadata"},
		&core.Connection{Name: "return_all", Type: core.ConnectionTypeBoolean, Value: true},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2 (marker pagination)", requests)
	}
	if gotPath != "/my-container" {
		t.Errorf("path = %q, want the container path", gotPath)
	}
	if secondMarker != "2!MTk!MDAwMTk" {
		t.Errorf("second page marker = %q, want the NextMarker echoed back", secondMarker)
	}

	q, err := url.ParseQuery(firstQuery)
	if err != nil {
		t.Fatalf("query %q: %v", firstQuery, err)
	}
	if q.Get("restype") != "container" || q.Get("comp") != "list" {
		t.Errorf("query = %q, want restype=container&comp=list", firstQuery)
	}
	if q.Get("prefix") != "reports/" || q.Get("include") != "metadata" {
		t.Errorf("query = %q", firstQuery)
	}
	// Return All raises the page size to the service maximum to minimise round trips.
	if q.Get("maxresults") != "5000" {
		t.Errorf("maxresults = %q, want 5000 when returning all", q.Get("maxresults"))
	}

	if out["count"] != 2 {
		t.Errorf("count = %v", out["count"])
	}
	items := out["results"].([]interface{})
	first := items[0].(map[string]interface{})
	if first["name"] != "reports/a.pdf" {
		t.Errorf("first = %#v", first)
	}
	props := first["properties"].(map[string]interface{})
	if props["contentLength"] != int64(1024) || props["blobType"] != "BlockBlob" || props["serverEncrypted"] != true {
		t.Errorf("properties = %#v", props)
	}
	if first["metadata"].(map[string]interface{})["owner"] != "ops" {
		t.Errorf("metadata = %#v", first["metadata"])
	}
	if !strings.Contains(out["tool_result"].(string), "Listed 2 blobs in my-container") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

// TestExecuteCombinesIncludeValues — the reason `include` is free text: Azure
// takes a comma-separated list, and metadata + tags in ONE pass is what saves a
// tag-driven cleanup flow from listing twice and joining client-side.
func TestExecuteCombinesIncludeValues(t *testing.T) {
	var gotInclude string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInclude = r.URL.Query().Get("include")
		_, _ = w.Write([]byte(pageTwo))
	}))
	defer srv.Close()

	for _, tc := range []struct{ in, want string }{
		{"metadata,tags", "metadata,tags"},
		{" Metadata , Tags ", "metadata,tags"},
		{"uncommittedblobs", "uncommittedblobs"}, // unreachable before: not in the old Options set
		{"copy", "copy"},
	} {
		out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
			&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
			&core.Connection{Name: "include", Type: core.ConnectionTypeComboBox, Value: tc.in},
		))
		if err != nil {
			t.Fatalf("Execute(%q): %v", tc.in, err)
		}
		if out["success"] != true {
			t.Fatalf("Execute(%q): %v", tc.in, out["error"])
		}
		if gotInclude != tc.want {
			t.Errorf("include=%q on the wire for input %q, want %q", gotInclude, tc.in, tc.want)
		}
	}
}

// TestExecuteRejectsAnUnknownIncludeValue — the service answers an unknown
// include token with a flat 400 that names nothing, so it is caught here.
func TestExecuteRejectsAnUnknownIncludeValue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("an unknown include value must not reach the service")
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "include", Type: core.ConnectionTypeComboBox, Value: "metadata,snapshot"},
	))
	if err != nil {
		t.Fatalf("an unknown include value must be a soft failure, got %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), `"snapshot" is not supported`) {
		t.Errorf("out = %v, want the offending token named", out)
	}
	if !strings.Contains(out["error"].(string), "snapshots") {
		t.Errorf("error = %v, want the supported values listed", out["error"])
	}
}

// TestExecuteSinglePageHonoursLimit — without Return All the action must take
// the first page and stop, even though the service offered a NextMarker.
func TestExecuteSinglePageHonoursLimit(t *testing.T) {
	var requests int
	var gotMaxResults string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		gotMaxResults = r.URL.Query().Get("maxresults")
		_, _ = w.Write([]byte(pageOne)) // carries a NextMarker
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "limit", Type: core.ConnectionTypeInteger, Value: 3},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if requests != 1 {
		t.Errorf("requests = %d, want 1 (no cursor walk without Return All)", requests)
	}
	if gotMaxResults != "3" {
		t.Errorf("maxresults = %q, want the limit", gotMaxResults)
	}
	if out["count"] != 1 {
		t.Errorf("count = %v", out["count"])
	}
}

// TestExecuteDefaultsLimit — an unset limit falls back to 50, not to unbounded.
func TestExecuteDefaultsLimit(t *testing.T) {
	var gotMaxResults string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMaxResults = r.URL.Query().Get("maxresults")
		_, _ = w.Write([]byte(pageTwo))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if gotMaxResults != "50" {
		t.Errorf("maxresults = %q, want the default 50", gotMaxResults)
	}
	// An unset `include` must not put an empty param on the wire.
	if out["count"] != 1 {
		t.Errorf("count = %v", out["count"])
	}
}

func TestExecuteOmitsUnsetOptionalParams(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(pageTwo))
	}))
	defer srv.Close()

	if _, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
	)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.Contains(gotQuery, "prefix=") || strings.Contains(gotQuery, "include=") {
		t.Errorf("query = %q, want no empty prefix/include params", gotQuery)
	}
}

// TestExecuteReturnAllStopsAtTheSafetyCap — a container that never stops
// offering a NextMarker must not spin unbounded requests: the walk stops at the
// backstop and SAYS it truncated rather than quietly returning a partial list.
func TestExecuteReturnAllStopsAtTheSafetyCap(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		// Always another page.
		_, _ = w.Write([]byte(`<EnumerationResults><Blobs><Blob><Name>b.pdf</Name></Blob></Blobs><NextMarker>more</NextMarker></EnumerationResults>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "return_all", Type: core.ConnectionTypeBoolean, Value: true},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if requests != 200 {
		t.Errorf("requests = %d, want the 200-page backstop to stop the walk", requests)
	}
	if !strings.Contains(out["tool_result"].(string), "stopped at the pagination safety cap") {
		t.Errorf("tool_result = %v, want the truncation flagged", out["tool_result"])
	}
}

func TestExecuteContainerNotFoundIsSoft(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>ContainerNotFound</Code><Message>The specified container does not exist.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	msg := out["error"].(string)
	// A ContainerNotFound is intercepted and rendered as the friendly message the
	// action builds (the SDK correctly surfaced the ContainerNotFound code, which
	// is what HasCode branches on).
	if out["success"] != false || !strings.Contains(msg, `container "my-container" was not found`) {
		t.Errorf("out = %v", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
}

func TestExecuteMissingContainerIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "container is required") {
		t.Errorf("out = %v", out)
	}
}
