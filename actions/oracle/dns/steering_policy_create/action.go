// Package oracle_dns_steering_policy_create creates a traffic-management steering
// policy (failover, load-balance, geo/ASN/IP routing). Synchronous.
package oracle_dns_steering_policy_create

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
	Name         = "OCI DNS: Create Steering Policy"
	Description  = "Create a traffic-management steering policy (failover, load-balance, or geo/ASN/IP routing) in a compartment. Attach it to a domain with Create Steering Policy Attachment."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+diagram-project"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name for the policy", Required: true},
	{Name: "template", Type: core.ConnectionTypeString, Label: "Template", Placeholder: "The policy behaviour", Required: true, Options: []core.ConnectionOption{
		{Name: "Failover", Value: "FAILOVER"},
		{Name: "Load Balance", Value: "LOAD_BALANCE"},
		{Name: "Route by Geo", Value: "ROUTE_BY_GEO"},
		{Name: "Route by ASN", Value: "ROUTE_BY_ASN"},
		{Name: "Route by IP", Value: "ROUTE_BY_IP"},
		{Name: "Custom", Value: "CUSTOM"},
	}},
	{Name: "ttl", Type: core.ConnectionTypeString, Label: "TTL (seconds)", Placeholder: "The record TTL the policy serves, e.g. 30 (optional)"},
	{Name: "answers_json", Type: core.ConnectionTypeText, Label: "Answers (JSON array)", Placeholder: `[{"name":"web-1","rtype":"A","rdata":"10.0.0.5","pool":"primary"}] — the candidate records (optional)`},
	{Name: "rules_json", Type: core.ConnectionTypeText, Label: "Rules (JSON array)", Placeholder: `The template's rules in order, e.g. [{"ruleType":"FILTER",…},{"ruleType":"WEIGHTED",…},{"ruleType":"LIMIT","defaultCount":1}] — required for templated policies (FAILOVER/LOAD_BALANCE/ROUTE_BY_*); optional for CUSTOM`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "steering_policy", Type: core.ConnectionTypeObject, Label: "Steering Policy"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Steering Policy OCID"},
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
	displayName, err := dnsn.RequiredString("display_name", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	template, err := dnsn.RequiredString("template", inputs)
	if err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	}
	tmpl := dns.CreateSteeringPolicyDetailsTemplateEnum(strings.ToUpper(template))
	if _, ok := dns.GetMappingCreateSteeringPolicyDetailsTemplateEnum(string(tmpl)); !ok {
		return dnsn.ErrorResult("template must be one of: FAILOVER, LOAD_BALANCE, ROUTE_BY_GEO, ROUTE_BY_ASN, ROUTE_BY_IP, CUSTOM"), nil
	}
	details := dns.CreateSteeringPolicyDetails{CompartmentId: &compartment, DisplayName: &displayName, Template: tmpl}
	if v, ok, err := dnsn.OptionalInt("ttl", inputs); err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	} else if ok {
		details.Ttl = &v
	}
	// Templated policies (FAILOVER/LOAD_BALANCE/ROUTE_BY_*) require their answers +
	// template-ordered rules; only CUSTOM is valid bare. Both are optional inputs.
	if raw := strings.TrimSpace(dnsn.OptionalString("answers_json", inputs)); raw != "" {
		var answers []dns.SteeringPolicyAnswer
		if err := json.Unmarshal([]byte(raw), &answers); err != nil {
			return dnsn.ErrorResult(fmt.Sprintf(`answers must be a JSON array of steering-policy answers, e.g. [{"name":"web-1","rtype":"A","rdata":"10.0.0.5"}]: %s`, err.Error())), nil
		}
		details.Answers = answers
	}
	if raw := strings.TrimSpace(dnsn.OptionalString("rules_json", inputs)); raw != "" {
		// SteeringPolicyRule is a polymorphic interface — decode via the SDK's own
		// CreateSteeringPolicyDetails.UnmarshalJSON by wrapping the raw rules array.
		body, err := json.Marshal(map[string]json.RawMessage{"rules": json.RawMessage(raw)})
		if err != nil {
			return dnsn.ErrorResult(fmt.Sprintf(`rules must be a JSON array of steering-policy rules, e.g. [{"ruleType":"LIMIT","defaultCount":1}]: %s`, err.Error())), nil
		}
		var decoded dns.CreateSteeringPolicyDetails
		if err := json.Unmarshal(body, &decoded); err != nil {
			return dnsn.ErrorResult(fmt.Sprintf(`rules must be a JSON array of steering-policy rules, e.g. [{"ruleType":"LIMIT","defaultCount":1}]: %s`, err.Error())), nil
		}
		details.Rules = decoded.Rules
	}
	resp, err := client.CreateSteeringPolicy(dnsn.Context(), dns.CreateSteeringPolicyRequest{CreateSteeringPolicyDetails: details})
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	policy := dnsn.SummariseSteeringPolicy(&resp.SteeringPolicy)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created steering policy %q (%s)", displayName, template),
		"steering_policy": policy,
		"id":              policy["id"],
		"success":         true,
	}, nil
}
