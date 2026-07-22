// Package oracle_functions_application_create creates a new Functions application in a compartment.
// An application is the top-level container that groups functions and shares their subnets, config,
// and network settings. Provide one or more subnet OCIDs for the functions to run in.
package oracle_functions_application_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	fn "flomation.app/automate/executor/actions/oracle/functions"

	"github.com/oracle/oci-go-sdk/v65/functions"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Functions: Create Application"
	Description  = "Create a Functions application in a compartment, providing the subnets its functions run in and optional shared config."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+code"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where the application is created)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name unique within the compartment", Required: true},
	{Name: "subnet_ids", Type: core.ConnectionTypeString, Label: "Subnet OCIDs", Placeholder: "One or more subnet OCIDs, comma-separated", Required: true},
	{Name: "config", Type: core.ConnectionTypeText, Label: "Config", Placeholder: "Shared config as a JSON object, e.g. {\"MY_KEY\":\"value\"} (optional)"},
	{Name: "freeform_tags", Type: core.ConnectionTypeText, Label: "Freeform Tags", Placeholder: "Tags as a JSON object, e.g. {\"Department\":\"Finance\"} (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "application", Type: core.ConnectionTypeObject, Label: "Application"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Application OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := fn.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	displayName, err := fn.RequiredString("display_name", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	subnetsRaw, err := fn.RequiredString("subnet_ids", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	var subnetIDs []string
	for _, s := range strings.Split(subnetsRaw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			subnetIDs = append(subnetIDs, s)
		}
	}
	if len(subnetIDs) == 0 {
		return fn.ErrorResult("subnet ids is required (one or more subnet OCIDs, comma-separated)"), nil
	}
	config, err := fn.ConfigMap("config", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	tags, err := fn.FreeformTags("freeform_tags", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}

	details := functions.CreateApplicationDetails{
		CompartmentId: &compartment,
		DisplayName:   &displayName,
		SubnetIds:     subnetIDs,
	}
	if len(config) > 0 {
		details.Config = config
	}
	if len(tags) > 0 {
		details.FreeformTags = tags
	}

	resp, err := client.CreateApplication(fn.Context(), functions.CreateApplicationRequest{CreateApplicationDetails: details})
	if err != nil {
		return fn.ErrorResult(auth.OCIError(err)), nil
	}
	app := fn.SummariseApplication(&resp.Application)
	return fn.Result(fmt.Sprintf("Created application %q", displayName), map[string]interface{}{
		"application":     app,
		"id":              fn.Str(resp.Id),
		"lifecycle_state": string(resp.LifecycleState),
	}), nil
}
