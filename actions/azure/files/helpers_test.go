// Shared fixtures for the files_test action tests (the drift tables live in
// files_inputs_drift_test.go, the resource-group tests in files_*_test.go).
package files_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

// testKey is Azurite's well-known public development key. Azurite implements no
// File service, so it is only ever a valid-looking base64 key here — never an
// emulator target.
const testKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

// authInputs builds the Shared Key credential block pointed at an httptest
// server, plus any action inputs.
func authInputs(endpoint string, extra ...*core.Connection) []*core.Connection {
	inputs := []*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "devstoreaccount1"},
		{Name: "account_key", Type: core.ConnectionTypeSecret, Value: testKey},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: endpoint},
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

// azureError writes the File service's XML error envelope.
func azureError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("x-ms-error-code", code)
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?><Error><Code>%s</Code><Message>%s
RequestId:00000000-0000-0000-0000-000000000000</Message></Error>`, code, message)
}

// xmlBody writes an XML response body.
func xmlBody(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>` + body))
}

// wantSoftFailure asserts the (ErrorResult, nil) soft-failure contract: a
// provider or validation problem is DATA on the error port, never a Go error.
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
	// tool_result is what the AI tool loop shows the model; on a failure it must
	// carry the message rather than a stale success summary.
	if out["tool_result"] != out["error"] {
		t.Errorf("tool_result = %v, want the error message", out["tool_result"])
	}
}

// wantSuccess asserts the happy-path contract and returns the result object.
func wantSuccess(t *testing.T, out map[string]interface{}, err error) map[string]interface{} {
	t.Helper()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v, want true (error: %v)", out["success"], out["error"])
	}
	if out["error"] != "" {
		t.Errorf("error = %q, want empty on success", out["error"])
	}
	result, _ := out["result"].(map[string]interface{})
	return result
}

// wantNoSecretLeak is the redaction contract, asserted on the outputs that
// actually travel: every error string is persisted in the run record and
// forwarded to every downstream node.
func wantNoSecretLeak(t *testing.T, out map[string]interface{}) {
	t.Helper()
	for _, key := range []string{"error", "tool_result"} {
		if s, ok := out[key].(string); ok && strings.Contains(s, testKey) {
			t.Errorf("the account key leaked into the %s output: %q", key, s)
		}
	}
}
