// Package oracle_speech_list_voices lists the text-to-speech voices available for synthesis in a
// compartment, optionally filtered by TTS model, language code, or display name.
package oracle_speech_list_voices

import (
	"fmt"

	core "flomation.app/automate/executor"
	sp "flomation.app/automate/executor/actions/oracle/speech"

	"github.com/oracle/oci-go-sdk/v65/aispeech"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Speech: List Voices"
	Description  = "List the text-to-speech voices available for synthesis. Optionally filter by TTS model, language code, or display name."
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
	{Name: "model_name", Type: core.ConnectionTypeString, Label: "Model Filter", Placeholder: "Only voices for this TTS model (optional)", Options: []core.ConnectionOption{
		{Name: "TTS Standard (TTS_1_STANDARD)", Value: "TTS_1_STANDARD"},
		{Name: "TTS Natural (TTS_2_NATURAL)", Value: "TTS_2_NATURAL"},
	}},
	{Name: "language_code", Type: core.ConnectionTypeString, Label: "Language Code Filter", Placeholder: "e.g. en-US (optional)"},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only the voice with this exact display name (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "voices", Type: core.ConnectionTypeObject, Label: "Voices"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
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
	req := aispeech.ListVoicesRequest{CompartmentId: &compartment}
	if m := sp.OptionalString("model_name", inputs); m != "" {
		req.ModelName = aispeech.TtsOracleModelDetailsModelNameEnum(m)
	}
	if lc := sp.OptionalString("language_code", inputs); lc != "" {
		req.LanguageCode = &lc
	}
	if dn := sp.OptionalString("display_name", inputs); dn != "" {
		req.DisplayName = &dn
	}

	resp, err := client.ListVoices(sp.Context(), req)
	if err != nil {
		return sp.ErrorResult(auth.OCIError(err)), nil
	}

	out := make([]map[string]interface{}, 0, len(resp.Items))
	for i := range resp.Items {
		v := &resp.Items[i]
		out = append(out, map[string]interface{}{
			"voice_id":             sp.Str(v.VoiceId),
			"display_name":         sp.Str(v.DisplayName),
			"gender":               string(v.Gender),
			"sample_rate_in_hertz": sp.IntOrNil(v.SampleRateInHertz),
			"words_per_minute":     sp.IntOrNil(v.WordsPerMinute),
			"description":          sp.Str(v.Description),
			"supported_models":     v.SupportedModels,
			"language_code":        sp.Str(v.LanguageCode),
			"language_description": sp.Str(v.LanguageDescription),
			"is_default_voice":     sp.Bool(v.IsDefaultVoice),
		})
	}

	return sp.Result(fmt.Sprintf("Found %d voice(s)", len(out)), map[string]interface{}{
		"voices": out, "count": fmt.Sprintf("%d", len(out)),
	}), nil
}
