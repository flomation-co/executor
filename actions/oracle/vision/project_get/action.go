// Package oracle_vision_project_get fetches a single OCI Vision project by its OCID, returning its
// display name, description, compartment, lifecycle state and timestamps.
package oracle_vision_project_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	vis "flomation.app/automate/executor/actions/oracle/vision"

	"github.com/oracle/oci-go-sdk/v65/aivision"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vision: Get Project"
	Description  = "Fetch a single Vision project by its OCID — its display name, description, compartment and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+eye"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "project_ocid", Type: core.ConnectionTypeString, Label: "Project OCID", Placeholder: "ocid1.aivisionproject.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "project", Type: core.ConnectionTypeObject, Label: "Project"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Project OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := vis.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	projectID, err := vis.RequiredString("project_ocid", inputs)
	if err != nil {
		return vis.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetProject(vis.Context(), aivision.GetProjectRequest{ProjectId: &projectID})
	if err != nil {
		return vis.ErrorResult(auth.OCIError(err)), nil
	}
	project := vis.SummariseProject(&resp.Project)
	return vis.Result(fmt.Sprintf("Project %q (%s)", project["display_name"], project["lifecycle_state"]), map[string]interface{}{
		"project": project, "id": project["id"], "lifecycle_state": project["lifecycle_state"],
	}), nil
}
