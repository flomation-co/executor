// Package get_credits reports the HeyGen account's remaining credit balance and
// plan — a useful pre-flight guardrail before spending credits on a video.
package get_credits

import (
	"fmt"

	core "flomation.app/automate/executor"
	heygen "flomation.app/automate/executor/actions/heygen"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Credit Balance"
	Description  = "Get the HeyGen account's remaining credits and plan."
	Website      = "https://www.flomation.co"
	Icon         = "gauge"
	Date         = "11/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HeyGen API Key", Placeholder: "${secrets.HeyGenApiKey}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "remaining_credits", Type: core.ConnectionTypeString, Label: "Remaining Credits"},
	{Name: "plan", Type: core.ConnectionTypeString, Label: "Plan"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := heygen.GetAPIKey(inputs)
	if err != nil {
		return heygen.ErrorResult(err.Error()), nil
	}

	resp, err := heygen.NewClient(apiKey).Get(flow, "/v3/users/me", nil)
	if err != nil {
		return heygen.MapError(err), nil
	}

	data := heygen.DataObj(resp)
	credits := creditsFrom(data)
	plan := heygen.Str(data, "plan")
	if plan == "" {
		plan = heygen.Str(data, "plan_name")
	}

	summary := fmt.Sprintf("HeyGen credits remaining: %s", credits)
	if plan != "" {
		summary += fmt.Sprintf(" (plan: %s)", plan)
	}
	return heygen.Result(summary, map[string]interface{}{
		"remaining_credits": credits,
		"plan":              plan,
	}), nil
}

// creditsFrom reads the credit balance across the field names HeyGen has used
// (remaining_quota / remaining_credits / credits), numeric or string.
func creditsFrom(data map[string]interface{}) string {
	if data == nil {
		return ""
	}
	for _, k := range []string{"remaining_credits", "remaining_quota", "credits", "quota"} {
		switch v := data[k].(type) {
		case string:
			if v != "" {
				return v
			}
		case float64:
			return fmt.Sprintf("%g", v)
		}
	}
	return ""
}
