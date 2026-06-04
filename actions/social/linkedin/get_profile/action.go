package linkedin_get_profile

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	linkedin "flomation.app/automate/executor/actions/social/linkedin"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "LinkedIn Get Profile"
	Description  = "Get the authenticated user's LinkedIn profile and member URN"
	Website      = "https://www.flomation.co"
	Icon         = "linkedin+user"
	Date         = "21/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "LinkedIn Access Token", Placeholder: "${credentials.linkedin}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
	{Name: "member_urn", Type: core.ConnectionTypeString, Label: "Member URN"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Full Name"},
	{Name: "given_name", Type: core.ConnectionTypeString, Label: "First Name"},
	{Name: "family_name", Type: core.ConnectionTypeString, Label: "Last Name"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email"},
	{Name: "picture_url", Type: core.ConnectionTypeString, Label: "Profile Picture URL"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := linkedin.GetAccessToken(inputs)
	if err != nil {
		return linkedin.ErrorResult(err.Error()), nil
	}

	// Get profile using OpenID userinfo endpoint
	resp, err := linkedin.ExecuteAPI(token, "GET", "https://api.linkedin.com/v2/userinfo", nil)
	if err != nil {
		return linkedin.ErrorResult(fmt.Sprintf("failed to get profile: %v", err)), nil
	}

	if err := linkedin.CheckResponse(resp); err != nil {
		return linkedin.ErrorResult(err.Error()), nil
	}

	var profile struct {
		Sub        string `json:"sub"`
		Name       string `json:"name"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
		Email      string `json:"email"`
		Picture    string `json:"picture"`
	}
	if err := json.Unmarshal(resp.Body, &profile); err != nil {
		return linkedin.ErrorResult(fmt.Sprintf("failed to parse profile: %v", err)), nil
	}

	memberURN := "urn:li:person:" + profile.Sub

	return linkedin.SuccessResult(
		fmt.Sprintf("Profile: %s (%s)", profile.Name, profile.Email),
		map[string]interface{}{
			"member_urn":  memberURN,
			"name":        profile.Name,
			"given_name":  profile.GivenName,
			"family_name": profile.FamilyName,
			"email":       profile.Email,
			"picture_url": profile.Picture,
		},
	), nil
}
