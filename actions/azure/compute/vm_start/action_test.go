package azure_compute_vm_start

import (
	"strings"
	"testing"

	core "flomation.app/automate/executor"
)

// creds returns a full, well-formed credential block. azidentity builds the
// credential offline, so these tests exercise Execute's input validation
// without any network call — a missing required field must be caught and
// returned as a soft failure BEFORE any ARM request is attempted.
func creds() []*core.Connection {
	return []*core.Connection{
		{Name: "azure_tenant_id", Type: core.ConnectionTypeString, Value: "00000000-0000-0000-0000-000000000000"},
		{Name: "azure_client_id", Type: core.ConnectionTypeString, Value: "11111111-1111-1111-1111-111111111111"},
		{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Value: "secret"},
		{Name: "subscription_id", Type: core.ConnectionTypeString, Value: "22222222-2222-2222-2222-222222222222"},
		{Name: "resource_group", Type: core.ConnectionTypeString, Value: "my-rg"},
		{Name: "vm_name", Type: core.ConnectionTypeString, Value: "my-vm"},
	}
}

func without(name string) []*core.Connection {
	in := creds()
	for _, c := range in {
		if c.Name == name {
			c.Value = ""
		}
	}
	return in
}

func TestExecuteValidatesInputs(t *testing.T) {
	for field, want := range map[string]string{
		"resource_group": "resource group",
		"vm_name":        "vm name",
	} {
		out, err := Execute(nil, nil, without(field))
		if err != nil {
			t.Errorf("Execute(missing %s) returned a hard error %v; want a soft failure", field, err)
			continue
		}
		if out["success"] != false {
			t.Errorf("Execute(missing %s) success = %v, want false", field, out["success"])
		}
		msg, _ := out["error"].(string)
		if !strings.Contains(msg, want) {
			t.Errorf("Execute(missing %s) error = %q, want it to mention %q", field, msg, want)
		}
	}
}
