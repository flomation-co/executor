// Package share manages sharing permissions for a Google Drive file.
package share

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Share Drive File"
	Description  = "Share a Google Drive file with a user or make it public"
	Website      = "https://www.flomation.co"
	Icon         = "folder"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	driveAPI = "https://www.googleapis.com/drive/v3"
)

var Inputs = [...]core.Connection{
	{Name: "file_id", Type: core.ConnectionTypeString, Label: "File ID", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email Address"},
	{
		Name:  "role",
		Type:  core.ConnectionTypeString,
		Label: "Role",
		Options: []core.ConnectionOption{
			{Name: "Viewer", Value: "reader"},
			{Name: "Commenter", Value: "commenter"},
			{Name: "Editor", Value: "writer"},
			{Name: "Owner", Value: "owner"},
		},
	},
	{
		Name:  "type",
		Type:  core.ConnectionTypeString,
		Label: "Permission Type",
		Options: []core.ConnectionOption{
			{Name: "User", Value: "user"},
			{Name: "Group", Value: "group"},
			{Name: "Domain", Value: "domain"},
			{Name: "Anyone (public)", Value: "anyone"},
		},
	},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "permission", Type: core.ConnectionTypeString, Label: "Permission (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	fileID := google.OptStr("file_id", inputs)
	if fileID == "" {
		return google.ErrorResult("file_id is required")
	}

	email := google.OptStr("email", inputs)
	role := google.OptStr("role", inputs)
	permType := google.OptStr("type", inputs)
	credential := google.OptStr("credential", inputs)
	account := google.OptStr("account", inputs)

	if role == "" {
		role = "reader"
	}
	if permType == "" {
		permType = "user"
	}
	if permType != "anyone" && email == "" {
		return google.ErrorResult("email is required for user/group/domain permissions")
	}

	tokens, err := google.FetchTokens(flow, credential)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	active := google.FilterTokens(tokens, account)
	if len(active) == 0 {
		return google.ErrorResult("no active Google account available")
	}
	token := active[0]

	perm := map[string]interface{}{
		"role": role,
		"type": permType,
	}
	if permType == "user" || permType == "group" {
		perm["emailAddress"] = email
	} else if permType == "domain" {
		perm["domain"] = email
	}

	body, _ := json.Marshal(perm)
	endpoint := fmt.Sprintf("%s/files/%s/permissions?sendNotificationEmail=true", driveAPI, fileID)

	status, respBody, err := google.DoRequest(flow, "POST", endpoint, token.AccessToken, body)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			google.HandleAuthError(flow, token.Email, status)
		}
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(respBody)))
	}

	target := email
	if permType == "anyone" {
		target = "anyone"
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Shared file %s with %s as %s", fileID, target, role),
		"permission":  string(respBody),
		"success":     true,
		"error":       "",
	}, nil
}
