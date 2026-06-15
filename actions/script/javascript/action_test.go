package script_javascript

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// helpers ─────────────────────────────────────────────────────────

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

// Happy path: declared outputs returned from the script
func TestExecute_DeclaredOutputs_ReturnedFromScript(t *testing.T) {
	RegisterTestingT(t)

	conns := []*core.Connection{
		stringInput("code", "return { total: inputs.a + inputs.b };"),
		objectInput("inputs_data", map[string]interface{}{"a": 2.0, "b": 3.0}),
		kvInput("outputs", [2]string{"total", "number"}),
	}
	out, err := Execute(&core.Flow{}, nil, conns)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["total"]).To(BeNumerically("==", 5))
}

// Mutation-style outputs (outputs.x = ...) also picked up
func TestExecute_MutationStyleOutputs(t *testing.T) {
	RegisterTestingT(t)

	conns := []*core.Connection{
		stringInput("code", "outputs.greeting = 'hello ' + inputs.name;"),
		objectInput("inputs_data", map[string]interface{}{"name": "Andy"}),
		kvInput("outputs", [2]string{"greeting", "string"}),
	}
	out, err := Execute(&core.Flow{}, nil, conns)
	Expect(err).To(BeNil())
	Expect(out["greeting"]).To(Equal("hello Andy"))
}

// No declared outputs → whole return wrapped in "result"
func TestExecute_NoDeclaredOutputs_WrappedInResult(t *testing.T) {
	RegisterTestingT(t)

	conns := []*core.Connection{
		stringInput("code", "return { x: 1, y: 2 };"),
	}
	out, err := Execute(&core.Flow{}, nil, conns)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	res, ok := out["result"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(res["x"]).To(BeNumerically("==", 1))
}

// console.log is captured in the logs output
func TestExecute_ConsoleLog_Captured(t *testing.T) {
	RegisterTestingT(t)

	conns := []*core.Connection{
		stringInput("code", "console.log('hi from script'); return {};"),
	}
	out, err := Execute(&core.Flow{}, nil, conns)
	Expect(err).To(BeNil())
	Expect(out["logs"]).To(ContainSubstring("hi from script"))
}

// Runtime exception in the script surfaces as a clean failure
func TestExecute_ScriptThrows_SurfacedAsFailure(t *testing.T) {
	RegisterTestingT(t)

	conns := []*core.Connection{
		stringInput("code", "throw new Error('boom');"),
	}
	out, err := Execute(&core.Flow{}, nil, conns)
	Expect(err).NotTo(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("boom"))
}

// Infinite loop is killed by the timeout
func TestExecute_InfiniteLoop_KilledByTimeout(t *testing.T) {
	RegisterTestingT(t)

	conns := []*core.Connection{
		stringInput("code", "while(true){}"),
		&core.Connection{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Value: int64(1)},
	}
	out, err := Execute(&core.Flow{}, nil, conns)
	Expect(err).NotTo(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("timeout"))
}

// Literal secret in the script body is refused before execution
func TestExecute_LiteralSecretInCode_Refused(t *testing.T) {
	RegisterTestingT(t)

	conns := []*core.Connection{
		// Slack bot token shape — should match the detector. nosecret
		stringInput("code", "const TOKEN = 'xoxb-1234567890-1234567890-AbCdEfGhIjKlMnOpQrStUvWx'; return {};"), // nosecret
	}
	out, err := Execute(&core.Flow{}, nil, conns)
	Expect(err).NotTo(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("literal secret"))
}

// No network access — fetch isn't bound, so calling it raises
func TestExecute_NoNetworkAccess(t *testing.T) {
	RegisterTestingT(t)

	conns := []*core.Connection{
		stringInput("code", "return { ok: typeof fetch };"),
	}
	out, err := Execute(&core.Flow{}, nil, conns)
	Expect(err).To(BeNil())
	res, _ := out["result"].(map[string]interface{})
	Expect(res["ok"]).To(Equal("undefined"))
}
