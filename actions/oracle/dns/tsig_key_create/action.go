// Package oracle_dns_tsig_key_create creates a TSIG key used to authenticate zone
// transfers to/from external DNS servers. Synchronous.
package oracle_dns_tsig_key_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Create TSIG Key"
	Description  = "Create a TSIG key in a compartment, used to authenticate zone transfers to/from external DNS servers."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+key"
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
	{Name: "tsig_key_name", Type: core.ConnectionTypeString, Label: "TSIG Key Name", Placeholder: "The key name, e.g. transfer-key", Required: true},
	{Name: "algorithm", Type: core.ConnectionTypeString, Label: "Algorithm", Placeholder: "The TSIG algorithm, e.g. hmac-sha256", Required: true},
	{Name: "secret", Type: core.ConnectionTypeSecret, Label: "Secret", Placeholder: "The base64-encoded shared secret (kept as a secret)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "tsig_key", Type: core.ConnectionTypeObject, Label: "TSIG Key"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "TSIG Key OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := dnsn.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	name, err := dnsn.RequiredString("tsig_key_name", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	algorithm, err := dnsn.RequiredString("algorithm", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	secret, err := dnsn.RequiredString("secret", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	resp, err := client.CreateTsigKey(dnsn.Context(), dns.CreateTsigKeyRequest{
		CreateTsigKeyDetails: dns.CreateTsigKeyDetails{
			CompartmentId: &compartment,
			Name:          &name,
			Algorithm:     &algorithm,
			Secret:        &secret,
		},
	})
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	key := dnsn.SummariseTsigKey(&resp.TsigKey)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created TSIG key %q (%s)", name, algorithm),
		"tsig_key":    key,
		"id":          key["id"],
		"success":     true,
	}, nil
}
