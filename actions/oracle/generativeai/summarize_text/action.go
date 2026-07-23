// Package oracle_generativeai_summarize_text summarizes a block of input text with an on-demand
// OCI Generative AI model, returning the generated summary.
package oracle_generativeai_summarize_text

import (
	"fmt"

	core "flomation.app/automate/executor"
	gai "flomation.app/automate/executor/actions/oracle/generativeai"

	"github.com/oracle/oci-go-sdk/v65/generativeaiinference"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Generative AI: Summarize Text"
	Description  = "Summarize a block of text with an on-demand OCI Generative AI model and return the summary."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+robot"
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
	{Name: "model_id", Type: core.ConnectionTypeString, Label: "Model OCID", Placeholder: "ocid1.generativeaimodel.oc1..aaaa… (a model with the TEXT_SUMMARIZATION capability)", Required: true},
	{Name: "input", Type: core.ConnectionTypeString, Label: "Input Text", Placeholder: "The text to summarize", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "summary", Type: core.ConnectionTypeString, Label: "Summary"},
	{Name: "model_id", Type: core.ConnectionTypeString, Label: "Model OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := gai.InferenceClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}
	modelID, err := gai.RequiredString("model_id", inputs)
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}
	input, err := gai.RequiredString("input", inputs)
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}

	resp, err := client.SummarizeText(gai.Context(), generativeaiinference.SummarizeTextRequest{
		SummarizeTextDetails: generativeaiinference.SummarizeTextDetails{
			CompartmentId: &compartment,
			ServingMode:   generativeaiinference.OnDemandServingMode{ModelId: &modelID},
			Input:         &input,
		},
	})
	if err != nil {
		return gai.ErrorResult(auth.OCIError(err)), nil
	}

	summary := gai.Str(resp.SummarizeTextResult.Summary)
	return gai.Result(fmt.Sprintf("Summarized %d characters into %d.", len(input), len(summary)), map[string]interface{}{
		"summary":  summary,
		"model_id": modelID,
	}), nil
}
