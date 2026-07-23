// Package oracle_dns_resolver_update updates a private-DNS resolver's display name,
// freeform tags, attached views and forwarding rules. UpdateResolver is a full-replace
// PUT, so this action reads the resolver first and RE-SENDS its current attached views
// and rules — the highest-data-loss operation in the DNS node — overwriting only the
// fields the operator supplies.
package oracle_dns_resolver_update

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	dnsn "flomation.app/automate/executor/actions/oracle/dns"

	dns "github.com/oracle/oci-go-sdk/v65/dns"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI DNS: Update Resolver"
	Description  = "Update a private-DNS resolver's display name, freeform tags, attached views and forwarding rules. Reads the resolver first and RE-SENDS its current attached views and rules so they are preserved — the update is a full-replace PUT. Attached views/rules are only changed when you supply attached_views_json/rules_json; everything else is left blank to keep the current value."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the resolver picker)"},
	{Name: "resolver_ocid", Type: core.ConnectionTypeString, Label: "Resolver OCID", Placeholder: "ocid1.dns-resolver.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New display name — leave blank to keep the current one"},
	{Name: "tags", Type: core.ConnectionTypeText, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} — REPLACES the resolver's freeform tags; leave blank to keep the current ones`},
	{Name: "attached_views_json", Type: core.ConnectionTypeText, Label: "Attached Views (JSON array)", Placeholder: `[{"viewId":"ocid1.dnsview.oc1..aaaa…"}] — REPLACES the attached views; leave blank to keep the current ones`},
	{Name: "rules_json", Type: core.ConnectionTypeText, Label: "Rules (JSON array)", Placeholder: `[{"action":"FORWARD","destinationAddresses":["10.0.0.2"],"sourceEndpointName":"ep1","qnameCoverConditions":["internal.example.com"]}] — REPLACES the forwarding rules; leave blank to keep the current ones`},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Placeholder: "GLOBAL or PRIVATE (optional)", Options: []core.ConnectionOption{
		{Name: "Global (public)", Value: "GLOBAL"},
		{Name: "Private", Value: "PRIVATE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "resolver", Type: core.ConnectionTypeObject, Label: "Resolver"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Resolver OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := dnsn.ResourceClient(inputs, "resolver_ocid")
	if errResult != nil {
		return errResult, nil
	}
	scope, err := dnsn.OptionalScope(inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}

	// READ: fetch the resolver so we can re-send its current attached views / rules
	// rather than wiping them (UpdateResolver is a full-replace PUT).
	getReq := dns.GetResolverRequest{ResolverId: &id}
	if scope != "" {
		getReq.Scope = dns.GetResolverScopeEnum(scope)
	}
	getResp, err := client.GetResolver(dnsn.Context(), getReq)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	current := getResp.Resolver

	// MODIFY: seed the update body from the current values so nothing is lost.
	attached := make([]dns.AttachedViewDetails, 0, len(current.AttachedViews))
	for _, v := range current.AttachedViews {
		attached = append(attached, dns.AttachedViewDetails{ViewId: v.ViewId})
	}
	details := dns.UpdateResolverDetails{
		DisplayName:   current.DisplayName,
		FreeformTags:  current.FreeformTags,
		DefinedTags:   current.DefinedTags,
		AttachedViews: attached,
	}

	// Rules: preserve the current rules unless the operator replaces them via rules_json.
	if raw := strings.TrimSpace(dnsn.OptionalString("rules_json", inputs)); raw != "" {
		body, mErr := json.Marshal(map[string]json.RawMessage{"rules": json.RawMessage(raw)})
		if mErr != nil {
			return dnsn.ErrorResult(`rules must be a JSON array of resolver rule objects, e.g. [{"action":"FORWARD","destinationAddresses":["10.0.0.2"],"sourceEndpointName":"ep1"}]: ` + mErr.Error()), nil
		}
		var wrapper dns.UpdateResolverDetails
		if uErr := json.Unmarshal(body, &wrapper); uErr != nil {
			return dnsn.ErrorResult(`rules must be a JSON array of resolver rule objects, e.g. [{"action":"FORWARD","destinationAddresses":["10.0.0.2"],"sourceEndpointName":"ep1"}]: ` + uErr.Error()), nil
		}
		details.Rules = wrapper.Rules
	} else {
		seeded, sErr := currentRulesToDetails(current.Rules)
		if sErr != nil {
			return dnsn.ErrorResult(sErr.Error()), nil
		}
		details.Rules = seeded
	}

	// Overlay only the operator-supplied inputs.
	if dn := strings.TrimSpace(dnsn.OptionalString("display_name", inputs)); dn != "" {
		details.DisplayName = &dn
	}
	if dnsn.BoolWasSet("tags", inputs) {
		tags, tErr := dnsn.FreeformTags("tags", inputs)
		if tErr != nil {
			return dnsn.ErrorResult(tErr.Error()), nil
		}
		details.FreeformTags = tags
	}
	if raw := strings.TrimSpace(dnsn.OptionalString("attached_views_json", inputs)); raw != "" {
		var views []dns.AttachedViewDetails
		if vErr := json.Unmarshal([]byte(raw), &views); vErr != nil {
			return dnsn.ErrorResult(`attached views must be a JSON array of {"viewId":"ocid1.dnsview.oc1..aaaa…"} objects: ` + vErr.Error()), nil
		}
		details.AttachedViews = views
	}

	// WRITE.
	req := dns.UpdateResolverRequest{ResolverId: &id, UpdateResolverDetails: details}
	if scope != "" {
		req.Scope = dns.UpdateResolverScopeEnum(scope)
	}
	resp, err := client.UpdateResolver(dnsn.Context(), req)
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	resolver := dnsn.SummariseResolver(&resp.Resolver)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updated resolver %q", resolver["display_name"]),
		"resolver":        resolver,
		"id":              resolver["id"],
		"work_request_id": dnsn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}

// currentRulesToDetails converts a resolver's live rules (the read-only ResolverRule
// interface) into the ResolverRuleDetails form the update body carries, so an update
// that does not touch the rules re-sends them unchanged instead of wiping them. FORWARD
// is the only rule action the DNS API defines; an unconvertible rule is reported so the
// operator can re-supply the full rule set via rules_json rather than lose it silently.
func currentRulesToDetails(rules []dns.ResolverRule) ([]dns.ResolverRuleDetails, error) {
	if len(rules) == 0 {
		return nil, nil
	}
	out := make([]dns.ResolverRuleDetails, 0, len(rules))
	for _, r := range rules {
		fr, ok := r.(dns.ResolverForwardRule)
		if !ok {
			return nil, fmt.Errorf("this resolver has a rule of a type this action cannot preserve automatically — re-supply the full rule set via 'rules_json' to update it safely")
		}
		out = append(out, dns.ResolverForwardRuleDetails{
			DestinationAddresses:    fr.DestinationAddresses,
			SourceEndpointName:      fr.SourceEndpointName,
			ClientAddressConditions: fr.ClientAddressConditions,
			QnameCoverConditions:    fr.QnameCoverConditions,
		})
	}
	return out, nil
}
