// Package oracle_generativeai_model_list lists the Generative AI models available in a compartment,
// optionally filtered by capability, vendor or lifecycle state. Walks pagination up to a safe cap.
package oracle_generativeai_model_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	gai "flomation.app/automate/executor/actions/oracle/generativeai"

	"github.com/oracle/oci-go-sdk/v65/generativeai"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Generative AI: List Models"
	Description  = "List the Generative AI models in a compartment. Optionally filter by capability, vendor or lifecycle state. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "capability", Type: core.ConnectionTypeString, Label: "Capability Filter", Placeholder: "Only models with this capability (optional)", Options: []core.ConnectionOption{
		{Name: "Chat", Value: "CHAT"},
		{Name: "Text Generation", Value: "TEXT_GENERATION"},
		{Name: "Text Summarization", Value: "TEXT_SUMMARIZATION"},
		{Name: "Text Embeddings", Value: "TEXT_EMBEDDINGS"},
		{Name: "Text Rerank", Value: "TEXT_RERANK"},
		{Name: "Fine-Tune", Value: "FINE_TUNE"},
	}},
	{Name: "vendor", Type: core.ConnectionTypeString, Label: "Vendor Filter", Placeholder: "Only models from this exact vendor, e.g. cohere or meta (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Only models in this state (optional)", Options: []core.ConnectionOption{
		{Name: "Active", Value: "ACTIVE"},
		{Name: "Creating", Value: "CREATING"},
		{Name: "Deleting", Value: "DELETING"},
		{Name: "Deleted", Value: "DELETED"},
		{Name: "Failed", Value: "FAILED"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Max results per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "models", Type: core.ConnectionTypeObject, Label: "Models"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := gai.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}
	req := generativeai.ListModelsRequest{CompartmentId: &compartment}
	if capability := gai.OptionalString("capability", inputs); capability != "" {
		req.Capability = []generativeai.ModelCapabilityEnum{generativeai.ModelCapabilityEnum(capability)}
	}
	if vendor := gai.OptionalString("vendor", inputs); vendor != "" {
		req.Vendor = &vendor
	}
	if state := gai.OptionalString("lifecycle_state", inputs); state != "" {
		req.LifecycleState = generativeai.ModelLifecycleStateEnum(state)
	}
	if limit, ok, err := gai.OptionalInt("limit", inputs); err != nil {
		return gai.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &limit
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= gai.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListModels(gai.Context(), req)
		if err != nil {
			return gai.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, gai.SummariseModelSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return gai.Result(fmt.Sprintf("Found %d model(s)", len(out)), map[string]interface{}{
		"models": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
