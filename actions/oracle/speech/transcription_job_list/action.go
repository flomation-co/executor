// Package oracle_speech_transcription_job_list lists the OCI Speech transcription jobs in a
// compartment, optionally filtered by display name or lifecycle state. Walks pagination up to a
// safe cap.
package oracle_speech_transcription_job_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	sp "flomation.app/automate/executor/actions/oracle/speech"

	"github.com/oracle/oci-go-sdk/v65/aispeech"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Speech: List Transcription Jobs"
	Description  = "List the audio transcription jobs in a compartment. Optionally filter by exact display name or lifecycle state, and cap the number returned. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only jobs with this exact name (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Only jobs in this state (optional)", Options: []core.ConnectionOption{
		{Name: "Accepted", Value: "ACCEPTED"},
		{Name: "In Progress", Value: "IN_PROGRESS"},
		{Name: "Succeeded", Value: "SUCCEEDED"},
		{Name: "Failed", Value: "FAILED"},
		{Name: "Canceling", Value: "CANCELING"},
		{Name: "Canceled", Value: "CANCELED"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max items per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "transcription_jobs", Type: core.ConnectionTypeObject, Label: "Transcription Jobs"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := sp.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return sp.ErrorResult(err.Error()), nil
	}
	req := aispeech.ListTranscriptionJobsRequest{CompartmentId: &compartment}
	if name := sp.OptionalString("display_name", inputs); name != "" {
		req.DisplayName = &name
	}
	if state := sp.OptionalString("lifecycle_state", inputs); state != "" {
		req.LifecycleState = aispeech.TranscriptionJobLifecycleStateEnum(state)
	}
	if limit, ok, err := sp.OptionalInt("limit", inputs); err != nil {
		return sp.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &limit
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= sp.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListTranscriptionJobs(sp.Context(), req)
		if err != nil {
			return sp.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, sp.SummariseTranscriptionJobSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return sp.Result(fmt.Sprintf("Found %d transcription job(s)", len(out)), map[string]interface{}{
		"transcription_jobs": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
