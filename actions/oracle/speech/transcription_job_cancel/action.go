// Package oracle_speech_transcription_job_cancel cancels an in-progress OCI Speech transcription
// job by its OCID, stopping any outstanding transcription tasks.
package oracle_speech_transcription_job_cancel

import (
	"fmt"

	core "flomation.app/automate/executor"
	sp "flomation.app/automate/executor/actions/oracle/speech"

	"github.com/oracle/oci-go-sdk/v65/aispeech"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Speech: Cancel Transcription Job"
	Description  = "Cancel an in-progress Speech transcription job by its OCID — any outstanding transcription tasks are stopped."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+microphone"
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
	{Name: "transcription_job_ocid", Type: core.ConnectionTypeString, Label: "Transcription Job OCID", Placeholder: "ocid1.aispeechtranscriptionjob.oc1..aaaa… (the job to cancel)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Transcription Job OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := sp.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	jobID, err := sp.RequiredString("transcription_job_ocid", inputs)
	if err != nil {
		return sp.ErrorResult(err.Error()), nil
	}

	_, err = client.CancelTranscriptionJob(sp.Context(), aispeech.CancelTranscriptionJobRequest{
		TranscriptionJobId: &jobID,
	})
	if err != nil {
		return sp.ErrorResult(auth.OCIError(err)), nil
	}

	return sp.Result(fmt.Sprintf("Cancelled transcription job %s", jobID), map[string]interface{}{
		"id": jobID,
	}), nil
}
