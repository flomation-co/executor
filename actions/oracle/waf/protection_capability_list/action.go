// Package oracle_waf_protection_capability_list lists the OCI-managed protection capabilities
// available to WAF policies in a compartment, optionally filtered by unique key or capability type.
// Walks pagination up to a safe cap.
package oracle_waf_protection_capability_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	wf "flomation.app/automate/executor/actions/oracle/waf"

	"github.com/oracle/oci-go-sdk/v65/waf"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI WAF: List Protection Capabilities"
	Description  = "List the OCI-managed protection capabilities available to WAF policies in a compartment. Optionally filter by unique key or capability type. Walks pagination up to a safe cap."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-virus"
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
	{Name: "key", Type: core.ConnectionTypeString, Label: "Capability Key Filter", Placeholder: "Only the capability with this unique key, e.g. 920320 (optional)"},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Capability Type", Placeholder: "All types when unset (optional)", Options: []core.ConnectionOption{
		{Name: "Request Protection", Value: "REQUEST_PROTECTION_CAPABILITY"},
		{Name: "Response Protection", Value: "RESPONSE_PROTECTION_CAPABILITY"},
	}},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Page Size", Placeholder: "Items per page (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "capabilities", Type: core.ConnectionTypeObject, Label: "Capabilities"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := wf.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return wf.ErrorResult(err.Error()), nil
	}
	req := waf.ListProtectionCapabilitiesRequest{CompartmentId: &compartment}
	if key := wf.OptionalString("key", inputs); key != "" {
		req.Key = &key
	}
	if t := wf.OptionalString("type", inputs); t != "" {
		req.Type = waf.ProtectionCapabilitySummaryTypeEnum(t)
	}
	limit, ok, err := wf.OptionalInt("limit", inputs)
	if err != nil {
		return wf.ErrorResult(err.Error()), nil
	}
	if ok {
		req.Limit = &limit
	}

	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= wf.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListProtectionCapabilities(wf.Context(), req)
		if err != nil {
			return wf.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			c := resp.Items[i]
			out = append(out, map[string]interface{}{
				"key":               wf.Str(c.Key),
				"display_name":      wf.Str(c.DisplayName),
				"version":           wf.IntOrNilVal(c.Version),
				"type":              string(c.Type),
				"is_latest_version": wf.Bool(c.IsLatestVersion),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return wf.Result(fmt.Sprintf("Found %d protection capability(ies)", len(out)), map[string]interface{}{
		"capabilities": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
