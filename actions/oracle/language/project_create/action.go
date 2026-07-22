// Package oracle_language_project_create creates a Language project — the container that groups the
// custom models and endpoints you train and serve. Asynchronous: the project comes back CREATING
// with a work-request id; poll Get Project until it is ACTIVE before adding models.
package oracle_language_project_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	lang "flomation.app/automate/executor/actions/oracle/language"

	"github.com/oracle/oci-go-sdk/v65/ailanguage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Language: Create Project"
	Description  = "Create a Language project to group your custom models and endpoints. Returns the project in a CREATING state plus a work-request id — poll Get Project until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+comments"
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
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What this project is for (optional)"},
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
	auth, client, errResult := lang.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return lang.ErrorResult(err.Error()), nil
	}

	details := ailanguage.CreateProjectDetails{CompartmentId: &compartment}
	if name := lang.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}
	if desc := lang.OptionalString("description", inputs); desc != "" {
		details.Description = &desc
	}

	resp, err := client.CreateProject(lang.Context(), ailanguage.CreateProjectRequest{CreateProjectDetails: details})
	if err != nil {
		return lang.ErrorResult(auth.OCIError(err)), nil
	}

	project := lang.SummariseProject(&resp.Project)
	return lang.Result(fmt.Sprintf("Creating project %q — poll Get Project until ACTIVE", lang.Str(resp.DisplayName)), map[string]interface{}{
		"project":         project,
		"id":              project["id"],
		"lifecycle_state": project["lifecycle_state"],
		"work_request_id": lang.Str(resp.OpcWorkRequestId),
	}), nil
}
