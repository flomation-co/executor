package azure_storage_container_get_all

import (
	"net/http"
	"net/http/httptest"
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

// TestExecutePaginatesWithReturnAll walks a two-page marker cursor and checks
// the prefix/include params make it onto the request.
func TestExecutePaginatesWithReturnAll(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		q := r.URL.Query()
		if q.Get("comp") != "list" || q.Get("prefix") != "prod" || q.Get("include") != "metadata" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/xml")
		if q.Get("marker") == "" {
			_, _ = w.Write([]byte(`<EnumerationResults><Containers><Container><Name>prod-a</Name><Metadata><owner>ops</owner></Metadata></Container></Containers><NextMarker>m2</NextMarker></EnumerationResults>`))
			return
		}
		_, _ = w.Write([]byte(`<EnumerationResults><Containers><Container><Name>prod-b</Name></Container></Containers><NextMarker/></EnumerationResults>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "prefix", Type: core.ConnectionTypeString, Value: "prod"},
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
		t.Errorf("requests = %d, want 2 (marker pagination)", requests)
	}
	if out["count"] != 2 {
		t.Errorf("count = %v", out["count"])
	}
	items := out["results"].([]interface{})
	first := items[0].(map[string]interface{})
	if first["name"] != "prod-a" || first["metadata"].(map[string]interface{})["owner"] != "ops" {
		t.Errorf("first = %#v", first)
	}
}

// TestExecuteIncludeReachesSoftDeletedAndSystemContainers — soft-deleted
// containers are invisible to every other action in the node, so include=deleted
// is the only way to audit or drive a restore; system ($logs) needs its own
// token, and the two combine with metadata in one pass.
func TestExecuteIncludeReachesSoftDeletedAndSystemContainers(t *testing.T) {
	var gotInclude string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInclude = r.URL.Query().Get("include")
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<EnumerationResults><Containers><Container><Name>gone</Name><Deleted>true</Deleted><Version>01D</Version></Container></Containers><NextMarker/></EnumerationResults>`))
	}))
	defer srv.Close()

	for _, tc := range []struct{ in, want string }{
		{"deleted", "deleted"},
		{"system", "system"},
		{"metadata,deleted", "metadata,deleted"},
		{" Metadata , System ", "metadata,system"},
	} {
		out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
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

	// The soft-deleted marker survives into the output.
	out, _ := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "include", Type: core.ConnectionTypeComboBox, Value: "deleted"},
	))
	first := out["results"].([]interface{})[0].(map[string]interface{})
	if first["deleted"] != true {
		t.Errorf("listed container = %#v, want deleted flagged", first)
	}
}

func TestExecuteRejectsAnUnknownIncludeValue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("an unknown include value must not reach the service")
	}))
	defer srv.Close()

	// A blob token, not a container one — the service would answer with a flat
	// 400 that names nothing.
	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "include", Type: core.ConnectionTypeComboBox, Value: "tags"},
	))
	if err != nil {
		t.Fatalf("an unknown include value must be a soft failure, got %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), `"tags" is not supported`) {
		t.Errorf("out = %v, want the offending token named", out)
	}
	if !strings.Contains(out["error"].(string), "metadata, deleted, system") {
		t.Errorf("error = %v, want the supported values listed", out["error"])
	}
}

func TestExecuteAPIErrorIsSoft(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>AuthenticationFailed</Code><Message>Server failed to authenticate the request.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "AuthenticationFailed") {
		t.Errorf("out = %v", out)
	}
}
