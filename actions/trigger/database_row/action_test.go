package database_row

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// The executor half of a poll trigger is a pure echo: whatever Launch injects
// when it fires the flow (the row columns + metadata) must flow straight
// through as node outputs, and nil-valued config inputs must be dropped.
func TestExecuteEchoesInjectedRowData(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		// Row columns injected by Launch when a new row is detected.
		{Name: "id", Type: core.ConnectionTypeString, Value: "42"},
		{Name: "email", Type: core.ConnectionTypeString, Value: "ada@example.com"},
		{Name: "row", Type: core.ConnectionTypeObject, Value: map[string]interface{}{"id": "42", "email": "ada@example.com"}},
		{Name: "cursor", Type: core.ConnectionTypeString, Value: "42"},
		{Name: "table", Type: core.ConnectionTypeString, Value: "orders"},
		{Name: "triggered_at", Type: core.ConnectionTypeString, Value: "2026-07-16T10:00:00Z"},
		// A config input left unset (nil) must not appear in the output.
		{Name: "password", Type: core.ConnectionTypeSecret},
	}

	out, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())

	Expect(out["id"]).To(Equal("42"))
	Expect(out["email"]).To(Equal("ada@example.com"))
	Expect(out["cursor"]).To(Equal("42"))
	Expect(out["table"]).To(Equal("orders"))
	Expect(out["triggered_at"]).To(Equal("2026-07-16T10:00:00Z"))
	Expect(out["row"]).To(HaveKeyWithValue("email", "ada@example.com"))

	// Unset input is dropped.
	Expect(out).To(Not(HaveKey("password")))
}

// Guard the manifest contract: this is a trigger node and its declared outputs
// are the ones the editor wires downstream / Launch populates.
func TestMetadataContract(t *testing.T) {
	RegisterTestingT(t)

	Expect(Type).To(Equal(core.ActionTypeTrigger))

	outputNames := map[string]bool{}
	for _, o := range Outputs {
		outputNames[o.Name] = true
	}
	Expect(outputNames).To(HaveKey("row"))
	Expect(outputNames).To(HaveKey("cursor"))
	Expect(outputNames).To(HaveKey("table"))
	Expect(outputNames).To(HaveKey("triggered_at"))

	// The cursor column is the heart of the detection strategy — it must be a
	// required input alongside the connection essentials.
	required := map[string]bool{}
	for _, i := range Inputs {
		if i.Required {
			required[i.Name] = true
		}
	}
	for _, name := range []string{"dialect", "host", "port", "username", "password", "database", "table", "cursor_column"} {
		Expect(required).To(HaveKey(name))
	}
}
