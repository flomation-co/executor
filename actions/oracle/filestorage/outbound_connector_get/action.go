// Package oracle_filestorage_outbound_connector_get reads one File Storage
// outbound connector (e.g. an LDAP bind account) by OCID.
package oracle_filestorage_outbound_connector_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Get Outbound Connector"
	Description  = "Fetch a single Oracle Cloud File Storage outbound connector by OCID — its display name, availability domain, lifecycle state and (for an LDAP bind account) the server endpoints and bind distinguished name."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+server"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the outbound-connector picker)"},
	{Name: "outbound_connector_ocid", Type: core.ConnectionTypeString, Label: "Outbound Connector OCID", Placeholder: "ocid1.outboundconnector.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "outbound_connector", Type: core.ConnectionTypeObject, Label: "Outbound Connector"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Outbound Connector OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := fss.ResourceClient(inputs, "outbound_connector_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetOutboundConnector(fss.Context(), filestorage.GetOutboundConnectorRequest{OutboundConnectorId: &id})
	if err != nil {
		return fss.ErrorResult(auth.OCIError(err)), nil
	}

	oc := resp.OutboundConnector
	connector := map[string]interface{}{
		"id":                  fss.Str(oc.GetId()),
		"display_name":        fss.Str(oc.GetDisplayName()),
		"compartment_id":      fss.Str(oc.GetCompartmentId()),
		"availability_domain": fss.Str(oc.GetAvailabilityDomain()),
		"lifecycle_state":     string(oc.GetLifecycleState()),
		"freeform_tags":       oc.GetFreeformTags(),
		"defined_tags":        oc.GetDefinedTags(),
		"time_created":        fss.FormatTime(oc.GetTimeCreated()),
	}

	// The only concrete connector type today is an LDAP bind account — surface its
	// distinctive fields (endpoints, bind DN, Vault secret references) when present.
	if ldap, ok := oc.(filestorage.LdapBindAccount); ok {
		connector["connector_type"] = "LDAPBIND"
		connector["bind_distinguished_name"] = fss.Str(ldap.BindDistinguishedName)
		endpoints := make([]map[string]interface{}, 0, len(ldap.Endpoints))
		for _, e := range ldap.Endpoints {
			endpoints = append(endpoints, map[string]interface{}{
				"hostname": fss.Str(e.Hostname),
				"port":     fss.Int64OrNil(e.Port),
			})
		}
		connector["endpoints"] = endpoints
		connector["password_secret_id"] = fss.Str(ldap.PasswordSecretId)
		if ldap.PasswordSecretVersion != nil {
			connector["password_secret_version"] = *ldap.PasswordSecretVersion
		}
		connector["trusted_certificate_secret_id"] = fss.Str(ldap.TrustedCertificateSecretId)
		if ldap.TrustedCertificateSecretVersion != nil {
			connector["trusted_certificate_secret_version"] = *ldap.TrustedCertificateSecretVersion
		}
	}

	return fss.Result(
		fmt.Sprintf("Outbound connector %q is %s", connector["display_name"], connector["lifecycle_state"]),
		map[string]interface{}{
			"outbound_connector": connector,
			"id":                 connector["id"],
			"lifecycle_state":    connector["lifecycle_state"],
		},
	), nil
}
