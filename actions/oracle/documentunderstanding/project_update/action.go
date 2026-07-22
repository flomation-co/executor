// Package oracle_documentunderstanding_project_update applies a partial update to a Document
// Understanding project: only the display name and description you supply are changed; blank fields
// are left as-is. Asynchronous — the change returns a work-request id; poll Get Project until the
// update settles.
package oracle_documentunderstanding_project_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	du "flomation.app/automate/executor/actions/oracle/documentunderstanding"

	"github.com/oracle/oci-go-sdk/v65/aidocument"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Document Understanding: Update Project"
	Description  = "Partially update a Document Understanding project — change only the display name or description you supply; blank fields are left unchanged. Asynchronous: returns a work-request id, poll Get Project until it settles."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+image"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "project_ocid", Type: core.ConnectionTypeString, Label: "Project OCID", Placeholder: "ocid1.aidocumentproject.oc1..aaaa… — the project to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Project OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := du.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	projectID, err := du.RequiredString("project_ocid", inputs)
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the fields the operator actually supplied; blank fields are left as-is.
	details := aidocument.UpdateProjectDetails{}
	if v := du.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	if v := du.OptionalString("description", inputs); v != "" {
		details.Description = &v
	}

	resp, err := client.UpdateProject(du.Context(), aidocument.UpdateProjectRequest{
		ProjectId:            &projectID,
		UpdateProjectDetails: details,
	})
	if err != nil {
		return du.ErrorResult(auth.OCIError(err)), nil
	}
	return du.Result(fmt.Sprintf("Updating project %s — poll Get Project until it settles", projectID), map[string]interface{}{
		"id":              projectID,
		"work_request_id": du.Str(resp.OpcWorkRequestId),
	}), nil
}
