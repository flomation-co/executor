// Package oracle_filestorage_outbound_connector_create creates an outbound connector —
// the LDAP bind account File Storage uses to reach an external directory server, so mount
// targets can map NFS UIDs/GIDs to LDAP identities. Only the LDAPBIND connector type exists.
// The call returns the connector in a CREATING state; poll Get Outbound Connector until ACTIVE.
package oracle_filestorage_outbound_connector_create

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	fss "flomation.app/automate/executor/actions/oracle/filestorage"

	filestorage "github.com/oracle/oci-go-sdk/v65/filestorage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI File Storage: Create Outbound Connector"
	Description  = "Create an Oracle Cloud File Storage outbound connector — the LDAP bind account mount targets use to reach an external LDAP directory. Returns the OCID immediately in a CREATING state; poll Get Outbound Connector until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+server"
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
	{Name: "availability_domain", Type: core.ConnectionTypeString, Label: "Availability Domain", Placeholder: "e.g. Uocm:UK-LONDON-1-AD-1", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A friendly name for the outbound connector", Required: true},
	{Name: "endpoints_json", Type: core.ConnectionTypeText, Label: "LDAP Server Endpoints (JSON array)", Placeholder: `[{"hostname":"ldap.example.com","port":389}] — one entry per DNS server/port`, Required: true},
	{Name: "bind_distinguished_name", Type: core.ConnectionTypeString, Label: "Bind Distinguished Name", Placeholder: "e.g. cn=admin,dc=example,dc=com", Required: true},
	{Name: "password_secret_ocid", Type: core.ConnectionTypeString, Label: "Password Secret OCID", Placeholder: "ocid1.vaultsecret.oc1..aaaa… of the bind password in the Vault (optional)"},
	{Name: "password_secret_version", Type: core.ConnectionTypeString, Label: "Password Secret Version", Placeholder: "Version of the password secret to use (optional, whole number)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
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
	auth, client, errResult := fss.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	ad, err := fss.RequiredAvailabilityDomain(inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	displayName, err := fss.RequiredString("display_name", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	bindDN, err := fss.RequiredString("bind_distinguished_name", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	raw, err := fss.RequiredString("endpoints_json", inputs)
	if err != nil {
		return fss.ErrorResult(err.Error()), nil
	}
	var endpoints []filestorage.Endpoint
	if err := json.Unmarshal([]byte(raw), &endpoints); err != nil {
		return fss.ErrorResult(fmt.Sprintf(`LDAP server endpoints must be a JSON array of {"hostname","port"} objects, e.g. [{"hostname":"ldap.example.com","port":389}]: %s`, err.Error())), nil
	}
	if len(endpoints) == 0 {
		return fss.ErrorResult("at least one LDAP server endpoint is required"), nil
	}

	details := filestorage.CreateLdapBindAccountDetails{
		CompartmentId:         &compartment,
		AvailabilityDomain:    &ad,
		DisplayName:           &displayName,
		BindDistinguishedName: &bindDN,
		Endpoints:             endpoints,
	}
	if v := strings.TrimSpace(fss.OptionalString("password_secret_ocid", inputs)); v != "" {
		details.PasswordSecretId = &v
	}
	if n, ok, err := fss.OptionalInt("password_secret_version", inputs); err != nil {
		return fss.ErrorResult(err.Error()), nil
	} else if ok {
		details.PasswordSecretVersion = &n
	}
	if tags, err := fss.FreeformTags("tags", inputs); err != nil {
		return fss.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}

	resp, err := client.CreateOutboundConnector(fss.Context(), filestorage.CreateOutboundConnectorRequest{CreateOutboundConnectorDetails: details})
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
		"time_created":        fss.FormatTime(oc.GetTimeCreated()),
	}
	// LDAPBIND is the only connector type — surface its distinctive fields when present.
	if ldap, ok := oc.(filestorage.LdapBindAccount); ok {
		connector["bind_distinguished_name"] = fss.Str(ldap.BindDistinguishedName)
		connector["password_secret_id"] = fss.Str(ldap.PasswordSecretId)
		hosts := make([]map[string]interface{}, 0, len(ldap.Endpoints))
		for i := range ldap.Endpoints {
			hosts = append(hosts, map[string]interface{}{
				"hostname": fss.Str(ldap.Endpoints[i].Hostname),
				"port":     fss.Int64OrNil(ldap.Endpoints[i].Port),
			})
		}
		connector["endpoints"] = hosts
	}

	return fss.Result(fmt.Sprintf("Creating outbound connector %q (%s) — poll Get Outbound Connector until ACTIVE", displayName, connector["lifecycle_state"]), map[string]interface{}{
		"outbound_connector": connector, "id": connector["id"], "lifecycle_state": connector["lifecycle_state"],
	}), nil
}
