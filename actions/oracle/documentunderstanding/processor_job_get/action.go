// Package oracle_documentunderstanding_processor_job_get fetches a single Document Understanding
// processor job by OCID, returning its lifecycle state and processing progress.
package oracle_documentunderstanding_processor_job_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	du "flomation.app/automate/executor/actions/oracle/documentunderstanding"

	"github.com/oracle/oci-go-sdk/v65/aidocument"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Document Understanding: Get Processor Job"
	Description  = "Fetch a single Document Understanding processor job by its OCID — its lifecycle state and processing progress."
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
	{Name: "processor_job_ocid", Type: core.ConnectionTypeString, Label: "Processor Job OCID", Placeholder: "ocid1.aidocumentprocessorjob.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "processor_job", Type: core.ConnectionTypeObject, Label: "Processor Job"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Processor Job OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "percent_complete", Type: core.ConnectionTypeString, Label: "Percent Complete"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := du.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	jobID, err := du.RequiredString("processor_job_ocid", inputs)
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetProcessorJob(du.Context(), aidocument.GetProcessorJobRequest{ProcessorJobId: &jobID})
	if err != nil {
		return du.ErrorResult(auth.OCIError(err)), nil
	}
	job := du.SummariseProcessorJob(&resp.ProcessorJob)
	return du.Result(fmt.Sprintf("Processor job %q (%s)", job["id"], job["lifecycle_state"]), map[string]interface{}{
		"processor_job":    job,
		"id":               job["id"],
		"lifecycle_state":  job["lifecycle_state"],
		"percent_complete": job["percent_complete"],
	}), nil
}
