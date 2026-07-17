// Shared fixtures for the entra_test action tests (the drift tables live in
// entra_inputs_drift_test.go, the resource-group tests in entra_*_test.go).
package entra_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

// authInputs builds the credential block pointed at an httptest server, plus
// any action inputs. The token exchange itself is stubbed per-test with
// entra.SetTokenForTest.
func authInputs(endpoint string, extra ...*core.Connection) []*core.Connection {
	inputs := []*core.Connection{
		{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Value: "tenant-guid"},
		{Name: "azure_client_id", Type: core.ConnectionTypeString, Value: "client-guid"},
		{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Value: "s3cret"},
		{Name: "graph_endpoint", Type: core.ConnectionTypeString, Value: endpoint},
	}
	return append(inputs, extra...)
}

func str(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: val}
}

func text(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeText, Value: val}
}

func obj(name, jsonStr string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: jsonStr}
}

func boolean(name string, v bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: v}
}

func integer(name string, v int) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeInteger, Value: v}
}

func secret(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeSecret, Value: val}
}

// decodeBody parses a captured request body into a generic map.
func decodeBody(t *testing.T, r *http.Request) map[string]interface{} {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	out := map[string]interface{}{}
	if len(b) == 0 {
		return out
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("request body not JSON: %v (%s)", err, b)
	}
	return out
}

// graphError writes a Graph error envelope.
func graphError(w http.ResponseWriter, status int, code, message string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{"code": code, "message": message},
	})
}

// wantSoftFailure asserts the (ErrorResult, nil) soft-failure contract.
func wantSoftFailure(t *testing.T, out map[string]interface{}, err error, contains string) {
	t.Helper()
	if err != nil {
		t.Fatalf("soft failure must return nil error, got %v", err)
	}
	if out["success"] != false {
		t.Fatalf("success = %v, want false (out: %v)", out["success"], out)
	}
	msg, _ := out["error"].(string)
	if contains != "" && !strings.Contains(msg, contains) {
		t.Fatalf("error %q does not contain %q", msg, contains)
	}
}
