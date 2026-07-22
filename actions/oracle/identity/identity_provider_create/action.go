// Package oracle_identity_identity_provider_create creates a SAML2 federation identity provider in the tenancy.
package oracle_identity_identity_provider_create

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
	Name         = "OCI Identity: Create Identity Provider"
	Description  = "Create a SAML2 federation identity provider in an Oracle Cloud tenancy — trust an external IdP (IDCS or ADFS) by supplying its SAML metadata XML so its users can federate in."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "Leave blank for the tenancy (identity providers live in the root)"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Unique IdP name, cannot be changed later, e.g. corp-adfs", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What this identity provider is for", Required: true},
	{Name: "product_type", Type: core.ConnectionTypeString, Label: "Product Type", Placeholder: "IDCS or ADFS", Required: true},
	{Name: "metadata", Type: core.ConnectionTypeText, Label: "SAML Metadata XML", Placeholder: "Paste the IdP's SAML 2.0 metadata XML (the full <EntityDescriptor>…</EntityDescriptor>)", Required: true},
	{Name: "metadata_url", Type: core.ConnectionTypeString, Label: "Metadata URL", Placeholder: "URL the IdP metadata was retrieved from (optional)"},
	{Name: "freeform_attributes", Type: core.ConnectionTypeString, Label: "Freeform Attributes (JSON)", Placeholder: `{"clientId":"app_sf3kdjf3"} (optional)`},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"team":"ops"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "identity_provider", Type: core.ConnectionTypeObject, Label: "Identity Provider"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Identity Provider OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := iam.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	name, err := iam.RequiredString("name", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	description, err := iam.RequiredString("description", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	metadata, err := iam.RequiredString("metadata", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	rawProduct, err := iam.RequiredString("product_type", inputs)
	if err != nil {
		return iam.ErrorResult(err.Error()), nil
	}
	productType, ok := identity.GetMappingCreateIdentityProviderDetailsProductTypeEnum(rawProduct)
	if !ok {
		return iam.ErrorResult(fmt.Sprintf("product type %q is not supported — use one of: %s", rawProduct, strings.Join(identity.GetCreateIdentityProviderDetailsProductTypeEnumStringValues(), ", "))), nil
	}

	compartment := auth.CompartmentOrTenancy()
	details := identity.CreateSaml2IdentityProviderDetails{
		CompartmentId: &compartment,
		Name:          &name,
		Description:   &description,
		Metadata:      &metadata,
		ProductType:   productType,
	}
	if metadataURL := iam.OptionalString("metadata_url", inputs); strings.TrimSpace(metadataURL) != "" {
		details.MetadataUrl = &metadataURL
	}
	if attrs, err := iam.FreeformTags("freeform_attributes", inputs); err != nil {
		return iam.ErrorResult(err.Error()), nil
	} else {
		details.FreeformAttributes = attrs
	}
	if tags, err := iam.FreeformTags("tags", inputs); err != nil {
		return iam.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}

	resp, err := client.CreateIdentityProvider(iam.Context(), identity.CreateIdentityProviderRequest{CreateIdentityProviderDetails: details})
	if err != nil {
		return iam.ErrorResult(auth.OCIError(err)), nil
	}
	idp := resp.IdentityProvider
	if idp == nil {
		return iam.ErrorResult("OCI returned an empty identity provider"), nil
	}
	provider := map[string]interface{}{
		"id":              iam.Str(idp.GetId()),
		"name":            iam.Str(idp.GetName()),
		"description":     iam.Str(idp.GetDescription()),
		"compartment_id":  iam.Str(idp.GetCompartmentId()),
		"product_type":    iam.Str(idp.GetProductType()),
		"lifecycle_state": string(idp.GetLifecycleState()),
		"time_created":    iam.FormatTime(idp.GetTimeCreated()),
	}
	return iam.Result(fmt.Sprintf("Created identity provider %q (%s)", name, provider["product_type"]), map[string]interface{}{
		"identity_provider": provider, "id": provider["id"],
	}), nil
}
