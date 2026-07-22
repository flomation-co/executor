// Package oracle_cloudguard_managed_list_create creates a Cloud Guard managed list: a named,
// typed collection of values (CIDR blocks, user or group OCIDs, IP addresses, …) that detector
// and responder rules reference to parameterise what they match on.
package oracle_cloudguard_managed_list_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	cg "flomation.app/automate/executor/actions/oracle/cloudguard"

	"github.com/oracle/oci-go-sdk/v65/cloudguard"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Cloud Guard: Create Managed List"
	Description  = "Create a Cloud Guard managed list — a named, typed collection of values (CIDR blocks, user/group OCIDs, IP addresses, …) that detector and responder rules reference to parameterise what they match on."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-halved"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… — where the managed list is created", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the managed list", Required: true},
	{Name: "list_type", Type: core.ConnectionTypeString, Label: "List Type", Placeholder: "The kind of value stored in the list", Required: true, Options: []core.ConnectionOption{
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
	{Name: "list_items", Type: core.ConnectionTypeText, Label: "List Items (CSV)", Placeholder: "Comma-separated values, e.g. 10.0.0.0/24, 192.168.0.0/16 (optional)"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "managed_list", Type: core.ConnectionTypeObject, Label: "Managed List"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Managed List OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
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
	name, err := cg.RequiredString("display_name", inputs)
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}
	listType, err := cg.RequiredString("list_type", inputs)
	if err != nil {
		return cg.ErrorResult(err.Error()), nil
	}
	if _, ok := cloudguard.GetMappingManagedListTypeEnum(listType); !ok {
		return cg.ErrorResult(fmt.Sprintf("list type must be one of: %s", strings.Join(cloudguard.GetManagedListTypeEnumStringValues(), ", "))), nil
	}

	details := cloudguard.CreateManagedListDetails{
		DisplayName:   &name,
		CompartmentId: &compartment,
		ListType:      cloudguard.ManagedListTypeEnum(listType),
	}
	if items := csvItems(cg.OptionalString("list_items", inputs)); len(items) > 0 {
		details.ListItems = items
	}
	if d := cg.OptionalString("description", inputs); strings.TrimSpace(d) != "" {
		details.Description = &d
	}

	resp, err := client.CreateManagedList(cg.Context(), cloudguard.CreateManagedListRequest{CreateManagedListDetails: details})
	if err != nil {
		return cg.ErrorResult(auth.OCIError(err)), nil
	}
	list := cg.SummariseManagedList(&resp.ManagedList)
	return cg.Result(fmt.Sprintf("Created Cloud Guard managed list %q (%s)", list["display_name"], list["lifecycle_state"]), map[string]interface{}{
		"managed_list": list, "id": list["id"], "lifecycle_state": list["lifecycle_state"],
	}), nil
}

// csvItems splits a comma-separated string into trimmed, non-empty values.
func csvItems(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}
