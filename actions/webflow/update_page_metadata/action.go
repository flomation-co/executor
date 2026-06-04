package webflow_update_page_metadata

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	webflow "flomation.app/automate/executor/actions/webflow"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Update Page Metadata"
	Description  = "Update the title, slug, SEO, and Open Graph metadata of a Webflow page"
	Website      = "https://www.flomation.co"
	Icon         = "webflow+pencil"
	Date         = "29/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "api_token",
		Type:        core.ConnectionTypeString,
		Label:       "Webflow API Token",
		Placeholder: "wfl_...",
		Required:    true,
	},
	{
		Name:        "page_id",
		Type:        core.ConnectionTypeString,
		Label:       "Page ID",
		Placeholder: "The page ID",
		Required:    true,
	},
	{
		Name:        "title",
		Type:        core.ConnectionTypeString,
		Label:       "Title",
		Placeholder: "Page title",
	},
	{
		Name:        "slug",
		Type:        core.ConnectionTypeString,
		Label:       "Slug",
		Placeholder: "Page URL slug",
	},
	{
		Name:        "seo_title",
		Type:        core.ConnectionTypeString,
		Label:       "SEO Title",
		Placeholder: "SEO title tag",
	},
	{
		Name:        "seo_description",
		Type:        core.ConnectionTypeString,
		Label:       "SEO Description",
		Placeholder: "SEO meta description",
	},
	{
		Name:        "og_title",
		Type:        core.ConnectionTypeString,
		Label:       "Open Graph Title",
		Placeholder: "og:title value",
	},
	{
		Name:        "og_description",
		Type:        core.ConnectionTypeString,
		Label:       "Open Graph Description",
		Placeholder: "og:description value",
	},
	{
		Name:        "og_image",
		Type:        core.ConnectionTypeString,
		Label:       "Open Graph Image",
		Placeholder: "og:image URL",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "page", Type: core.ConnectionTypeObject, Label: "Page"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := webflow.GetAPIToken(inputs)
	if err != nil {
		return nil, err
	}

	pageID, err := webflow.RequiredString("page_id", inputs)
	if err != nil {
		return nil, err
	}

	reqBody := make(map[string]interface{})

	if v := webflow.OptionalString("title", inputs); v != "" {
		reqBody["title"] = v
	}
	if v := webflow.OptionalString("slug", inputs); v != "" {
		reqBody["slug"] = v
	}

	// Build SEO object
	seo := make(map[string]interface{})
	if v := webflow.OptionalString("seo_title", inputs); v != "" {
		seo["title"] = v
	}
	if v := webflow.OptionalString("seo_description", inputs); v != "" {
		seo["description"] = v
	}
	if len(seo) > 0 {
		reqBody["seo"] = seo
	}

	// Build Open Graph object
	og := make(map[string]interface{})
	if v := webflow.OptionalString("og_title", inputs); v != "" {
		og["title"] = v
		og["titleCopied"] = false
	}
	if v := webflow.OptionalString("og_description", inputs); v != "" {
		og["description"] = v
		og["descriptionCopied"] = false
	}
	if v := webflow.OptionalString("og_image", inputs); v != "" {
		og["image"] = v
	}
	if len(og) > 0 {
		reqBody["openGraph"] = og
	}

	if len(reqBody) == 0 {
		return webflow.ErrorResult("No metadata fields provided to update")
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to build request body: %s", err))
	}

	status, body, err := webflow.ExecuteRequest(token, "PATCH", "/pages/"+pageID, bodyBytes)
	if err != nil {
		return webflow.ErrorResult(fmt.Sprintf("Failed to update page metadata: %s", err))
	}
	if status < 200 || status >= 300 {
		return webflow.ErrorResult(fmt.Sprintf("Webflow API error (%d): %s", status, string(body)))
	}

	var parsed interface{}
	_ = json.Unmarshal(body, &parsed)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Updated metadata for page %s", pageID),
		"page":        parsed,
		"success":     true,
		"error":       "",
	}, nil
}
