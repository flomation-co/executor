// Package oracle_generativeai_endpoint_create creates a Generative AI endpoint — the hosting
// front door that serves a model from a dedicated AI cluster for inference. Asynchronous: the
// endpoint comes back with a work-request id; poll Get Endpoint until it is ACTIVE before use.
package oracle_generativeai_endpoint_create

import (
	core "flomation.app/automate/executor"
	gai "flomation.app/automate/executor/actions/oracle/generativeai"

	"github.com/oracle/oci-go-sdk/v65/generativeai"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Generative AI: Create Endpoint"
	Description  = "Create an endpoint that hosts a model on a dedicated AI cluster for inference. Returns a work-request id — poll Get Endpoint until ACTIVE."
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
	{Name: "model_ocid", Type: core.ConnectionTypeString, Label: "Model OCID", Placeholder: "ocid1.generativeaimodel.oc1..aaaa… — the model to host", Required: true},
	{Name: "dedicated_ai_cluster_ocid", Type: core.ConnectionTypeString, Label: "Dedicated AI Cluster OCID", Placeholder: "ocid1.generativeaidedicatedaicluster.oc1..aaaa… — where the model is deployed", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the endpoint (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
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
	modelID, err := gai.RequiredString("model_ocid", inputs)
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}
	clusterID, err := gai.RequiredString("dedicated_ai_cluster_ocid", inputs)
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}

	details := generativeai.CreateEndpointDetails{
		CompartmentId:        &compartment,
		ModelId:              &modelID,
		DedicatedAiClusterId: &clusterID,
	}
	if name, err := gai.RequiredString("display_name", inputs); err == nil {
		details.DisplayName = &name
	}

	resp, err := client.CreateEndpoint(gai.Context(), generativeai.CreateEndpointRequest{CreateEndpointDetails: details})
	if err != nil {
		return gai.ErrorResult(auth.OCIError(err)), nil
	}
	return gai.Result("Creating endpoint — poll Get Endpoint until ACTIVE", map[string]interface{}{
		"work_request_id": gai.Str(resp.OpcWorkRequestId),
	}), nil
}
