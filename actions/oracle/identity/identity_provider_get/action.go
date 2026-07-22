// Package oracle_identity_identity_provider_get reads one IAM identity provider by OCID.
package oracle_identity_identity_provider_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	iam "flomation.app/automate/executor/actions/oracle/identity"

	identity "github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Identity: Get Identity Provider"
	Description  = "Fetch a single Oracle Cloud IAM identity provider by OCID — its name, product type, protocol and (for SAML2) metadata/redirect URLs and signing certificate."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (identity providers live in the tenancy)"},
	{Name: "identity_provider_ocid", Type: core.ConnectionTypeString, Label: "Identity Provider OCID", Placeholder: "ocid1.saml2idp.oc1..aaaa… of the identity provider to fetch", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "identity_provider", Type: core.ConnectionTypeObject, Label: "Identity Provider"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Identity Provider OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := iam.ResourceClient(inputs, "identity_provider_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetIdentityProvider(iam.Context(), identity.GetIdentityProviderRequest{IdentityProviderId: &id})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}

	// resp.IdentityProvider is a polymorphic interface — build the base map from its
	// getters, then surface SAML2-specific fields when that is the concrete type.
	ip := resp.IdentityProvider
	provider := map[string]interface{}{
		"id":              iam.Str(ip.GetId()),
		"name":            iam.Str(ip.GetName()),
		"description":     iam.Str(ip.GetDescription()),
		"compartment_id":  iam.Str(ip.GetCompartmentId()),
		"product_type":    iam.Str(ip.GetProductType()),
		"lifecycle_state": string(ip.GetLifecycleState()),
		"time_created":    iam.FormatTime(ip.GetTimeCreated()),
		"freeform_tags":   ip.GetFreeformTags(),
		"defined_tags":    ip.GetDefinedTags(),
	}
	if s := ip.GetInactiveStatus(); s != nil {
		provider["inactive_status"] = *s
	}

	protocol := ""
	if saml, ok := ip.(identity.Saml2IdentityProvider); ok {
		protocol = "SAML2"
		provider["protocol"] = protocol
		provider["metadata_url"] = iam.Str(saml.MetadataUrl)
		provider["redirect_url"] = iam.Str(saml.RedirectUrl)
		provider["signing_certificate"] = iam.Str(saml.SigningCertificate)
		provider["metadata"] = iam.Str(saml.Metadata)
		provider["freeform_attributes"] = saml.FreeformAttributes
	}

	summary := fmt.Sprintf("Identity provider %q (%s) is %s", provider["name"], provider["product_type"], provider["lifecycle_state"])
	if protocol != "" {
		summary = fmt.Sprintf("Identity provider %q (%s, %s) is %s", provider["name"], provider["product_type"], protocol, provider["lifecycle_state"])
	}
	return iam.Result(summary, map[string]interface{}{"identity_provider": provider, "id": provider["id"]}), nil
}
