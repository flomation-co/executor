// Package oracle_exadata_db_server_get reads one Exadata infrastructure DB server by
// OCID — its shape, CPU/memory capacity and lifecycle state.
package oracle_exadata_db_server_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: Get DB Server"
	Description  = "Fetch a single Exadata infrastructure DB server by OCID — its shape, CPU/memory capacity and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+microchip"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "cloud_exadata_infrastructure_ocid", Type: core.ConnectionTypeString, Label: "Cloud Exadata Infrastructure OCID", Placeholder: "ocid1.cloudexadatainfrastructure.oc1..aaaa…", Required: true},
	{Name: "db_server_ocid", Type: core.ConnectionTypeString, Label: "DB Server OCID", Placeholder: "ocid1.dbserver.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "db_server", Type: core.ConnectionTypeObject, Label: "DB Server"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "DB Server OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := exa.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	infraID, err := exa.RequiredString("cloud_exadata_infrastructure_ocid", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	dbServerID, err := exa.RequiredString("db_server_ocid", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetDbServer(exa.Context(), db.GetDbServerRequest{ExadataInfrastructureId: &infraID, DbServerId: &dbServerID})
	if err != nil {
		return exa.ErrorResult(auth.OCIError(err)), nil
	}
	server := exa.SummariseDbServer(&resp.DbServer)
	return exa.Result(fmt.Sprintf("DB server %q is %s", server["display_name"], server["lifecycle_state"]), map[string]interface{}{
		"db_server": server, "id": server["id"], "lifecycle_state": server["lifecycle_state"],
	}), nil
}
