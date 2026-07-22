// Package oracle_generativeai_generate_text runs a single text-generation (completion) request
// against an on-demand Generative AI model and returns the generated text.
package oracle_generativeai_generate_text

import (
	"fmt"

	core "flomation.app/automate/executor"
	gai "flomation.app/automate/executor/actions/oracle/generativeai"

	"github.com/oracle/oci-go-sdk/v65/generativeaiinference"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Generative AI: Generate Text"
	Description  = "Run a single text-generation (completion) request against an on-demand Generative AI model and return the generated text."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+robot"
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
	{Name: "model_id", Type: core.ConnectionTypeString, Label: "Model ID", Placeholder: "On-demand model OCID or name, e.g. cohere.command-r-08-2024", Required: true},
	{Name: "prompt", Type: core.ConnectionTypeText, Label: "Prompt", Placeholder: "The prompt to complete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "response", Type: core.ConnectionTypeString, Label: "Generated text"},
	{Name: "generations", Type: core.ConnectionTypeObject, Label: "Generations"},
	{Name: "model_id", Type: core.ConnectionTypeString, Label: "Model OCID"},
	{Name: "model_version", Type: core.ConnectionTypeString, Label: "Model Version"},
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
	prompt, err := gai.RequiredString("prompt", inputs)
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}

	details := generativeaiinference.GenerateTextDetails{
		CompartmentId: &compartment,
		ServingMode:   generativeaiinference.OnDemandServingMode{ModelId: &modelID},
		InferenceRequest: generativeaiinference.CohereLlmInferenceRequest{
			Prompt: &prompt,
		},
	}

	resp, err := client.GenerateText(gai.Context(), generativeaiinference.GenerateTextRequest{GenerateTextDetails: details})
	if err != nil {
		return gai.ErrorResult(auth.OCIError(err)), nil
	}

	text := ""
	generations := []map[string]interface{}{}
	if cohere, ok := resp.InferenceResponse.(generativeaiinference.CohereLlmInferenceResponse); ok {
		for i, g := range cohere.GeneratedTexts {
			generations = append(generations, map[string]interface{}{
				"id":            gai.Str(g.Id),
				"text":          gai.Str(g.Text),
				"finish_reason": gai.Str(g.FinishReason),
			})
			if i == 0 {
				text = gai.Str(g.Text)
			}
		}
	}

	return gai.Result(fmt.Sprintf("Generated text from model %q", gai.Str(resp.ModelId)), map[string]interface{}{
		"response":      text,
		"generations":   generations,
		"model_id":      gai.Str(resp.ModelId),
		"model_version": gai.Str(resp.ModelVersion),
	}), nil
}
