// Package oracle_speech_transcription_job_get fetches a single OCI Speech transcription job by its
// OCID, returning its lifecycle state and how far the transcription has progressed.
package oracle_speech_transcription_job_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	sp "flomation.app/automate/executor/actions/oracle/speech"

	"github.com/oracle/oci-go-sdk/v65/aispeech"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Speech: Get Transcription Job"
	Description  = "Fetch a single Speech transcription job by its OCID — its lifecycle state and percent complete."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+microphone"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "transcription_job_ocid", Type: core.ConnectionTypeString, Label: "Transcription Job OCID", Placeholder: "ocid1.aispeechtranscriptionjob.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "transcription_job", Type: core.ConnectionTypeObject, Label: "Transcription Job"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Transcription Job OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "percent_complete", Type: core.ConnectionTypeInteger, Label: "Percent Complete"},
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

	resp, err := client.GetTranscriptionJob(sp.Context(), aispeech.GetTranscriptionJobRequest{TranscriptionJobId: &jobID})
	if err != nil {
		return sp.ErrorResult(auth.OCIError(err)), nil
	}
	job := sp.SummariseTranscriptionJob(&resp.TranscriptionJob)
	return sp.Result(fmt.Sprintf("Transcription job %q (%s)", job["display_name"], job["lifecycle_state"]), map[string]interface{}{
		"transcription_job": job,
		"id":                job["id"],
		"lifecycle_state":   job["lifecycle_state"],
		"percent_complete":  job["percent_complete"],
	}), nil
}
