package azure_storage_container_create

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

func TestExecuteCreatesContainer(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotAccess, gotMeta, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.RawQuery
		gotAccess = r.Header.Get("x-ms-blob-public-access")
		gotMeta = r.Header.Get("x-ms-meta-project")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("ETag", `"0x1"`)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
		&core.Connection{Name: "public_access", Type: core.ConnectionTypeString, Value: "blob"},
		&core.Connection{Name: "metadata", Type: core.ConnectionTypeObject, Value: `{"project":"alpha"}`},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v (error: %v)", out["success"], out["error"])
	}
	if gotMethod != http.MethodPut || gotPath != "/my-container" || gotQuery != "restype=container" {
		t.Errorf("request = %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	if gotAccess != "blob" || gotMeta != "alpha" {
		t.Errorf("headers: access=%q meta=%q", gotAccess, gotMeta)
	}
	if !strings.HasPrefix(gotAuth, "SharedKey devstoreaccount1:") {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if out["id"] != "my-container" {
		t.Errorf("id = %v", out["id"])
	}
}

func TestExecuteInvalidNameIsSoftError(t *testing.T) {
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid",
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "Bad--Name"},
	))
	if err != nil {
		t.Fatalf("validation must be a soft failure, got hard error %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "invalid") {
		t.Errorf("out = %v", out)
	}
}

func TestExecuteAlreadyExistsIsSoftError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>ContainerAlreadyExists</Code><Message>The specified container already exists.</Message></Error>`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		&core.Connection{Name: "container", Type: core.ConnectionTypeString, Value: "my-container"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != false || !strings.Contains(out["error"].(string), "already exists") {
		t.Errorf("out = %v", out)
	}
}
