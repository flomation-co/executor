// Package oracle_dns_view_change_compartment moves a private-DNS view into a
// different compartment. Asynchronous — returns a work-request id when the service
// supplies one.
package oracle_dns_view_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Change View Compartment"
	Description  = "Move an Oracle Cloud private-DNS view into a different compartment. Asynchronous — returns a work-request id; poll Get View to confirm the new compartment."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+layer-group"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the view picker)"},
	{Name: "view_ocid", Type: core.ConnectionTypeString, Label: "View OCID", Placeholder: "ocid1.dnsview.oc1..aaaa… — the view to move", Required: true},
	{Name: "target_compartment_ocid", Type: core.ConnectionTypeString, Label: "Target Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… — the compartment to move the view into", Required: true},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL (public) or PRIVATE — views are private DNS", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "View OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := dnsn.ResourceClient(inputs, "view_ocid")
	if errResult != nil {
		return errResult, nil
	}
	target, err := dnsn.RequiredString("target_compartment_ocid", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	req := dns.ChangeViewCompartmentRequest{
		ViewId:                       &id,
		ChangeViewCompartmentDetails: dns.ChangeViewCompartmentDetails{CompartmentId: &target},
	}
	if scope != "" {
		req.Scope = dns.ChangeViewCompartmentScopeEnum(scope)
	}
	resp, err := client.ChangeViewCompartment(dnsn.Context(), req)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	result := dnsn.AsyncResult(fmt.Sprintf("Move requested for view %s to compartment %s", id, target), dnsn.Str(resp.OpcWorkRequestId))
	result["id"] = id
	return result, nil
}
