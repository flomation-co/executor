// Package oracle_dns_tsig_key_update updates the tags on an existing DNS TSIG key.
package oracle_dns_tsig_key_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Update TSIG Key"
	Description  = "Update the free-form tags on an existing Oracle Cloud DNS TSIG key. Tags left blank are preserved."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+key"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the TSIG key picker)"},
	{Name: "tsig_key_ocid", Type: core.ConnectionTypeString, Label: "TSIG Key OCID", Placeholder: "ocid1.dns-tsig-key.oc1..aaaa… — the key to update", Required: true},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Free-form Tags (JSON)", Placeholder: "JSON object, e.g. {\"env\":\"prod\"} — leave blank to keep the current tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "tsig_key", Type: core.ConnectionTypeObject, Label: "TSIG Key"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "TSIG Key OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := dnsn.ResourceClient(inputs, "tsig_key_ocid")
	if errResult != nil {
		return errResult, nil
	}
	// Optional tags input: when omitted the current tags are preserved (blank != wipe).
	tags, err := dnsn.FreeformTags("tags", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}

	// READ-MODIFY-WRITE: UpdateTsigKeyDetails is a full-replace PUT, so seed it from the
	// current resource and overlay only what the operator supplied.
	current, err := client.GetTsigKey(dnsn.Context(), dns.GetTsigKeyRequest{TsigKeyId: &id})
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	details := dns.UpdateTsigKeyDetails{
		FreeformTags: current.TsigKey.FreeformTags,
		DefinedTags:  current.TsigKey.DefinedTags,
	}
	if tags != nil {
		details.FreeformTags = tags
	}

	resp, err := client.UpdateTsigKey(dnsn.Context(), dns.UpdateTsigKeyRequest{
		TsigKeyId:            &id,
		UpdateTsigKeyDetails: details,
	})
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	key := dnsn.SummariseTsigKey(&resp.TsigKey)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Updated TSIG key %q", key["name"]),
		"tsig_key":    key,
		"id":          key["id"],
		"success":     true,
	}, nil
}
