// Package oracle_dns_steering_policy_update updates a traffic-management steering
// policy, preserving every collection the policy currently carries. Synchronous.
package oracle_dns_steering_policy_update

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
	Name         = "OCI DNS: Update Steering Policy"
	Description  = "Update a traffic-management steering policy. Reads the policy first and re-sends its current display name, TTL, template, answers and rules, overwriting only the fields you supply — the answers/rules collections are preserved when their JSON inputs are left blank (UpdateSteeringPolicy is a full-replace PUT)."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the steering-policy picker)"},
	{Name: "steering_policy_ocid", Type: core.ConnectionTypeString, Label: "Steering Policy OCID", Placeholder: "ocid1.dnssteeringpolicy.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "New friendly name (optional — kept if blank)"},
	{Name: "ttl", Type: core.ConnectionTypeString, Label: "TTL (seconds)", Placeholder: "New record TTL, e.g. 30 (optional — kept if blank)"},
	{Name: "template", Type: core.ConnectionTypeString, Label: "Template", Placeholder: "New policy behaviour (optional — kept if blank)", Options: []core.ConnectionOption{
		{Name: "Failover", Value: "FAILOVER"},
		{Name: "Load Balance", Value: "LOAD_BALANCE"},
		{Name: "Route by Geo", Value: "ROUTE_BY_GEO"},
		{Name: "Route by ASN", Value: "ROUTE_BY_ASN"},
		{Name: "Route by IP", Value: "ROUTE_BY_IP"},
		{Name: "Custom", Value: "CUSTOM"},
	}},
	{Name: "answers_json", Type: core.ConnectionTypeText, Label: "Answers (JSON array)", Placeholder: `[{"name":"web-1","rtype":"A","rdata":"10.0.0.5","pool":"primary","isDisabled":false}] — REPLACES every answer; leave blank to keep the current ones`},
	{Name: "rules_json", Type: core.ConnectionTypeText, Label: "Rules (JSON array)", Placeholder: `[{"ruleType":"FILTER","defaultAnswerData":[{"shouldKeep":true,"answerCondition":"answer.isDisabled != true"}]},{"ruleType":"LIMIT","defaultCount":1}] — each rule needs ruleType; REPLACES every rule; leave blank to keep the current ones`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "steering_policy", Type: core.ConnectionTypeObject, Label: "Steering Policy"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Steering Policy OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := dnsn.ResourceClient(inputs, "steering_policy_ocid")
	if errResult != nil {
		return errResult, nil
	}

	// READ: fetch the policy so we can re-send its current collections rather than
	// wiping them (UpdateSteeringPolicy is a full-replace PUT).
	getResp, err := client.GetSteeringPolicy(dnsn.Context(), dns.GetSteeringPolicyRequest{SteeringPolicyId: &id})
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	current := getResp.SteeringPolicy

	// MODIFY: seed the update body from the current values.
	details := dns.UpdateSteeringPolicyDetails{
		DisplayName:          current.DisplayName,
		Ttl:                  current.Ttl,
		HealthCheckMonitorId: current.HealthCheckMonitorId,
		Template:             dns.UpdateSteeringPolicyDetailsTemplateEnum(string(current.Template)),
		FreeformTags:         current.FreeformTags,
		DefinedTags:          current.DefinedTags,
		Answers:              current.Answers,
		Rules:                current.Rules,
	}

	// Overlay only the operator-supplied inputs.
	if dn := strings.TrimSpace(dnsn.OptionalString("display_name", inputs)); dn != "" {
		details.DisplayName = &dn
	}
	if v, ok, err := dnsn.OptionalInt("ttl", inputs); err != nil {
		return dnsn.ErrorResult(err.Error()), nil
	} else if ok {
		details.Ttl = &v
	}
	if t := strings.TrimSpace(dnsn.OptionalString("template", inputs)); t != "" {
		tmpl := dns.UpdateSteeringPolicyDetailsTemplateEnum(strings.ToUpper(t))
		if _, ok := dns.GetMappingUpdateSteeringPolicyDetailsTemplateEnum(string(tmpl)); !ok {
			return dnsn.ErrorResult("template must be one of: FAILOVER, LOAD_BALANCE, ROUTE_BY_GEO, ROUTE_BY_ASN, ROUTE_BY_IP, CUSTOM"), nil
		}
		details.Template = tmpl
	}
	if raw := strings.TrimSpace(dnsn.OptionalString("answers_json", inputs)); raw != "" {
		var answers []dns.SteeringPolicyAnswer
		if err := json.Unmarshal([]byte(raw), &answers); err != nil {
			return dnsn.ErrorResult(fmt.Sprintf(`answers must be a JSON array of steering-policy answers, e.g. [{"name":"web-1","rtype":"A","rdata":"10.0.0.5"}]: %s`, err.Error())), nil
		}
		details.Answers = answers
	}
	if raw := strings.TrimSpace(dnsn.OptionalString("rules_json", inputs)); raw != "" {
		// SteeringPolicyRule is a polymorphic interface — decode it through the SDK's
		// own UpdateSteeringPolicyDetails.UnmarshalJSON by wrapping the raw array.
		wrapper := struct {
			Rules json.RawMessage `json:"rules"`
		}{Rules: json.RawMessage(raw)}
		body, err := json.Marshal(wrapper)
		if err != nil {
			return dnsn.ErrorResult(fmt.Sprintf(`rules must be a JSON array of steering-policy rules, e.g. [{"ruleType":"LIMIT","defaultCount":1}]: %s`, err.Error())), nil
		}
		var decoded dns.UpdateSteeringPolicyDetails
		if err := json.Unmarshal(body, &decoded); err != nil {
			return dnsn.ErrorResult(fmt.Sprintf(`rules must be a JSON array of steering-policy rules, e.g. [{"ruleType":"LIMIT","defaultCount":1}]: %s`, err.Error())), nil
		}
		details.Rules = decoded.Rules
	}

	// WRITE.
	resp, err := client.UpdateSteeringPolicy(dnsn.Context(), dns.UpdateSteeringPolicyRequest{
		SteeringPolicyId:            &id,
		UpdateSteeringPolicyDetails: details,
	})
	if err != nil {
		return dnsn.ErrorResult(auth.OCIError(err)), nil
	}
	policy := dnsn.SummariseSteeringPolicy(&resp.SteeringPolicy)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updated steering policy %q (%s)", policy["display_name"], policy["template"]),
		"steering_policy": policy,
		"id":              policy["id"],
		"success":         true,
	}, nil
}
