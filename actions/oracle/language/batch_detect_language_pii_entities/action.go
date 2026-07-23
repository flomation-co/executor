// Package oracle_language_batch_detect_language_pii_entities scans a piece of text for personal
// identification information (PII) using OCI Language's pre-trained model, returning each detected
// entity (type, offset, confidence) along with a masked version of the text.
package oracle_language_batch_detect_language_pii_entities

import (
	"fmt"

	core "flomation.app/automate/executor"
	lang "flomation.app/automate/executor/actions/oracle/language"

	"github.com/oracle/oci-go-sdk/v65/ailanguage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Language: Detect PII Entities"
	Description  = "Scan text for personal identification information (PII) using OCI Language's pre-trained model, returning each detected entity and a masked version of the text."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (inference is served from the pre-trained model in this compartment)", Required: true},
	{Name: "text", Type: core.ConnectionTypeText, Label: "Text", Placeholder: "The text to scan for PII", Required: true},
	{Name: "language_code", Type: core.ConnectionTypeString, Label: "Language Code", Placeholder: "e.g. en (optional — auto-detected when blank)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "PII detection results"},
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
	if lc := lang.OptionalString("language_code", inputs); lc != "" {
		doc.LanguageCode = &lc
	}
	req := ailanguage.BatchDetectLanguagePiiEntitiesRequest{
		BatchDetectLanguagePiiEntitiesDetails: ailanguage.BatchDetectLanguagePiiEntitiesDetails{
			CompartmentId: &compartment,
			Documents:     []ailanguage.TextDocument{doc},
		},
	}

	resp, err := client.BatchDetectLanguagePiiEntities(lang.Context(), req)
	if err != nil {
		return lang.ErrorResult(auth.OCIError(err)), nil
	}

	var docs []map[string]interface{}
	totalEntities := 0
	for i := range resp.Documents {
		d := &resp.Documents[i]
		var entities []map[string]interface{}
		for j := range d.Entities {
			e := &d.Entities[j]
			entities = append(entities, map[string]interface{}{
				"offset":        lang.IntOrNil(e.Offset),
				"length":        lang.IntOrNil(e.Length),
				"text":          lang.Str(e.Text),
				"type":          lang.Str(e.Type),
				"score":         floatOrNil(e.Score),
				"id":            lang.Str(e.Id),
				"relexify_text": lang.Str(e.RelexifyText),
			})
		}
		totalEntities += len(d.Entities)
		docs = append(docs, map[string]interface{}{
			"key":           lang.Str(d.Key),
			"language_code": lang.Str(d.LanguageCode),
			"masked_text":   lang.Str(d.MaskedText),
			"entities":      entities,
		})
	}

	var errs []map[string]interface{}
	for i := range resp.Errors {
		e := &resp.Errors[i]
		em := map[string]interface{}{"key": lang.Str(e.Key)}
		if e.Error != nil {
			em["code"] = lang.Str(e.Error.Code)
			em["message"] = lang.Str(e.Error.Message)
		}
		errs = append(errs, em)
	}

	results := map[string]interface{}{"documents": docs}
	if len(errs) > 0 {
		results["errors"] = errs
	}

	return lang.Result(fmt.Sprintf("Detected %d PII entit%s", totalEntities, plural(totalEntities)), map[string]interface{}{
		"results": results,
	}), nil
}

func floatOrNil(p *float64) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
