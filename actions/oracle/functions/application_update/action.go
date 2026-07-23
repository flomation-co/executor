// Package oracle_functions_application_update applies a partial update to an OCI Functions
// application: only the fields the operator supplies (config and/or free-form tags) are sent,
// leaving everything else untouched. It returns the updated application.
package oracle_functions_application_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	fn "flomation.app/automate/executor/actions/oracle/functions"

	"github.com/oracle/oci-go-sdk/v65/functions"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Functions: Update Application"
	Description  = "Partially update an OCI Functions application by OCID — set its configuration and/or free-form tags. Only the fields you supply are changed."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+code"
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
	{Name: "application_ocid", Type: core.ConnectionTypeString, Label: "Application OCID", Placeholder: "ocid1.fnapp.oc1..aaaa…", Required: true},
	{Name: "config", Type: core.ConnectionTypeText, Label: "Configuration", Placeholder: "JSON object of string env vars, e.g. {\"MY_KEY\":\"value\"} (optional)"},
	{Name: "freeform_tags", Type: core.ConnectionTypeText, Label: "Free-form Tags", Placeholder: "JSON object of string key/values, e.g. {\"Department\":\"Finance\"} (optional)"},
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
	applicationID, err := fn.RequiredString("application_ocid", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	auth, client, errResult := fn.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}

	config, err := fn.ConfigMap("config", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	tags, err := fn.FreeformTags("freeform_tags", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}

	details := functions.UpdateApplicationDetails{}
	if config != nil {
		details.Config = config
	}
	if tags != nil {
		details.FreeformTags = tags
	}

	resp, err := client.UpdateApplication(fn.Context(), functions.UpdateApplicationRequest{
		ApplicationId:            &applicationID,
		UpdateApplicationDetails: details,
	})
	if err != nil {
		return fn.ErrorResult(auth.OCIError(err)), nil
	}

	app := fn.SummariseApplication(&resp.Application)
	return fn.Result(fmt.Sprintf("Updated application %q", fn.Str(resp.DisplayName)), map[string]interface{}{
		"application":     app,
		"id":              fn.Str(resp.Id),
		"lifecycle_state": string(resp.LifecycleState),
	}), nil
}
