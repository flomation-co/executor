// Package oracle_dns_view_update renames a private-DNS view and/or replaces its
// free-form tags. UpdateView is a full-replace PUT, so this reads the current view
// first and overlays only the operator-supplied fields (read-modify-write) to avoid
// wiping the display name, tags or defined tags. Synchronous.
package oracle_dns_view_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Update View"
	Description  = "Rename an Oracle Cloud private-DNS view and/or replace its free-form tags. Reads the current view first so unset fields are preserved."
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
	{Name: "view_ocid", Type: core.ConnectionTypeString, Label: "View OCID", Placeholder: "ocid1.dnsview.oc1..aaaa… — the view to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name (leave blank to keep the current name)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Free-form Tags (JSON)", Placeholder: "{\"env\":\"prod\"} — replaces all free-form tags; leave blank to keep the current ones"},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL (public) or PRIVATE — only needed for private views", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "view", Type: core.ConnectionTypeObject, Label: "View"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "View OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := dnsn.ResourceClient(inputs, "view_ocid")
	if errResult != nil {
		return errResult, nil
	}
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}

	// READ: fetch the current view so we can preserve fields the operator leaves alone.
	getReq := dns.GetViewRequest{ViewId: &id}
	if scope != "" {
		getReq.Scope = dns.GetViewScopeEnum(scope)
	}
	getResp, err := client.GetView(dnsn.Context(), getReq)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	current := getResp.View

	// MODIFY: seed the full-replace body from the current values, then overlay inputs.
	details := dns.UpdateViewDetails{
		DisplayName:  current.DisplayName,
		FreeformTags: current.FreeformTags,
		DefinedTags:  current.DefinedTags,
	}
	if dn := strings.TrimSpace(dnsn.OptionalString("display_name", inputs)); dn != "" {
		details.DisplayName = &dn
	}
	if strings.TrimSpace(dnsn.OptionalString("tags", inputs)) != "" {
		tags, err := dnsn.FreeformTags("tags", inputs)
		if err != nil {
			return dnsn.ErrorResult(err.Error()), nil
		}
		details.FreeformTags = tags
	}

	// WRITE.
	updReq := dns.UpdateViewRequest{ViewId: &id, UpdateViewDetails: details}
	if scope != "" {
		updReq.Scope = dns.UpdateViewScopeEnum(scope)
	}
	resp, err := client.UpdateView(dnsn.Context(), updReq)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	view := dnsn.SummariseView(&resp.View)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Updated view %q", view["display_name"]),
		"view":        view,
		"id":          view["id"],
		"success":     true,
	}, nil
}
