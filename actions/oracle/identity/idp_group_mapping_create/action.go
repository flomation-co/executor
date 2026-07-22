// Package oracle_identity_idp_group_mapping_create maps an identity provider (IdP) group
// to an IAM Service group, so federated users in that IdP group inherit the IAM group.
package oracle_identity_idp_group_mapping_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Create IdP Group Mapping"
	Description  = "Map an Oracle Cloud identity provider (IdP) group to an IAM Service group, so federated users in that IdP group inherit the IAM group's access."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (scopes the group picker)"},
	{Name: "identity_provider_ocid", Type: core.ConnectionTypeString, Label: "Identity Provider OCID", Placeholder: "ocid1.saml2idp.oc1..aaaa… the IdP to map from", Required: true},
	{Name: "idp_group_name", Type: core.ConnectionTypeString, Label: "IdP Group Name", Placeholder: "The name of the group as defined in the identity provider", Required: true},
	{Name: "group_ocid", Type: core.ConnectionTypeString, Label: "IAM Group OCID", Placeholder: "ocid1.group.oc1..aaaa… the IAM Service group to map to", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "idp_group_mapping", Type: core.ConnectionTypeObject, Label: "IdP Group Mapping"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Mapping OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := iam.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	idpID, err := iam.RequiredString("identity_provider_ocid", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	idpGroupName, err := iam.RequiredString("idp_group_name", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	groupID, err := iam.RequiredString("group_ocid", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	resp, err := client.CreateIdpGroupMapping(iam.Context(), identity.CreateIdpGroupMappingRequest{
		IdentityProviderId: &idpID,
		CreateIdpGroupMappingDetails: identity.CreateIdpGroupMappingDetails{
			IdpGroupName: &idpGroupName,
			GroupId:      &groupID,
		},
	})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	m := resp.IdpGroupMapping
	mapping := map[string]interface{}{
		"id":                   iam.Str(m.Id),
		"idp_group_name":       iam.Str(m.IdpGroupName),
		"group_id":             iam.Str(m.GroupId),
		"identity_provider_id": iam.Str(m.IdpId),
		"compartment_id":       iam.Str(m.CompartmentId),
		"lifecycle_state":      string(m.LifecycleState),
		"time_created":         iam.FormatTime(m.TimeCreated),
	}
	return iam.Result(
		fmt.Sprintf("Mapped IdP group %q to IAM group %s (%s)", iam.Str(m.IdpGroupName), iam.Str(m.GroupId), string(m.LifecycleState)),
		map[string]interface{}{"idp_group_mapping": mapping, "id": mapping["id"]},
	), nil
}
