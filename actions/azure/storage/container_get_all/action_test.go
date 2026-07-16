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
		&core.Connection{Name: "include_metadata", Type: core.ConnectionTypeBoolean, Value: true},
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
