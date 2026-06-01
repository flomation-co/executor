// Package insert_image inserts an image into a Google Docs document.
package insert_image

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Insert Image in Document"
	Description  = "Insert an image into a Google Docs document"
	Website      = "https://www.flomation.co"
	Icon         = "file-lines"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	docsAPI = "https://docs.googleapis.com/v1/documents"
)

var Inputs = [...]core.Connection{
	{Name: "document_id", Type: core.ConnectionTypeString, Label: "Document ID", Required: true},
	{Name: "image_url", Type: core.ConnectionTypeString, Label: "Image URL", Required: true},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (points)", Placeholder: "400"},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Height (points)", Placeholder: "300"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	docID := google.OptStr("document_id", inputs)
	if docID == "" {
		return google.ErrorResult("document_id is required")
	}
	imageURL := google.OptStr("image_url", inputs)
	if imageURL == "" {
		return google.ErrorResult("image_url is required")
	}

	width := google.OptInt("width", inputs, 400)
	height := google.OptInt("height", inputs, 300)
	credential := google.OptStr("credential", inputs)
	account := google.OptStr("account", inputs)

	tokens, err := google.FetchTokens(flow, credential)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	active := google.FilterTokens(tokens, account)
	if len(active) == 0 {
		return google.ErrorResult("no active Google account available")
	}
	token := active[0]

	// Points to EMU: 1 point = 12700 EMU
	widthEMU := int64(width) * 12700
	heightEMU := int64(height) * 12700

	payload, _ := json.Marshal(map[string]interface{}{
		"requests": []map[string]interface{}{
			{
				"insertInlineImage": map[string]interface{}{
					"uri": imageURL,
					"objectSize": map[string]interface{}{
						"width":  map[string]interface{}{"magnitude": widthEMU, "unit": "EMU"},
						"height": map[string]interface{}{"magnitude": heightEMU, "unit": "EMU"},
					},
					"endOfSegmentLocation": map[string]interface{}{},
				},
			},
		},
	})

	endpoint := fmt.Sprintf("%s/%s:batchUpdate", docsAPI, docID)

	status, body, err := google.DoRequest(flow, "POST", endpoint, token.AccessToken, payload)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			google.HandleAuthError(flow, token.Email, status)
		}
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(body)))
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Inserted image (%dx%d) into document %s", width, height, docID),
		"success":     true,
		"error":       "",
	}, nil
}
