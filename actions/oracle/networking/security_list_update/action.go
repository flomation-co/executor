// Package oracle_networking_security_list_update updates the editable attributes of a
// security list — its display name and its ingress / egress rule sets.
package oracle_networking_security_list_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	net "flomation.app/automate/executor/actions/oracle/networking"

	ocicore "github.com/oracle/oci-go-sdk/v65/core"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Networking: Update Security list"
	Description  = "Update editable attributes of an Oracle Cloud security list."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+pen"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "security_list_ocid", Type: core.ConnectionTypeString, Label: "Security List OCID", Placeholder: "ocid1.securitylist.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name (optional)"},
	{Name: "ingress_security_rules", Type: core.ConnectionTypeText, Label: "Ingress Rules (JSON)", Placeholder: `Replaces ALL ingress rules, e.g. [{"protocol":"6","source":"0.0.0.0/0","tcpOptions":{"destinationPortRange":{"min":443,"max":443}}}]`},
	{Name: "egress_security_rules", Type: core.ConnectionTypeText, Label: "Egress Rules (JSON)", Placeholder: `Replaces ALL egress rules, e.g. [{"protocol":"all","destination":"0.0.0.0/0"}]`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "security_list", Type: core.ConnectionTypeObject, Label: "Security List"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := net.NetworkResourceClient(inputs, "security_list_ocid")
	if errResult != nil {
		return errResult, nil
	}
	details := ocicore.UpdateSecurityListDetails{}
	if v := strings.TrimSpace(net.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	// REPLACE semantics: the arrays overwrite the whole rule set, so only send
	// them when the operator actually supplied JSON.
	ingress, err := net.DecodeIngressRules("ingress_security_rules", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	if ingress != nil {
		details.IngressSecurityRules = ingress
	}
	egress, err := net.DecodeEgressRules("egress_security_rules", inputs)
	if err != nil {
		return net.ErrorResult(err.Error()), nil
	}
	if egress != nil {
		details.EgressSecurityRules = egress
	}
	resp, err := client.UpdateSecurityList(net.Context(), ocicore.UpdateSecurityListRequest{
		SecurityListId:            &id,
		UpdateSecurityListDetails: details,
	})
	if err != nil {
		return net.ErrorResult(auth.OCIError(err)), nil
	}
	sl := net.SummariseSecurityList(&resp.SecurityList)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updated security list %q (%s)", sl["display_name"], sl["lifecycle_state"]),
		"security_list":   sl,
		"lifecycle_state": sl["lifecycle_state"],
		"success":         true,
	}, nil
}
