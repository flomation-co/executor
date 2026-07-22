// Package oracle_generativeai_dedicated_ai_cluster_list lists the dedicated AI clusters in a
// compartment, optionally filtered by exact display name or lifecycle state. Walks pagination up
// to a safe cap.
package oracle_generativeai_dedicated_ai_cluster_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	gai "flomation.app/automate/executor/actions/oracle/generativeai"

	"github.com/oracle/oci-go-sdk/v65/generativeai"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Generative AI: List Dedicated AI Clusters"
	Description  = "List the dedicated AI clusters in a compartment. Optionally filter by exact display name or lifecycle state, and cap the page size. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only clusters with this exact name (optional)"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Filter by lifecycle state (optional)", Options: []core.ConnectionOption{
		{Name: "Creating", Value: "CREATING"},
		{Name: "Active", Value: "ACTIVE"},
		{Name: "Updating", Value: "UPDATING"},
		{Name: "Deleting", Value: "DELETING"},
		{Name: "Deleted", Value: "DELETED"},
		{Name: "Failed", Value: "FAILED"},
		{Name: "Needs Attention", Value: "NEEDS_ATTENTION"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Max results per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "clusters", Type: core.ConnectionTypeObject, Label: "Dedicated AI Clusters"},
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
	req := generativeai.ListDedicatedAiClustersRequest{CompartmentId: &compartment}
	if name := gai.OptionalString("display_name", inputs); name != "" {
		req.DisplayName = &name
	}
	if state := gai.OptionalString("lifecycle_state", inputs); state != "" {
		req.LifecycleState = generativeai.DedicatedAiClusterLifecycleStateEnum(state)
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
		resp, err := client.ListDedicatedAiClusters(gai.Context(), req)
		if err != nil {
			return gai.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, gai.SummariseDedicatedAiClusterSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return gai.Result(fmt.Sprintf("Found %d dedicated AI cluster(s)", len(out)), map[string]interface{}{
		"clusters": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
