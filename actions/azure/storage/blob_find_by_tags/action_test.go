package azure_storage_blob_find_by_tags

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

// The Find Blobs by Tags envelope: account-wide, so each hit names its
// container and carries the matched tags rather than full properties.
const pageOne = `<?xml version="1.0" encoding="utf-8"?>
<EnumerationResults ServiceEndpoint="https://devstoreaccount1.blob.core.windows.net/">
  <Where>"project"='alpha'</Where>
  <Blobs>
    <Blob>
      <Name>reports/a.pdf</Name>
      <ContainerName>my-container</ContainerName>
      <Tags><TagSet><Tag><Key>project</Key><Value>alpha</Value></Tag></TagSet></Tags>
    </Blob>
  </Blobs>
  <NextMarker>2!MTk</NextMarker>
</EnumerationResults>`

const pageTwo = `<?xml version="1.0" encoding="utf-8"?>
<EnumerationResults>
  <Blobs>
    <Blob>
      <Name>archive/b.pdf</Name>
      <ContainerName>other-container</ContainerName>
      <Tags><TagSet><Tag><Key>project</Key><Value>alpha</Value></Tag></TagSet></Tags>
    </Blob>
  </Blobs>
  <NextMarker/>
</EnumerationResults>`

// TestExecuteFindsByTagsWithPagination pins the account-scoped search route,
// the where expression, and the marker walk.
func TestExecuteFindsByTagsWithPagination(t *testing.T) {
	var requests int
	var firstQuery, gotPath, secondMarker string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		gotPath = r.URL.EscapedPath()
		q := r.URL.Query()
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
		&core.Connection{Name: "where", Type: core.ConnectionTypeString, Value: `"project"='alpha' AND "status"='final'`},
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
	// Account-wide search lives at the service root, not under a container.
	if gotPath != "/" {
		t.Errorf("path = %q, want the account root", gotPath)
	}
	if secondMarker != "2!MTk" {
		t.Errorf("second page marker = %q", secondMarker)
	}

	q, err := url.ParseQuery(firstQuery)
	if err != nil {
		t.Fatalf("query %q: %v", firstQuery, err)
	}
	if q.Get("comp") != "blobs" {
		t.Errorf("comp = %q, want blobs", q.Get("comp"))
	}
	// The expression travels verbatim (quotes and all) once URL-decoded.
	if q.Get("where") != `"project"='alpha' AND "status"='final'` {
		t.Errorf("where = %q", q.Get("where"))
	}
	if q.Get("maxresults") != "5000" {
		t.Errorf("maxresults = %q, want 5000 when returning all", q.Get("maxresults"))
	}

	if out["count"] != 2 {
		t.Errorf("count = %v", out["count"])
	}
	items := out["results"].([]interface{})
	first := items[0].(map[string]interface{})
	if first["name"] != "reports/a.pdf" || first["container"] != "my-container" {
		t.Errorf("first = %#v", first)
	}
	if first["tags"].(map[string]interface{})["project"] != "alpha" {
		t.Errorf("tags = %#v", first["tags"])
	}
	second := items[1].(map[string]interface{})
	if second["container"] != "other-container" {
		t.Errorf("second = %#v, want the hit from another container", second)
	}
	if !strings.Contains(out["tool_result"].(string), "Found 2 blobs") {
		t.Errorf("tool_result = %v", out["tool_result"])
	}
}

// TestExecuteSinglePageHonoursLimit — without Return All, one page only, sized
// by the limit, even though the service offered a NextMarker.
func TestExecuteSinglePageHonoursLimit(t *testing.T) {
	var requests int
	var gotMaxResults string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		gotMaxResults = r.URL.Query().Get("maxresults")
		_, _ = w.Write([]byte(pageOne))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "where", Type: core.ConnectionTypeString, Value: `"project"='alpha'`},
		&core.Connection{Name: "limit", Type: core.ConnectionTypeInteger, Value: 10},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if requests != 1 {
		t.Errorf("requests = %d, want 1", requests)
	}
	if gotMaxResults != "10" {
		t.Errorf("maxresults = %q", gotMaxResults)
	}
	if out["count"] != 1 {
		t.Errorf("count = %v", out["count"])
	}
}

// TestExecuteClampsLimitToServiceMaximum — a wild limit is clamped rather than
// sent for the service to reject.
func TestExecuteClampsLimitToServiceMaximum(t *testing.T) {
	var gotMaxResults string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMaxResults = r.URL.Query().Get("maxresults")
		_, _ = w.Write([]byte(pageTwo))
	}))
	defer srv.Close()

	if _, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "where", Type: core.ConnectionTypeString, Value: `"project"='alpha'`},
		&core.Connection{Name: "limit", Type: core.ConnectionTypeInteger, Value: 999999},
	)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotMaxResults != "5000" {
		t.Errorf("maxresults = %q, want the 5000 ceiling", gotMaxResults)
	}
}

func TestExecuteMissingWhereIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "where is required") {
		t.Errorf("out = %v", out)
	}
}

// TestExecuteBadExpressionIsSoftError — a malformed where expression comes back
// as the service's own parse error, cleanly.
func TestExecuteBadExpressionIsSoftError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>InvalidQueryParameterValue</Code><Message>Error parsing query at or near position 0.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "where", Type: core.ConnectionTypeString, Value: "project = alpha"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	msg := out["error"].(string)
	// The SDK surfaces the service error as a verbose block (ERROR CODE line plus
	// the raw <Error><Code>…</Code><Message>…</Message></Error> XML); the code and
	// the parse message must both survive into the soft error.
	if out["success"] != false ||
		!strings.Contains(msg, "InvalidQueryParameterValue") ||
		!strings.Contains(msg, "Error parsing query") {
		t.Errorf("out = %v, want the service's code and parse message", out)
	}
	if strings.Contains(msg, testKey) {
		t.Errorf("error leaked the account key: %q", msg)
	}
}
