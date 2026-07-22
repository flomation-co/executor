// Package oracle_identity_idp_group_mapping_delete deletes one IdP-group mapping from an IAM identity provider.
package oracle_identity_idp_group_mapping_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Delete IdP Group Mapping"
	Description  = "Delete a single group mapping (the link between an identity-provider group and an Oracle Cloud IAM group) by its OCID. Synchronous."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (scopes the identity-provider picker)"},
	{Name: "identity_provider_ocid", Type: core.ConnectionTypeString, Label: "Identity Provider OCID", Placeholder: "ocid1.saml2idp.oc1..aaaa… the mapping belongs to", Required: true},
	{Name: "mapping_ocid", Type: core.ConnectionTypeString, Label: "Group Mapping OCID (to delete)", Placeholder: "ocid1.idpgroupmapping.oc1..aaaa… of the mapping to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Group Mapping OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, providerID, errResult := iam.ResourceClient(inputs, "identity_provider_ocid")
	if errResult != nil {
		return errResult, nil
	}
	mappingID, err := iam.RequiredString("mapping_ocid", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	if _, err := client.DeleteIdpGroupMapping(iam.Context(), identity.DeleteIdpGroupMappingRequest{
		IdentityProviderId: &providerID,
		MappingId:          &mappingID,
	}); err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	return iam.Result(fmt.Sprintf("Deleted group mapping %s", mappingID), map[string]interface{}{"id": mappingID}), nil
}
