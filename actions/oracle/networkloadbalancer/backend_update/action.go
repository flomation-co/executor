// Package oracle_networkloadbalancer_backend_update changes a backend server's
// traffic settings (weight, backup, drain, offline) within a backend set. The NLB
// update is a full-replace PUT, so the action reads the backend first and seeds every
// field from its current value, overlaying only the settings the operator supplied —
// leaving a blank input keeps the existing value rather than nulling it. Asynchronous.
package oracle_networkloadbalancer_backend_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	nlbn "flomation.app/automate/executor/actions/oracle/networkloadbalancer"

	nlb "github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Network Load Balancer: Update Backend"
	Description  = "Update a backend server's traffic settings (weight, backup, drain, offline) in a backend set of an Oracle Cloud network load balancer. Settings you leave blank keep their current value. Asynchronous — returns a work-request id."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+server"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the network load balancer picker)"},
	{Name: "network_load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Network Load Balancer OCID", Placeholder: "ocid1.networkloadbalancer.oc1..aaaa…", Required: true},
	{Name: "backend_set_name", Type: core.ConnectionTypeString, Label: "Backend Set Name", Placeholder: "The backend set the server belongs to, e.g. app-servers", Required: true},
	{Name: "backend_name", Type: core.ConnectionTypeString, Label: "Backend Name", Placeholder: "The backend to update, e.g. 10.0.1.5:8080 (or its explicit name)", Required: true},
	{Name: "weight", Type: core.ConnectionTypeString, Label: "Weight", Placeholder: "Relative traffic weight, e.g. 3 (blank = keep current)"},
	{Name: "is_backup", Type: core.ConnectionTypeBoolean, Label: "Backup", Placeholder: "Only receive traffic when all non-backup backends are unhealthy (blank = keep current)"},
	{Name: "is_drain", Type: core.ConnectionTypeBoolean, Label: "Drain", Placeholder: "Stop sending NEW connections, drain existing (blank = keep current)"},
	{Name: "is_offline", Type: core.ConnectionTypeBoolean, Label: "Offline", Placeholder: "Take the backend out of rotation entirely (blank = keep current)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, nlbID, errResult := nlbn.ResourceClient(inputs, "network_load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	bsName, err := nlbn.RequiredString("backend_set_name", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}
	backendName, err := nlbn.RequiredString("backend_name", inputs)
	if err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	}

	// Full-replace PUT: read the backend and seed every mutable field from its current
	// value so an unsupplied setting is preserved rather than nulled.
	cur, err := client.GetBackend(nlbn.Context(), nlb.GetBackendRequest{
		NetworkLoadBalancerId: &nlbID,
		BackendSetName:        &bsName,
		BackendName:           &backendName,
	})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}
	details := nlb.UpdateBackendDetails{
		Weight:    cur.Backend.Weight,
		IsBackup:  cur.Backend.IsBackup,
		IsDrain:   cur.Backend.IsDrain,
		IsOffline: cur.Backend.IsOffline,
	}

	// Overlay only the fields the operator supplied.
	if v, ok, err := nlbn.OptionalInt("weight", inputs); err != nil {
		return nlbn.ErrorResult(err.Error()), nil
	} else if ok {
		details.Weight = &v
	}
	for _, f := range []struct {
		name string
		set  func(*bool)
	}{
		{"is_backup", func(b *bool) { details.IsBackup = b }},
		{"is_drain", func(b *bool) { details.IsDrain = b }},
		{"is_offline", func(b *bool) { details.IsOffline = b }},
	} {
		if nlbn.BoolWasSet(f.name, inputs) {
			b := nlbn.OptionalBool(f.name, inputs, false)
			f.set(&b)
		}
	}

	resp, err := client.UpdateBackend(nlbn.Context(), nlb.UpdateBackendRequest{
		NetworkLoadBalancerId: &nlbID,
		BackendSetName:        &bsName,
		BackendName:           &backendName,
		UpdateBackendDetails:  details,
	})
	if err != nil {
		return nlbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updating backend %q in backend set %q — poll work request %s", backendName, bsName, nlbn.Str(resp.OpcWorkRequestId)),
		"work_request_id": nlbn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
