// Package oracle_dataflow_statement_create submits an interactive statement (a block of Spark
// code) to a running Data Flow SESSION run and returns the created statement.
package oracle_dataflow_statement_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	df "flomation.app/automate/executor/actions/oracle/dataflow"

	"github.com/oracle/oci-go-sdk/v65/dataflow"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Flow: Submit Statement"
	Description  = "Submit an interactive statement (a block of Spark code) to a running Data Flow SESSION run and return the created statement."
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
	{Name: "run_ocid", Type: core.ConnectionTypeString, Label: "Run OCID", Placeholder: "ocid1.dataflowrun.oc1..aaaa… (a SESSION run)", Required: true},
	{Name: "code", Type: core.ConnectionTypeText, Label: "Statement Code", Placeholder: "e.g. println(sc.version)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "statement", Type: core.ConnectionTypeObject, Label: "Statement"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Statement ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := df.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	runID, err := df.RequiredString("run_ocid", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	code, err := df.RequiredString("code", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}

	resp, err := client.CreateStatement(df.Context(), dataflow.CreateStatementRequest{
		CreateStatementDetails: dataflow.CreateStatementDetails{Code: &code},
		RunId:                  &runID,
	})
	if err != nil {
		return df.ErrorResult(auth.OCIError(err)), nil
	}
	statement := df.SummariseStatement(&resp.Statement)
	return df.Result(fmt.Sprintf("Submitted statement %v to run %s (%s)", statement["id"], statement["run_id"], statement["lifecycle_state"]), map[string]interface{}{
		"statement": statement, "id": statement["id"],
	}), nil
}
