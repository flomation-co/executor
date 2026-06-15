package script_python

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// helpers (parallel to the JS test file)

func stringInput(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeText, Value: value}
}

func objectInput(name string, value map[string]interface{}) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: value}
}

func kvInput(name string, pairs ...[2]string) *core.Connection {
	kvs := make([]interface{}, 0, len(pairs))
	for _, p := range pairs {
		kvs = append(kvs, map[string]interface{}{"key": p[0], "value": p[1]})
	}
	return &core.Connection{Name: name, Type: core.ConnectionTypeKeyValueArray, Value: kvs}
}

// The first test pays the CPython compilation cost (~2-3 seconds);
// subsequent tests share the cached compiled module so they run
// in ~150ms each.
func TestExecute_DeclaredOutputs_FromScript(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CPython-WASI integration test in short mode")
	}
	RegisterTestingT(t)

	conns := []*core.Connection{
		stringInput("code", "outputs['total'] = inputs['a'] + inputs['b']"),
		objectInput("inputs_data", map[string]interface{}{"a": 2.0, "b": 3.0}),
		kvInput("outputs", [2]string{"total", "number"}),
	}
	out, err := Execute(&core.Flow{}, nil, conns)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["total"]).To(BeNumerically("==", 5))
}

func TestExecute_PrintCapturedAsLogs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CPython-WASI integration test in short mode")
	}
	RegisterTestingT(t)

	conns := []*core.Connection{
		stringInput("code", "print('hi from python')\noutputs['ok'] = True"),
	}
	out, err := Execute(&core.Flow{}, nil, conns)
	Expect(err).To(BeNil())
	Expect(out["logs"]).To(ContainSubstring("hi from python"))
}

func TestExecute_NoNetwork_SocketRefused(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CPython-WASI integration test in short mode")
	}
	RegisterTestingT(t)

	// urllib.request should fail because WASI grants no sockets.
	// We don't assert on the exact error string (it varies by
	// CPython version) — just that the script fails.
	conns := []*core.Connection{
		stringInput("code", `
import urllib.request
urllib.request.urlopen('http://example.com', timeout=2).read()
outputs['unreachable'] = True
`),
	}
	out, err := Execute(&core.Flow{}, nil, conns)
	Expect(err).NotTo(BeNil())
	Expect(out["success"]).To(Equal(false))
}

// Top-level `return` should work — the bootstrap AST-wraps the
// user code inside a function so this is valid Python. Matches the
// JavaScript node's IIFE behaviour.
func TestExecute_TopLevelReturn_Works(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CPython-WASI integration test in short mode")
	}
	RegisterTestingT(t)

	conns := []*core.Connection{
		stringInput("code", "return {'total': inputs['a'] + inputs['b']}"),
		objectInput("inputs_data", map[string]interface{}{"a": 4.0, "b": 5.0}),
		kvInput("outputs", [2]string{"total", "number"}),
	}
	out, err := Execute(&core.Flow{}, nil, conns)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["total"]).To(BeNumerically("==", 9))
}

// Return value MERGES on top of mutated outputs. Both should
// surface; the explicit return wins on conflicting keys.
func TestExecute_ReturnMergesWithOutputs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CPython-WASI integration test in short mode")
	}
	RegisterTestingT(t)

	conns := []*core.Connection{
		stringInput("code", "outputs['a'] = 1\noutputs['b'] = 2\nreturn {'b': 99}"),
		kvInput("outputs", [2]string{"a", "number"}, [2]string{"b", "number"}),
	}
	out, err := Execute(&core.Flow{}, nil, conns)
	Expect(err).To(BeNil())
	Expect(out["a"]).To(BeNumerically("==", 1))
	Expect(out["b"]).To(BeNumerically("==", 99))
}

func TestExecute_SecretInCode_Refused(t *testing.T) {
	RegisterTestingT(t)

	conns := []*core.Connection{
		// Slack bot token shape — should match the detector. nosecret
		stringInput("code", "API = 'xoxb-1234567890-1234567890-AbCdEfGhIjKlMnOpQrStUvWx'"), // nosecret
	}
	out, err := Execute(&core.Flow{}, nil, conns)
	Expect(err).NotTo(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("literal secret"))
}
