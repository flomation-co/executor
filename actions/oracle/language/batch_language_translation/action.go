// Package oracle_language_batch_language_translation translates a single piece of text into a
// target language using OCI Language's pre-trained neural machine-translation model, returning the
// translated text and the auto-detected source language.
package oracle_language_batch_language_translation

import (
	"fmt"

	core "flomation.app/automate/executor"
	lang "flomation.app/automate/executor/actions/oracle/language"

	"github.com/oracle/oci-go-sdk/v65/ailanguage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Language: Translate Text"
	Description  = "Translate a piece of text into a target language using OCI Language's pre-trained neural machine-translation model."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+comments"
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
	{Name: "text", Type: core.ConnectionTypeText, Label: "Text", Placeholder: "The text to translate", Required: true},
	{Name: "target_language_code", Type: core.ConnectionTypeString, Label: "Target Language Code", Placeholder: "e.g. es, fr, de, ja, zh-CN", Required: true},
	{Name: "source_language_code", Type: core.ConnectionTypeString, Label: "Source Language Code", Placeholder: "Leave blank to auto-detect (e.g. en, fr, de)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "translations", Type: core.ConnectionTypeObject, Label: "Translations"},
	{Name: "translated_text", Type: core.ConnectionTypeString, Label: "Translated Text"},
	{Name: "source_language_code", Type: core.ConnectionTypeString, Label: "Detected Source Language"},
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
	target, err := lang.RequiredString("target_language_code", inputs)
	if err != nil {
		return lang.ErrorResult(err.Error()), nil
	}

	key := "1"
	doc := ailanguage.TextDocument{Key: &key, Text: &text}
	if src := lang.OptionalString("source_language_code", inputs); src != "" {
		doc.LanguageCode = &src
	}

	details := ailanguage.BatchLanguageTranslationDetails{
		Documents:          []ailanguage.TextDocument{doc},
		TargetLanguageCode: &target,
		CompartmentId:      &compartment,
	}

	resp, err := client.BatchLanguageTranslation(lang.Context(), ailanguage.BatchLanguageTranslationRequest{BatchLanguageTranslationDetails: details})
	if err != nil {
		return lang.ErrorResult(auth.OCIError(err)), nil
	}

	translations := make([]map[string]interface{}, 0, len(resp.Documents))
	for i := range resp.Documents {
		d := resp.Documents[i]
		translations = append(translations, map[string]interface{}{
			"key":                  lang.Str(d.Key),
			"translated_text":      lang.Str(d.TranslatedText),
			"source_language_code": lang.Str(d.SourceLanguageCode),
			"target_language_code": lang.Str(d.TargetLanguageCode),
		})
	}

	var translatedText, detectedSource string
	if len(translations) > 0 {
		translatedText, _ = translations[0]["translated_text"].(string)
		detectedSource, _ = translations[0]["source_language_code"].(string)
	}

	return lang.Result(fmt.Sprintf("Translated text from %q to %q", detectedSource, target), map[string]interface{}{
		"translations":         translations,
		"translated_text":      translatedText,
		"source_language_code": detectedSource,
	}), nil
}
