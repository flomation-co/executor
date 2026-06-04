// Package export_slide exports a thumbnail image of a PowerPoint presentation from OneDrive.
package export_slide

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"

	core "flomation.app/automate/executor"
	microsoft "flomation.app/automate/executor/actions/microsoft"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Export Slide Thumbnail"
	Description  = "Export a presentation thumbnail as a PNG image from OneDrive"
	Website      = "https://www.flomation.co"
	Icon         = "display+image"
	Date         = "04/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Presentation Item ID", Required: true},
	{Name: "size", Type: core.ConnectionTypeString, Label: "Thumbnail Size", Placeholder: "large", Options: []core.ConnectionOption{
		{Name: "Small", Value: "small"},
		{Name: "Medium", Value: "medium"},
		{Name: "Large", Value: "large"},
	}},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Microsoft Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Microsoft OAuth Credential", Placeholder: "${credentials.MICROSOFT_ONEDRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "image", Type: core.ConnectionTypeString, Label: "Image (Base64)"},
	{Name: "content_type", Type: core.ConnectionTypeString, Label: "Content Type"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	itemID := microsoft.OptStr("item_id", inputs)
	if itemID == "" {
		return microsoft.ErrorResult("item_id is required")
	}

	size := microsoft.OptStr("size", inputs)
	if size == "" {
		size = "large"
	}

	credential := microsoft.OptStr("credential", inputs)
	account := microsoft.OptStr("account", inputs)

	tokens, err := microsoft.FetchTokens(flow, credential, "onedrive")
	if err != nil {
		return microsoft.ErrorResult(err.Error())
	}
	active := microsoft.FilterTokens(tokens, account)
	if len(active) == 0 {
		return microsoft.ErrorResult("no active Microsoft account available")
	}
	token := active[0]

	// Fetch the thumbnail image via the thumbnails endpoint.
	endpoint := fmt.Sprintf("%s/me/drive/items/%s/thumbnails/0/%s/content",
		microsoft.GraphAPI, itemID, size)

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, endpoint, nil)
	if err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("failed to create request: %v", err))
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return microsoft.ErrorResult(fmt.Sprintf("thumbnail request failed: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20))

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			microsoft.HandleAuthError(flow, token.Email, resp.StatusCode)
		}
		return microsoft.ErrorResult(fmt.Sprintf("thumbnail returned %d: %s", resp.StatusCode, microsoft.TruncateBody(body)))
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}

	encoded := base64.StdEncoding.EncodeToString(body)

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Exported %s thumbnail (%d bytes)", size, len(body)),
		"image":        encoded,
		"content_type": contentType,
		"success":      true,
		"error":        "",
	}, nil
}
