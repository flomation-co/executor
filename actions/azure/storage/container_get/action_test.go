package azure_storage_container_get

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

func TestExecuteReadsHeaderProperties(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.RawQuery != "restype=container" {
			t.Errorf("request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("x-ms-lease-status", "unlocked")
		w.Header().Set("x-ms-blob-public-access", "blob")
		w.Header().Set("x-ms-meta-owner", "ops")
		w.WriteHeader(http.StatusOK)
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
	result := out["result"].(map[string]interface{})
	props := result["properties"].(map[string]interface{})
	if props["leaseStatus"] != "unlocked" || props["blobPublicAccess"] != "blob" {
		t.Errorf("properties = %#v", props)
	}
	if result["metadata"].(map[string]interface{})["owner"] != "ops" {
		t.Errorf("metadata = %#v", result["metadata"])
	}
}

func TestExecuteNotFoundIsSoftError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>ContainerNotFound</Code><Message>The specified container does not exist.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "missing"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "ContainerNotFound") {
		t.Errorf("out = %v", out)
	}
}
