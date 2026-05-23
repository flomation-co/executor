package facebook_get_pages

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	fb "flomation.app/automate/executor/actions/social/facebook"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Facebook Get Pages"
	Description  = "List Facebook Pages managed by the authenticated user with their access tokens"
	Website      = "https://www.flomation.co"
	Icon         = "facebook"
	Date         = "21/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "Facebook User Token", Placeholder: "${credentials.facebook}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeString, Label: "App Secret", Placeholder: "${secrets.facebook_app_secret}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
	{Name: "pages", Type: core.ConnectionTypeString, Label: "Pages (JSON array)"},
	{Name: "page_count", Type: core.ConnectionTypeInteger, Label: "Page Count"},
	{Name: "first_page_id", Type: core.ConnectionTypeString, Label: "First Page ID"},
	{Name: "first_page_name", Type: core.ConnectionTypeString, Label: "First Page Name"},
	{Name: "first_page_token", Type: core.ConnectionTypeString, Label: "First Page Access Token"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := fb.GetAccessToken(inputs)
	if err != nil {
		return fb.ErrorResult(err.Error()), nil
	}

	appSecret := fb.GetAppSecret(inputs)
	resp, err := fb.ExecuteAPI(token, appSecret, "GET", fb.GraphAPIBase+"/me/accounts", nil)
	if err != nil {
		return fb.ErrorResult(fmt.Sprintf("failed to get pages: %v", err)), nil
	}

	if err := fb.CheckResponse(resp); err != nil {
		return fb.ErrorResult(err.Error()), nil
	}

	var result struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			AccessToken string `json:"access_token"`
			Category    string `json:"category"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return fb.ErrorResult(fmt.Sprintf("failed to parse pages: %v", err)), nil
	}

	pagesJSON, _ := json.Marshal(result.Data)

	outputs := map[string]interface{}{
		"pages":      string(pagesJSON),
		"page_count": int64(len(result.Data)),
	}

	if len(result.Data) > 0 {
		outputs["first_page_id"] = result.Data[0].ID
		outputs["first_page_name"] = result.Data[0].Name
		outputs["first_page_token"] = result.Data[0].AccessToken
	}

	summary := fmt.Sprintf("Found %d pages", len(result.Data))
	if len(result.Data) > 0 {
		summary += fmt.Sprintf(": %s", result.Data[0].Name)
	}

	return fb.SuccessResult(summary, outputs), nil
}
