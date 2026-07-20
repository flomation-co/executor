package azure_storage_container_delete

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

func TestExecuteDeletesContainer(t *testing.T) {
	var gotMethod, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotQuery = r.Method, r.URL.RawQuery
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || gotMethod != http.MethodDelete || gotQuery != "restype=container" {
		t.Errorf("out=%v request=%s ?%s", out, gotMethod, gotQuery)
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
	// A 404 is a soft failure (success==false, not a Go error), and the action
	// remaps the SDK's ContainerNotFound to a friendly message that names the
	// container rather than surfacing the verbose SDK error — assert that branch.
	msg, _ := out["error"].(string)
	if out["success"] != false || !strings.Contains(msg, "missing") || !strings.Contains(msg, "does not exist") {
		t.Errorf("out = %v", out)
	}
}
