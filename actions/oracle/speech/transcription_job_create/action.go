// Package oracle_speech_transcription_job_create starts an OCI Speech transcription job: it reads
// one audio file from an Object Storage bucket, transcribes it with the chosen language model, and
// writes the results to another Object Storage location. Asynchronous — the job comes back
// ACCEPTED; poll Get Transcription Job until it is SUCCEEDED before reading the output.
package oracle_speech_transcription_job_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	sp "flomation.app/automate/executor/actions/oracle/speech"

	"github.com/oracle/oci-go-sdk/v65/aispeech"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Speech: Create Transcription Job"
	Description  = "Start a transcription job that converts an audio file in Object Storage into text using the chosen language model, writing the results to an output bucket. Returns the job in an ACCEPTED state — poll Get Transcription Job until SUCCEEDED."
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
	{Name: "language_code", Type: core.ConnectionTypeString, Label: "Language Code", Placeholder: "Oracle model: en-US, es-ES, fr-FR… · Whisper model: en, es, fr…", Required: true},
	{Name: "model_type", Type: core.ConnectionTypeString, Label: "Model Type", Placeholder: "Which transcription model (default Oracle)", Options: []core.ConnectionOption{
		{Name: "Oracle (locale-specific codes, e.g. en-US)", Value: "ORACLE"},
		{Name: "Whisper Medium (locale-agnostic codes, e.g. en)", Value: "WHISPER_MEDIUM"},
		{Name: "Whisper Large v2 (on service request)", Value: "WHISPER_LARGE_V2"},
	}},
	{Name: "input_namespace", Type: core.ConnectionTypeString, Label: "Input Namespace", Placeholder: "Object Storage namespace holding the audio file", Required: true},
	{Name: "input_bucket", Type: core.ConnectionTypeString, Label: "Input Bucket", Placeholder: "Bucket holding the audio file", Required: true},
	{Name: "input_object", Type: core.ConnectionTypeString, Label: "Input Object", Placeholder: "Name of the audio file, e.g. recordings/call.wav", Required: true},
	{Name: "output_namespace", Type: core.ConnectionTypeString, Label: "Output Namespace", Placeholder: "Object Storage namespace for the results", Required: true},
	{Name: "output_bucket", Type: core.ConnectionTypeString, Label: "Output Bucket", Placeholder: "Bucket to write the transcription results into", Required: true},
	{Name: "output_prefix", Type: core.ConnectionTypeString, Label: "Output Prefix", Placeholder: "Folder/prefix for the result files, e.g. transcripts/", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "transcription_job", Type: core.ConnectionTypeObject, Label: "Transcription Job"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Transcription Job OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
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
	languageCode, err := sp.RequiredString("language_code", inputs)
	if err != nil {
		return sp.ErrorResult(err.Error()), nil
	}
	langEnum, ok := aispeech.GetMappingTranscriptionModelDetailsLanguageCodeEnum(languageCode)
	if !ok {
		return sp.ErrorResult(fmt.Sprintf("language code %q is not supported — use an Oracle code (e.g. en-US, es-ES, en-GB, fr-FR) or a Whisper code (e.g. en, es, fr)", languageCode)), nil
	}
	// Whisper language codes (bare e.g. "en") only work on a Whisper model; Oracle codes (locale
	// e.g. "en-US") only on the Oracle model. Default to Oracle; let the operator pick Whisper.
	modelType := sp.OptionalString("model_type", inputs)
	if modelType == "" {
		modelType = "ORACLE"
	}

	inNamespace, err := sp.RequiredString("input_namespace", inputs)
	if err != nil {
		return sp.ErrorResult(err.Error()), nil
	}
	inBucket, err := sp.RequiredString("input_bucket", inputs)
	if err != nil {
		return sp.ErrorResult(err.Error()), nil
	}
	inObject, err := sp.RequiredString("input_object", inputs)
	if err != nil {
		return sp.ErrorResult(err.Error()), nil
	}
	outNamespace, err := sp.RequiredString("output_namespace", inputs)
	if err != nil {
		return sp.ErrorResult(err.Error()), nil
	}
	outBucket, err := sp.RequiredString("output_bucket", inputs)
	if err != nil {
		return sp.ErrorResult(err.Error()), nil
	}
	outPrefix, err := sp.RequiredString("output_prefix", inputs)
	if err != nil {
		return sp.ErrorResult(err.Error()), nil
	}

	details := aispeech.CreateTranscriptionJobDetails{
		CompartmentId: &compartment,
		ModelDetails:  &aispeech.TranscriptionModelDetails{ModelType: &modelType, LanguageCode: langEnum},
		InputLocation: aispeech.ObjectListInlineInputLocation{
			ObjectLocations: []aispeech.ObjectLocation{{
				NamespaceName: &inNamespace,
				BucketName:    &inBucket,
				ObjectNames:   []string{inObject},
			}},
		},
		OutputLocation: &aispeech.OutputLocation{
			NamespaceName: &outNamespace,
			BucketName:    &outBucket,
			Prefix:        &outPrefix,
		},
	}

	resp, err := client.CreateTranscriptionJob(sp.Context(), aispeech.CreateTranscriptionJobRequest{CreateTranscriptionJobDetails: details})
	if err != nil {
		return sp.ErrorResult(auth.OCIError(err)), nil
	}
	job := sp.SummariseTranscriptionJob(&resp.TranscriptionJob)
	return sp.Result(fmt.Sprintf("Started transcription job %v (%v) — poll Get Transcription Job until SUCCEEDED", job["id"], job["lifecycle_state"]), map[string]interface{}{
		"transcription_job": job,
		"id":                job["id"],
		"lifecycle_state":   job["lifecycle_state"],
	}), nil
}
