// Package oracle_generativeai_chat sends a single message to an on-demand Cohere chat model in
// OCI Generative AI and returns the model's reply. The operator supplies the chat model OCID and
// the message; the reply text and the full raw chat result are returned.
package oracle_generativeai_chat

import (
	"encoding/json"

	core "flomation.app/automate/executor"
	gai "flomation.app/automate/executor/actions/oracle/generativeai"

	"github.com/oracle/oci-go-sdk/v65/generativeaiinference"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Generative AI: Chat"
	Description  = "Send a message to an on-demand Cohere chat model and return its reply."
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
	{Name: "model_id", Type: core.ConnectionTypeString, Label: "Model OCID", Placeholder: "ocid1.generativeaimodel.oc1..aaaa… — an on-demand Cohere chat model", Required: true},
	{Name: "message", Type: core.ConnectionTypeText, Label: "Message", Placeholder: "The text you want the model to respond to", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "response", Type: core.ConnectionTypeObject, Label: "Chat response"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	modelID, err := gai.RequiredString("model_id", inputs)
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}
	message, err := gai.RequiredString("message", inputs)
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}

	auth, client, errResult := gai.InferenceClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartmentID, err := auth.RequiredCompartment()
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}

	req := generativeaiinference.ChatRequest{
		ChatDetails: generativeaiinference.ChatDetails{
			CompartmentId: &compartmentID,
			ServingMode:   generativeaiinference.OnDemandServingMode{ModelId: &modelID},
			ChatRequest:   generativeaiinference.CohereChatRequest{Message: &message},
		},
	}

	resp, err := client.Chat(gai.Context(), req)
	if err != nil {
		return gai.ErrorResult(auth.OCIError(err)), nil
	}

	// Best-effort: marshal the polymorphic ChatResult to a plain map for the raw payload, and
	// pull the reply text out of the concrete Cohere response when available.
	response := map[string]interface{}{}
	if b, mErr := json.Marshal(resp.ChatResult); mErr == nil {
		_ = json.Unmarshal(b, &response)
	}
	text := ""
	if cohere, ok := resp.ChatResult.ChatResponse.(generativeaiinference.CohereChatResponse); ok {
		text = gai.Str(cohere.Text)
	}
	response["text"] = text

	summary := "Chat completed"
	if text != "" {
		summary = text
	}
	return gai.Result(summary, map[string]interface{}{"response": response}), nil
}
