package azure_storage_container_set_metadata

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

func TestExecuteSetsMetadata(t *testing.T) {
	var gotQuery, gotMeta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		gotMeta = r.Header.Get("x-ms-meta-project")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "metadata", Type: core.ConnectionTypeObject, Value: `{"project":"alpha"}`},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("error: %v", out["error"])
	}
	if !strings.Contains(gotQuery, "comp=metadata") || !strings.Contains(gotQuery, "restype=container") {
		t.Errorf("query = %s", gotQuery)
	}
	if gotMeta != "alpha" {
		t.Errorf("x-ms-meta-project = %q", gotMeta)
	}
}

func TestExecuteMissingMetadataIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "metadata is required") {
		t.Errorf("out = %v", out)
	}
}

func TestExecuteAPIErrorIsSoft(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>ContainerNotFound</Code><Message>The specified container does not exist.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "missing"},
		&core.Connection{Name: "metadata", Type: core.ConnectionTypeObject, Value: `{"a":"b"}`},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "ContainerNotFound") {
		t.Errorf("out = %v", out)
	}
}
