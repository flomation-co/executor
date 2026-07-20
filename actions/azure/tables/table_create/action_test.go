package azure_tables_table_create

import (
	"io"
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

func TestExecuteCreatesTable(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"TableName":"MyTable"}`))
	}))
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL, str("table", "MyTable")))
	mustSucceed(t, out, err)

	if gotMethod != http.MethodPost || gotPath != "/devstoreaccount1/Tables" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if !strings.Contains(gotBody, `"TableName":"MyTable"`) {
		t.Errorf("body = %s", gotBody)
	}
	// aztables signs with SharedKeyLite, NOT the SharedKey scheme the Blob
	// signer emits. If this ever reads "SharedKey ", the SDK changed schemes.
	if !strings.HasPrefix(gotAuth, "SharedKeyLite devstoreaccount1:") {
		t.Errorf("Authorization = %q, want a SharedKeyLite signature", gotAuth)
	}
	if out["id"] != "MyTable" {
		t.Errorf("id = %v", out["id"])
	}
}

func TestExecuteRejectsHyphenatedName(t *testing.T) {
	// The mistake an operator arriving from Blob Storage makes: container names
	// take hyphens, table names do not.
	out, err := Execute(&core.Flow{}, nil, baseInputs("http://unused.invalid", str("table", "my-table")))
	mustSoftFail(t, out, err, "letters and digits only")
}

func TestExecuteAlreadyExistsIsSoftError(t *testing.T) {
	srv := errorServer(http.StatusConflict, "TableAlreadyExists")
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL, str("table", "MyTable")))
	mustSoftFail(t, out, err, "already exists")
}

func TestExecuteIgnoreIfExistsSucceeds(t *testing.T) {
	srv := errorServer(http.StatusConflict, "TableAlreadyExists")
	defer srv.Close()

	out, err := Execute(&core.Flow{}, nil, baseInputs(srv.URL,
		str("table", "MyTable"), flag("ignore_if_exists", true)))
	mustSucceed(t, out, err)

	result := out["result"].(map[string]interface{})
	if result["created"] != false {
		t.Errorf("an ignored conflict must report created=false, got %v", result)
	}
}

func TestExecuteRedactsTheAccountKey(t *testing.T) {
	srv := errorServer(http.StatusForbidden, "AuthenticationFailed")
	defer srv.Close()

	out, _ := Execute(&core.Flow{}, nil, baseInputs(srv.URL, str("table", "MyTable")))
	if msg, _ := out["error"].(string); strings.Contains(msg, devKey) {
		t.Errorf("the account key leaked into the error: %q", msg)
	}
}
