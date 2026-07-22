// Package oracle_identity_idp_group_mapping_list lists the IdP-group-to-IAM-group mappings for one identity provider.
package oracle_identity_idp_group_mapping_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: List IdP Group Mappings"
	Description  = "List the group mappings for an Oracle Cloud identity provider — each links one federated IdP group to one IAM group. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+id-badge"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (not used by this action)"},
	{Name: "identity_provider_ocid", Type: core.ConnectionTypeString, Label: "Identity Provider OCID", Placeholder: "ocid1.saml2idp.oc1..aaaa… of the IdP whose mappings to list", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "mappings", Type: core.ConnectionTypeObject, Label: "Group Mappings"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, idpID, errResult := iam.ResourceClient(inputs, "identity_provider_ocid")
	if errResult != nil {
		return errResult, nil
	}
	req := identity.ListIdpGroupMappingsRequest{IdentityProviderId: &idpID}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= iam.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListIdpGroupMappings(iam.Context(), req)
		if err != nil {
			return iam.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, summariseMapping(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return iam.Result(fmt.Sprintf("Found %d group mapping(s)", len(out)), map[string]interface{}{
		"mappings": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}

func summariseMapping(m *identity.IdpGroupMapping) map[string]interface{} {
	out := map[string]interface{}{
		"id":              iam.Str(m.Id),
		"idp_id":          iam.Str(m.IdpId),
		"idp_group_name":  iam.Str(m.IdpGroupName),
		"group_id":        iam.Str(m.GroupId),
		"compartment_id":  iam.Str(m.CompartmentId),
		"lifecycle_state": string(m.LifecycleState),
		"time_created":    iam.FormatTime(m.TimeCreated),
	}
	if m.InactiveStatus != nil {
		out["inactive_status"] = *m.InactiveStatus
	}
	return out
}
