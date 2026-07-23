// Package oracle_cloudguard_managed_list_list lists the managed lists in a compartment, optionally
// filtered by display name, list type or lifecycle state. Walks pagination up to a safe cap.
package oracle_cloudguard_managed_list_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	cg "flomation.app/automate/executor/actions/oracle/cloudguard"

	"github.com/oracle/oci-go-sdk/v65/cloudguard"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Cloud Guard: List Managed Lists"
	Description  = "List the Cloud Guard managed lists in a compartment. Optionally filter by exact display name, list type or lifecycle state. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-halved"
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name Filter", Placeholder: "Only managed lists with this exact name (optional)"},
	{Name: "list_type", Type: core.ConnectionTypeString, Label: "List Type", Placeholder: "Filter by the kind of managed list (optional)", Options: []core.ConnectionOption{
		{Name: "CIDR Block", Value: "CIDR_BLOCK"},
		{Name: "Users", Value: "USERS"},
		{Name: "Groups", Value: "GROUPS"},
		{Name: "IPv4 Address", Value: "IPV4ADDRESS"},
		{Name: "IPv6 Address", Value: "IPV6ADDRESS"},
		{Name: "Resource OCID", Value: "RESOURCE_OCID"},
		{Name: "Region", Value: "REGION"},
		{Name: "Country", Value: "COUNTRY"},
		{Name: "State", Value: "STATE"},
		{Name: "City", Value: "CITY"},
		{Name: "Tags", Value: "TAGS"},
		{Name: "Generic", Value: "GENERIC"},
		{Name: "Fusion Apps Role", Value: "FUSION_APPS_ROLE"},
		{Name: "Fusion Apps Permission", Value: "FUSION_APPS_PERMISSION"},
		{Name: "Namespace Selector", Value: "NAMESPACE_SELECTOR"},
		{Name: "Pod Resource Selector", Value: "POD_RESOURCE_SELECTOR"},
	}},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State", Placeholder: "Defaults to ACTIVE when unset", Options: []core.ConnectionOption{
		{Name: "Creating", Value: "CREATING"},
		{Name: "Updating", Value: "UPDATING"},
		{Name: "Active", Value: "ACTIVE"},
		{Name: "Inactive", Value: "INACTIVE"},
		{Name: "Deleting", Value: "DELETING"},
		{Name: "Deleted", Value: "DELETED"},
		{Name: "Failed", Value: "FAILED"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Max items per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "managed_lists", Type: core.ConnectionTypeObject, Label: "Managed Lists"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := cg.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}
	req := cloudguard.ListManagedListsRequest{CompartmentId: &compartment}
	if name := cg.OptionalString("display_name", inputs); name != "" {
		req.DisplayName = &name
	}
	if lt := cg.OptionalString("list_type", inputs); lt != "" {
		req.ListType = cloudguard.ListManagedListsListTypeEnum(lt)
	}
	if state := cg.OptionalString("lifecycle_state", inputs); state != "" {
		req.LifecycleState = cloudguard.ListManagedListsLifecycleStateEnum(state)
	}
	if limit, ok, err := cg.OptionalInt("limit", inputs); err != nil {
		return cg.ErrorResult(err.Error()), nil
	} else if ok {
		req.Limit = &limit
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= cg.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListManagedLists(cg.Context(), req)
		if err != nil {
			return cg.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, cg.SummariseManagedListSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return cg.Result(fmt.Sprintf("Found %d managed list(s)", len(out)), map[string]interface{}{
		"managed_lists": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
