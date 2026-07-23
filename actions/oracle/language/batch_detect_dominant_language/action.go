// Package oracle_language_batch_detect_dominant_language detects the dominant (most likely)
// language of a piece of text, returning the ranked ISO language codes and confidence scores.
package oracle_language_batch_detect_dominant_language

import (
	"fmt"

	core "flomation.app/automate/executor"
	lang "flomation.app/automate/executor/actions/oracle/language"

	"github.com/oracle/oci-go-sdk/v65/ailanguage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Language: Detect Dominant Language"
	Description  = "Detect the most likely language of a piece of text, with ranked ISO language codes and confidence scores."
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
	{Name: "text", Type: core.ConnectionTypeString, Label: "Text", Placeholder: "The text whose dominant language should be detected", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Detection results"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func floatOrNil(p *float64) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := lang.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	// Pre-trained batch inference is served from the request's compartment — it routes and
	// authorizes the call. Every sibling batch op sets it; this one must too (per review).
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return lang.ErrorResult(err.Error()), nil
	}
	text, err := lang.RequiredString("text", inputs)
	if err != nil {
		return lang.ErrorResult(err.Error()), nil
	}

	key := "1"
	resp, err := client.BatchDetectDominantLanguage(lang.Context(), ailanguage.BatchDetectDominantLanguageRequest{
		BatchDetectDominantLanguageDetails: ailanguage.BatchDetectDominantLanguageDetails{
			CompartmentId: &compartment,
			Documents: []ailanguage.DominantLanguageDocument{
				{Key: &key, Text: &text},
			},
		},
	})
	if err != nil {
		return lang.ErrorResult(auth.OCIError(err)), nil
	}

	languages := []map[string]interface{}{}
	for _, doc := range resp.Documents {
		for _, l := range doc.Languages {
			languages = append(languages, map[string]interface{}{
				"code":  lang.Str(l.Code),
				"name":  lang.Str(l.Name),
				"score": floatOrNil(l.Score),
			})
		}
	}

	docErrors := []map[string]interface{}{}
	for _, e := range resp.Errors {
		var code, message string
		if e.Error != nil {
			code = lang.Str(e.Error.Code)
			message = lang.Str(e.Error.Message)
		}
		docErrors = append(docErrors, map[string]interface{}{
			"key":     lang.Str(e.Key),
			"code":    code,
			"message": message,
		})
	}

	results := map[string]interface{}{
		"languages": languages,
		"errors":    docErrors,
	}

	var msg string
	if len(languages) > 0 {
		top := languages[0]
		msg = fmt.Sprintf("Detected dominant language: %s (%s)", top["name"], top["code"])
	} else if len(docErrors) > 0 {
		msg = fmt.Sprintf("Language detection returned an error: %s", docErrors[0]["message"])
	} else {
		msg = "No dominant language could be detected for the supplied text"
	}

	return lang.Result(msg, map[string]interface{}{"results": results}), nil
}
