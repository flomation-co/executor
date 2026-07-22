// Package oracle_identity_tag_list lists the tag key definitions in a tag namespace.
package oracle_identity_tag_list

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: List Tag Keys"
	Description  = "List the tag key definitions in an Oracle Cloud tag namespace, optionally filtered by exact name. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+tag"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa… (the caller's user, for signing)", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "the tenancy home region, e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (tag namespaces are addressed by OCID)"},
	{Name: "tag_namespace_ocid", Type: core.ConnectionTypeString, Label: "Tag Namespace OCID", Placeholder: "ocid1.tagnamespace.oc1..aaaa… — the namespace whose tag keys to list", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name Filter", Placeholder: "Only the tag key with this exact name (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "tags", Type: core.ConnectionTypeObject, Label: "Tag Keys"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, namespaceID, errResult := iam.ResourceClient(inputs, "tag_namespace_ocid")
	if errResult != nil {
		return errResult, nil
	}
	nameFilter := strings.TrimSpace(iam.OptionalString("name", inputs))

	req := identity.ListTagsRequest{TagNamespaceId: &namespaceID}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= iam.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListTags(iam.Context(), req)
		if err != nil {
			return iam.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			t := &resp.Items[i]
			if nameFilter != "" && iam.Str(t.Name) != nameFilter {
				continue
			}
			out = append(out, map[string]interface{}{
				"id":               iam.Str(t.Id),
				"name":             iam.Str(t.Name),
				"description":      iam.Str(t.Description),
				"compartment_id":   iam.Str(t.CompartmentId),
				"lifecycle_state":  string(t.LifecycleState),
				"is_retired":       t.IsRetired != nil && *t.IsRetired,
				"is_cost_tracking": t.IsCostTracking != nil && *t.IsCostTracking,
				"freeform_tags":    t.FreeformTags,
				"defined_tags":     t.DefinedTags,
				"time_created":     iam.FormatTime(t.TimeCreated),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return iam.Result(fmt.Sprintf("Found %d tag key(s)", len(out)), map[string]interface{}{
		"tags": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
