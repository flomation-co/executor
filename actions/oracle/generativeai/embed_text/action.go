// Package oracle_generativeai_embed_text turns text into vector embeddings using an OCI
// Generative AI embedding model (on-demand serving), returning one float vector per input string.
package oracle_generativeai_embed_text

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	gai "flomation.app/automate/executor/actions/oracle/generativeai"

	"github.com/oracle/oci-go-sdk/v65/generativeaiinference"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Generative AI: Embed Text"
	Description  = "Turn one or more input strings into vector embeddings with an on-demand OCI Generative AI embedding model."
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
	{Name: "model_id", Type: core.ConnectionTypeString, Label: "Embedding Model OCID / Name", Placeholder: "e.g. cohere.embed-english-v3.0 or an ocid1.generativeaimodel… OCID", Required: true},
	{Name: "inputs", Type: core.ConnectionTypeText, Label: "Inputs (comma-separated)", Placeholder: "One or more strings to embed, comma-separated. Each up to 512 tokens.", Required: true},
	{Name: "truncate", Type: core.ConnectionTypeString, Label: "Truncate", Placeholder: "How to trim inputs longer than the model limit (default NONE)", Options: []core.ConnectionOption{
		{Name: "None (error if too long)", Value: "NONE"},
		{Name: "Start", Value: "START"},
		{Name: "End", Value: "END"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "embeddings", Type: core.ConnectionTypeObject, Label: "Embeddings (one vector per input)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Embedding Count"},
	{Name: "dimension", Type: core.ConnectionTypeInteger, Label: "Vector Dimension"},
	{Name: "model_id", Type: core.ConnectionTypeString, Label: "Model OCID"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Result ID"},
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
	rawInputs, err := gai.RequiredString("inputs", inputs)
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}
	texts := splitCSV(rawInputs)
	if len(texts) == 0 {
		return gai.ErrorResult("inputs must contain at least one non-empty string"), nil
	}

	details := generativeaiinference.EmbedTextDetails{
		CompartmentId: &compartment,
		ServingMode:   generativeaiinference.OnDemandServingMode{ModelId: &modelID},
		Inputs:        texts,
	}
	if t := strings.ToUpper(strings.TrimSpace(gai.OptionalString("truncate", inputs))); t != "" {
		details.Truncate = generativeaiinference.EmbedTextDetailsTruncateEnum(t)
	}

	resp, err := client.EmbedText(gai.Context(), generativeaiinference.EmbedTextRequest{EmbedTextDetails: details})
	if err != nil {
		return gai.ErrorResult(auth.OCIError(err)), nil
	}

	count := len(resp.Embeddings)
	dimension := 0
	if count > 0 {
		dimension = len(resp.Embeddings[0])
	}
	result := map[string]interface{}{
		"embeddings": resp.Embeddings,
		"count":      count,
		"dimension":  dimension,
		"model_id":   gai.Str(resp.ModelId),
		"id":         gai.Str(resp.Id),
	}
	return gai.Result(
		fmt.Sprintf("Embedded %d input(s) into %d-dimension vectors with model %s", count, dimension, gai.Str(resp.ModelId)),
		result,
	), nil
}

// splitCSV turns a comma-separated string into a trimmed list, dropping empty entries.
func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}
