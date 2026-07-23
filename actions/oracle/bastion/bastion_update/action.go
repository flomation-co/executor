// Package oracle_bastion_bastion_update applies a partial update to a bastion: only the maximum
// session time-to-live and the client CIDR allow-list you supply are changed; blank fields are left
// unchanged. Asynchronous — returns a work-request id you can poll for completion.
package oracle_bastion_bastion_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	bas "flomation.app/automate/executor/actions/oracle/bastion"

	"github.com/oracle/oci-go-sdk/v65/bastion"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Bastion: Update Bastion"
	Description  = "Partially update a bastion — change only the maximum session time-to-live or the client CIDR allow-list you supply; blank fields are left unchanged. Returns a work-request id to poll."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+terminal"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "bastion_ocid", Type: core.ConnectionTypeString, Label: "Bastion OCID", Placeholder: "ocid1.bastion.oc1..aaaa… — the bastion to update", Required: true},
	{Name: "max_session_ttl_seconds", Type: core.ConnectionTypeString, Label: "Max Session TTL (seconds)", Placeholder: "New maximum session lifetime, e.g. 10800 (leave blank to keep unchanged)"},
	{Name: "client_cidr_allow_list", Type: core.ConnectionTypeText, Label: "Client CIDR Allow-list", Placeholder: "Comma- or newline-separated CIDR ranges, e.g. 10.0.0.0/16 (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Bastion OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := bas.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	bastionID, err := bas.RequiredString("bastion_ocid", inputs)
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the fields the operator actually supplied. A nil *int / nil slice
	// leaves the corresponding field unchanged on the bastion.
	details := bastion.UpdateBastionDetails{}
	ttl, err := bas.OptionalInt("max_session_ttl_seconds", inputs)
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}
	details.MaxSessionTtlInSeconds = ttl

	if raw := strings.TrimSpace(bas.OptionalString("client_cidr_allow_list", inputs)); raw != "" {
		var cidrs []string
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
			if p := strings.TrimSpace(part); p != "" {
				cidrs = append(cidrs, p)
			}
		}
		details.ClientCidrBlockAllowList = cidrs
	}

	resp, err := client.UpdateBastion(bas.Context(), bastion.UpdateBastionRequest{
		BastionId:            &bastionID,
		UpdateBastionDetails: details,
	})
	if err != nil {
		return bas.ErrorResult(auth.OCIError(err)), nil
	}
	return bas.Result(fmt.Sprintf("Updating bastion %s — poll the work request for completion", bastionID), map[string]interface{}{
		"id":              bastionID,
		"work_request_id": bas.Str(resp.OpcWorkRequestId),
	}), nil
}
