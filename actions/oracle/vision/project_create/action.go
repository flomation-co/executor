// Package oracle_vision_project_create creates an OCI Vision project — the container that holds the
// custom models you train for image classification, object detection or document key-value/table
// extraction. Asynchronous: the project comes back CREATING with a work-request id; poll Get Project
// until it is ACTIVE before adding models.
package oracle_vision_project_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	vis "flomation.app/automate/executor/actions/oracle/vision"

	"github.com/oracle/oci-go-sdk/v65/aivision"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vision: Create Project"
	Description  = "Create a Vision project — the container for your custom image/document models. Returns the project in a CREATING state plus a work-request id; poll Get Project until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+eye"
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the project (optional)"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "An optional description of the project"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "project", Type: core.ConnectionTypeObject, Label: "Project"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Project OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := vis.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return vis.ErrorResult(err.Error()), nil
	}

	details := aivision.CreateProjectDetails{CompartmentId: &compartment}
	if name := vis.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}
	if desc := vis.OptionalString("description", inputs); desc != "" {
		details.Description = &desc
	}

	resp, err := client.CreateProject(vis.Context(), aivision.CreateProjectRequest{CreateProjectDetails: details})
	if err != nil {
		return vis.ErrorResult(auth.OCIError(err)), nil
	}
	project := vis.SummariseProject(&resp.Project)
	return vis.Result(fmt.Sprintf("Creating project %q (%s) — poll Get Project until ACTIVE", project["display_name"], project["lifecycle_state"]), map[string]interface{}{
		"project":         project,
		"id":              project["id"],
		"lifecycle_state": project["lifecycle_state"],
		"work_request_id": vis.Str(resp.OpcWorkRequestId),
	}), nil
}
