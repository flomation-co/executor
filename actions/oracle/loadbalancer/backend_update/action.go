// Package oracle_loadbalancer_backend_update updates a backend server (weight,
// backup, drain, offline) within a backend set. Asynchronous — returns a
// work-request id.
package oracle_loadbalancer_backend_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	lbn "flomation.app/automate/executor/actions/oracle/loadbalancer"

	lb "github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Load Balancer: Update Backend"
	Description  = "Update a backend server (weight, backup, drain, offline) in a backend set of an Oracle Cloud load balancer. Asynchronous — returns a work-request id."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+server"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the load balancer picker)"},
	{Name: "load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Load Balancer OCID", Placeholder: "ocid1.loadbalancer.oc1..aaaa…", Required: true},
	{Name: "backend_set_name", Type: core.ConnectionTypeString, Label: "Backend Set Name", Placeholder: "The backend set the server belongs to, e.g. web-servers", Required: true},
	{Name: "backend_name", Type: core.ConnectionTypeString, Label: "Backend Name", Placeholder: "The backend to update, as IP:port, e.g. 10.0.1.5:8080", Required: true},
	{Name: "weight", Type: core.ConnectionTypeString, Label: "Weight", Placeholder: "Relative traffic weight, e.g. 1 or 3 — blank keeps the current value"},
	{Name: "backup", Type: core.ConnectionTypeBoolean, Label: "Backup", Placeholder: "Only receive traffic when all non-backup backends are unhealthy"},
	{Name: "drain", Type: core.ConnectionTypeBoolean, Label: "Drain", Placeholder: "Stop sending NEW connections (drain existing)"},
	{Name: "offline", Type: core.ConnectionTypeBoolean, Label: "Offline", Placeholder: "Take the backend out of rotation entirely"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, lbID, errResult := lbn.ResourceClient(inputs, "load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	bsName, err := lbn.RequiredString("backend_set_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	backendName, err := lbn.RequiredString("backend_name", inputs)
	if err != nil {
		return lbn.ErrorResult(err.Error()), nil
	}
	// Replace-semantics: UpdateBackendDetails overwrites the whole backend and the SDK
	// marks weight/backup/drain/offline all mandatory, so we must send all four. Read
	// the backend first and seed them from its current values, then overlay only what
	// the operator supplied — otherwise changing e.g. the weight would silently un-mark
	// a backup/drained/offline backend.
	cur, err := client.GetBackend(lbn.Context(), lb.GetBackendRequest{LoadBalancerId: &lbID, BackendSetName: &bsName, BackendName: &backendName})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	details := lb.UpdateBackendDetails{
		Weight:  cur.Backend.Weight,
		Backup:  cur.Backend.Backup,
		Drain:   cur.Backend.Drain,
		Offline: cur.Backend.Offline,
	}
	if v, ok, err := lbn.OptionalInt("weight", inputs); err != nil {
		return lbn.ErrorResult(err.Error()), nil
	} else if ok {
		details.Weight = &v
	}
	if lbn.BoolWasSet("backup", inputs) {
		b := lbn.OptionalBool("backup", inputs, false)
		details.Backup = &b
	}
	if lbn.BoolWasSet("drain", inputs) {
		b := lbn.OptionalBool("drain", inputs, false)
		details.Drain = &b
	}
	if lbn.BoolWasSet("offline", inputs) {
		b := lbn.OptionalBool("offline", inputs, false)
		details.Offline = &b
	}
	resp, err := client.UpdateBackend(lbn.Context(), lb.UpdateBackendRequest{
		LoadBalancerId:       &lbID,
		BackendSetName:       &bsName,
		BackendName:          &backendName,
		UpdateBackendDetails: details,
	})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updating backend %q in backend set %q — poll work request %s", backendName, bsName, lbn.Str(resp.OpcWorkRequestId)),
		"work_request_id": lbn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
