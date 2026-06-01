// Package add_image adds an image to a Google Slides slide.
package add_image

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
	"github.com/google/uuid"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Add Image to Slide"
	Description  = "Add an image to a Google Slides slide"
	Website      = "https://www.flomation.co"
	Icon         = "display"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	slidesAPI = "https://slides.googleapis.com/v1/presentations"
)

var Inputs = [...]core.Connection{
	{Name: "presentation_id", Type: core.ConnectionTypeString, Label: "Presentation ID", Required: true},
	{Name: "slide_id", Type: core.ConnectionTypeString, Label: "Slide ID", Required: true},
	{Name: "image_url", Type: core.ConnectionTypeString, Label: "Image URL", Required: true},
	{Name: "x", Type: core.ConnectionTypeInteger, Label: "X Position (points)", Placeholder: "100"},
	{Name: "y", Type: core.ConnectionTypeInteger, Label: "Y Position (points)", Placeholder: "100"},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Width (points)", Placeholder: "400"},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Height (points)", Placeholder: "300"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeString, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "image_id", Type: core.ConnectionTypeString, Label: "Image Object ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	presID := google.OptStr("presentation_id", inputs)
	if presID == "" {
		return google.ErrorResult("presentation_id is required")
	}
	slideID := google.OptStr("slide_id", inputs)
	if slideID == "" {
		return google.ErrorResult("slide_id is required")
	}
	imageURL := google.OptStr("image_url", inputs)
	if imageURL == "" {
		return google.ErrorResult("image_url is required")
	}

	x := google.OptInt("x", inputs, 100)
	y := google.OptInt("y", inputs, 100)
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

	imageID := uuid.New().String()

	// Points to EMU: 1 point = 12700 EMU
	payload, _ := json.Marshal(map[string]interface{}{
		"requests": []map[string]interface{}{
			{
				"createImage": map[string]interface{}{
					"objectId": imageID,
					"url":      imageURL,
					"elementProperties": map[string]interface{}{
						"pageObjectId": slideID,
						"size": map[string]interface{}{
							"width":  map[string]interface{}{"magnitude": int64(width) * 12700, "unit": "EMU"},
							"height": map[string]interface{}{"magnitude": int64(height) * 12700, "unit": "EMU"},
						},
						"transform": map[string]interface{}{
							"scaleX":     1,
							"scaleY":     1,
							"translateX": int64(x) * 12700,
							"translateY": int64(y) * 12700,
							"unit":       "EMU",
						},
					},
				},
			},
		},
	})

	endpoint := fmt.Sprintf("%s/%s:batchUpdate", slidesAPI, presID)

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
		"tool_result": fmt.Sprintf("Added image to slide %s {id:%s}", slideID, imageID),
		"image_id":    imageID,
		"success":     true,
		"error":       "",
	}, nil
}
