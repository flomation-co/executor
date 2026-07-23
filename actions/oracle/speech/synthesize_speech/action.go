// Package oracle_speech_synthesize_speech converts a piece of text into spoken audio using the
// OCI Speech text-to-speech (TTS) service and returns the generated audio as base64. The service
// default voice and audio format are used, so only the text is required.
package oracle_speech_synthesize_speech

import (
	"encoding/base64"
	"fmt"
	"io"

	core "flomation.app/automate/executor"
	sp "flomation.app/automate/executor/actions/oracle/speech"

	"github.com/oracle/oci-go-sdk/v65/aispeech"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Speech: Synthesize Speech"
	Description  = "Convert text into spoken audio with the OCI Speech text-to-speech service and return the generated audio as base64."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "text", Type: core.ConnectionTypeText, Label: "Text", Placeholder: "The text to convert into spoken audio", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "audio_base64", Type: core.ConnectionTypeString, Label: "Audio (base64)"},
	{Name: "byte_count", Type: core.ConnectionTypeInteger, Label: "Audio byte count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	text, err := sp.RequiredString("text", inputs)
	if err != nil {
		return sp.ErrorResult(err.Error()), nil
	}
	auth, client, errResult := sp.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}

	details := aispeech.SynthesizeSpeechDetails{Text: &text}
	if auth.CompartmentOCID != "" {
		compartment := auth.CompartmentOCID
		details.CompartmentId = &compartment
	}

	resp, err := client.SynthesizeSpeech(sp.Context(), aispeech.SynthesizeSpeechRequest{SynthesizeSpeechDetails: details})
	if err != nil {
		return sp.ErrorResult(auth.OCIError(err)), nil
	}

	var audio []byte
	if resp.Content != nil {
		b, readErr := io.ReadAll(resp.Content)
		_ = resp.Content.Close()
		if readErr != nil {
			return sp.ErrorResult(fmt.Sprintf("speech synthesized but the audio could not be read: %s", readErr.Error())), nil
		}
		audio = b
	}

	return sp.Result(fmt.Sprintf("Synthesized %d bytes of audio", len(audio)), map[string]interface{}{
		"audio_base64": base64.StdEncoding.EncodeToString(audio),
		"byte_count":   len(audio),
	}), nil
}
