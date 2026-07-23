// Package oracle_language_batch_detect_language_key_phrases extracts the key phrases from a single
// piece of text using OCI Language's pre-trained key-phrase model.
package oracle_language_batch_detect_language_key_phrases

import (
	"fmt"

	core "flomation.app/automate/executor"
	lang "flomation.app/automate/executor/actions/oracle/language"

	"github.com/oracle/oci-go-sdk/v65/ailanguage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Language: Detect Key Phrases"
	Description  = "Extract the key phrases and their confidence scores from a piece of text with OCI Language's pre-trained model."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+comments"
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
	{Name: "text", Type: core.ConnectionTypeString, Label: "Text", Placeholder: "The text to extract key phrases from", Required: true},
	{Name: "language_code", Type: core.ConnectionTypeString, Label: "Language Code", Placeholder: "e.g. en, es, fr — leave blank to auto-detect (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Key Phrases"},
	{Name: "language_code", Type: core.ConnectionTypeString, Label: "Detected Language Code"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Key Phrase Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := lang.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return lang.ErrorResult(err.Error()), nil
	}
	text, err := lang.RequiredString("text", inputs)
	if err != nil {
		return lang.ErrorResult(err.Error()), nil
	}

	key := "doc1"
	doc := ailanguage.TextDocument{Key: &key, Text: &text}
	if langCode := lang.OptionalString("language_code", inputs); langCode != "" {
		doc.LanguageCode = &langCode
	}

	details := ailanguage.BatchDetectLanguageKeyPhrasesDetails{
		Documents:     []ailanguage.TextDocument{doc},
		CompartmentId: &compartment,
	}
	resp, err := client.BatchDetectLanguageKeyPhrases(lang.Context(), ailanguage.BatchDetectLanguageKeyPhrasesRequest{
		BatchDetectLanguageKeyPhrasesDetails: details,
	})
	if err != nil {
		return lang.ErrorResult(auth.OCIError(err)), nil
	}

	if len(resp.Documents) == 0 {
		if len(resp.Errors) > 0 && resp.Errors[0].Error != nil {
			return lang.ErrorResult(fmt.Sprintf("OCI Language could not process the text: %s", lang.Str(resp.Errors[0].Error.Message))), nil
		}
		return lang.ErrorResult("OCI Language returned no result for the text"), nil
	}

	d := resp.Documents[0]
	phrases := make([]map[string]interface{}, 0, len(d.KeyPhrases))
	for _, p := range d.KeyPhrases {
		phrases = append(phrases, map[string]interface{}{
			"text":  lang.Str(p.Text),
			"score": p.Score,
		})
	}
	detected := lang.Str(d.LanguageCode)

	return lang.Result(
		fmt.Sprintf("Extracted %d key phrase(s) (%s)", len(phrases), detected),
		map[string]interface{}{
			"results":       phrases,
			"language_code": detected,
			"count":         len(phrases),
		}), nil
}
