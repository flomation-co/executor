// Package oracle_dataflow_application_update applies a partial update to a Data Flow application:
// only the display name and number of executors you supply are changed; blank fields are left as-is.
package oracle_dataflow_application_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	df "flomation.app/automate/executor/actions/oracle/dataflow"

	"github.com/oracle/oci-go-sdk/v65/dataflow"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Flow: Update Application"
	Description  = "Partially update a Data Flow application — change only the display name or number of executors you supply; blank fields are left unchanged."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+diagram-project"
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
	{Name: "application_ocid", Type: core.ConnectionTypeString, Label: "Application OCID", Placeholder: "ocid1.dataflowapplication.oc1..aaaa… — the application to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New name (leave blank to keep unchanged)"},
	{Name: "num_executors", Type: core.ConnectionTypeString, Label: "Number of Executors", Placeholder: "New executor VM count (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "application", Type: core.ConnectionTypeObject, Label: "Application"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Application OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := df.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	appID, err := df.RequiredString("application_ocid", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the fields the operator actually supplied.
	details := dataflow.UpdateApplicationDetails{}
	if v := df.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	n, ok, err := df.OptionalInt("num_executors", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	if ok {
		details.NumExecutors = &n
	}

	resp, err := client.UpdateApplication(df.Context(), dataflow.UpdateApplicationRequest{
		ApplicationId:            &appID,
		UpdateApplicationDetails: details,
	})
	if err != nil {
		return df.ErrorResult(auth.OCIError(err)), nil
	}
	application := df.SummariseApplication(&resp.Application)
	return df.Result(fmt.Sprintf("Updated application %q (%s)", application["display_name"], application["lifecycle_state"]), map[string]interface{}{
		"application": application,
		"id":          application["id"],
	}), nil
}
