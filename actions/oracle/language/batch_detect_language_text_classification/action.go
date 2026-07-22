// Package oracle_language_batch_detect_language_text_classification classifies a single piece of
// text into one or more content classes using OCI Language's pre-trained text-classification model.
package oracle_language_batch_detect_language_text_classification

import (
	"fmt"

	core "flomation.app/automate/executor"
	lang "flomation.app/automate/executor/actions/oracle/language"

	"github.com/oracle/oci-go-sdk/v65/ailanguage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Language: Classify Text"
	Description  = "Classify a piece of text into content classes using OCI Language's pre-trained text-classification model."
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
	{Name: "text", Type: core.ConnectionTypeString, Label: "Text", Placeholder: "The text to classify", Required: true},
	{Name: "language_code", Type: core.ConnectionTypeString, Label: "Language Code", Placeholder: "Optional ISO code, e.g. en, es, fr, ja (default: auto-detect)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Classifications"},
	{Name: "language_code", Type: core.ConnectionTypeString, Label: "Detected Language Code"},
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

	key := "doc-1"
	doc := ailanguage.TextDocument{Key: &key, Text: &text}
	if languageCode := lang.OptionalString("language_code", inputs); languageCode != "" {
		doc.LanguageCode = &languageCode
	}

	resp, err := client.BatchDetectLanguageTextClassification(lang.Context(), ailanguage.BatchDetectLanguageTextClassificationRequest{
		BatchDetectLanguageTextClassificationDetails: ailanguage.BatchDetectLanguageTextClassificationDetails{
			CompartmentId: &compartment,
			Documents:     []ailanguage.TextDocument{doc},
		},
	})
	if err != nil {
		return lang.ErrorResult(auth.OCIError(err)), nil
	}

	if len(resp.Documents) == 0 {
		if len(resp.Errors) > 0 && resp.Errors[0].Error != nil {
			return lang.ErrorResult(fmt.Sprintf("OCI Language could not classify the text: %s", lang.Str(resp.Errors[0].Error.Message))), nil
		}
		return lang.ErrorResult("OCI Language returned no classification result for the text"), nil
	}

	docResult := resp.Documents[0]
	classifications := make([]map[string]interface{}, 0, len(docResult.TextClassification))
	for _, c := range docResult.TextClassification {
		entry := map[string]interface{}{"label": lang.Str(c.Label)}
		if c.Score != nil {
			entry["score"] = *c.Score
		}
		classifications = append(classifications, entry)
	}

	results := map[string]interface{}{
		"key":             lang.Str(docResult.Key),
		"language_code":   lang.Str(docResult.LanguageCode),
		"classifications": classifications,
	}

	topLabel := ""
	if len(classifications) > 0 {
		topLabel = fmt.Sprintf("%v", classifications[0]["label"])
	}
	msg := fmt.Sprintf("Classified text into %d class(es)", len(classifications))
	if topLabel != "" {
		msg = fmt.Sprintf("%s — top: %q", msg, topLabel)
	}

	return lang.Result(msg, map[string]interface{}{
		"results":       results,
		"language_code": lang.Str(docResult.LanguageCode),
	}), nil
}
