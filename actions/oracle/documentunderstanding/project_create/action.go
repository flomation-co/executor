// Package oracle_documentunderstanding_project_create creates a Document Understanding project — the
// container that holds custom models. Asynchronous: the project comes back CREATING with a
// work-request id; poll Get Project until it is ACTIVE before training models in it.
package oracle_documentunderstanding_project_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	du "flomation.app/automate/executor/actions/oracle/documentunderstanding"

	"github.com/oracle/oci-go-sdk/v65/aidocument"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Document Understanding: Create Project"
	Description  = "Create a Document Understanding project to hold custom models. Returns the project in a CREATING state plus a work-request id — poll Get Project until ACTIVE."
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
	auth, client, errResult := du.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}

	details := aidocument.CreateProjectDetails{CompartmentId: &compartment}
	if v := strings.TrimSpace(du.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	if v := strings.TrimSpace(du.OptionalString("description", inputs)); v != "" {
		details.Description = &v
	}

	resp, err := client.CreateProject(du.Context(), aidocument.CreateProjectRequest{CreateProjectDetails: details})
	if err != nil {
		return du.ErrorResult(auth.OCIError(err)), nil
	}
	return du.Result(fmt.Sprintf("Creating project %q — poll Get Project until ACTIVE", du.Str(resp.DisplayName)), map[string]interface{}{
		"project":         du.SummariseProject(&resp.Project),
		"id":              du.Str(resp.Id),
		"lifecycle_state": string(resp.LifecycleState),
		"work_request_id": du.Str(resp.OpcWorkRequestId),
	}), nil
}
