// Package oracle_language_batch_detect_language_entities runs OCI Language entity detection over a
// single piece of text: it names the people, places, organisations, dates, quantities and other
// entities the pre-trained model recognises, with each entity's type, position and confidence.
package oracle_language_batch_detect_language_entities

import (
	"fmt"

	core "flomation.app/automate/executor"
	lang "flomation.app/automate/executor/actions/oracle/language"

	"github.com/oracle/oci-go-sdk/v65/ailanguage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Language: Detect Entities"
	Description  = "Detect the named entities (people, places, organisations, dates, quantities and more) in a piece of text using OCI Language's pre-trained model."
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
	{Name: "text", Type: core.ConnectionTypeText, Label: "Text", Placeholder: "The text to detect entities in", Required: true},
	{Name: "language_code", Type: core.ConnectionTypeString, Label: "Language Code", Placeholder: "e.g. en, es, fr — or auto to detect (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Detected entities"},
	{Name: "entity_count", Type: core.ConnectionTypeInteger, Label: "Entity Count"},
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

	key := "1"
	doc := ailanguage.TextDocument{Key: &key, Text: &text}
	if code := lang.OptionalString("language_code", inputs); code != "" {
		doc.LanguageCode = &code
	}

	details := ailanguage.BatchDetectLanguageEntitiesDetails{
		CompartmentId: &compartment,
		Documents:     []ailanguage.TextDocument{doc},
	}

	resp, err := client.BatchDetectLanguageEntities(lang.Context(), ailanguage.BatchDetectLanguageEntitiesRequest{
		BatchDetectLanguageEntitiesDetails: details,
	})
	if err != nil {
		return lang.ErrorResult(auth.OCIError(err)), nil
	}

	// Single document in → single document result out; surface any per-document error the batch API
	// returns rather than the top-level error, so a failed detection is not silently empty.
	for _, de := range resp.Errors {
		if de.Error != nil {
			return lang.ErrorResult(fmt.Sprintf("OCI could not detect entities: %s", lang.Str(de.Error.Message))), nil
		}
	}
	if len(resp.Documents) == 0 {
		return lang.ErrorResult("OCI returned no entity results for the supplied text"), nil
	}

	docResult := resp.Documents[0]
	entities := make([]map[string]interface{}, 0, len(docResult.Entities))
	for i := range docResult.Entities {
		e := docResult.Entities[i]
		entities = append(entities, map[string]interface{}{
			"text":     lang.Str(e.Text),
			"type":     lang.Str(e.Type),
			"sub_type": lang.Str(e.SubType),
			"offset":   lang.IntOrNil(e.Offset),
			"length":   lang.IntOrNil(e.Length),
			"score":    floatOrNil(e.Score),
		})
	}

	results := map[string]interface{}{
		"key":           lang.Str(docResult.Key),
		"language_code": lang.Str(docResult.LanguageCode),
		"entities":      entities,
	}

	return lang.Result(
		fmt.Sprintf("Detected %d entities in the supplied text", len(entities)),
		map[string]interface{}{
			"results":       results,
			"entity_count":  len(entities),
			"language_code": lang.Str(docResult.LanguageCode),
		},
	), nil
}

func floatOrNil(p *float64) interface{} {
	if p == nil {
		return nil
	}
	return *p
}
