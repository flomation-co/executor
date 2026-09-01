// Package image_upload uploads an image to a Meta ad account's image library
// and returns the hash a creative references.
package image_upload

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"

	core "flomation.app/automate/executor"
	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Media: Upload Image"
	Description  = "Upload an image to a Meta ad account and return the image hash for use in a creative."
	Website      = "https://www.flomation.co"
	Icon         = "facebook+cloud-arrow-up"
	Date         = "17/08/2026"
	Type         = core.ActionTypeAction
)

// maxImageBytes guards the request before it is built. Meta's own limit is
// around 8 MB, and base64 inflates the payload by a third, so an oversized file
// would be rejected after the cost of encoding and uploading it. Failing early
// with the actual size is more useful than Graph's generic complaint.
const maxImageBytes = 8 << 20

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Meta Access Token", Placeholder: "${secrets.MetaAdsToken}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret (recommended)", Placeholder: "${secrets.MetaAppSecret}"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Ad Account ID", Placeholder: "act_1234567890", Required: true},
	// Takes any of the executor's media representations, so it can be wired
	// straight from an image action, a file download or an uploaded asset
	// without the author converting anything by hand.
	// The Label is what an AI agent sees as this parameter's description
	// (the Placeholder is editor-only), so it has to say what a valid value
	// looks like. Without that, an agent holding an image URL reasonably
	// assumes the field takes one — Meta's /adimages does not accept a URL,
	// and fetching it here would duplicate Download File's SSRF guard.
	{Name: "image", Type: core.ConnectionTypeFile, Required: true,
		Label:       "Image file reference (flo:blob:… or flo:file:…) from an upstream image or Download File action — NOT a URL",
		Placeholder: "Wire from an image node, or a flo:blob:/flo:file: reference"},
	// Meta keys the upload response by filename and shows it in the ad image
	// library, so it is worth being able to set. Left blank it comes from the
	// image itself, or is generated with the right extension.
	{Name: "filename", Type: core.ConnectionTypeString,
		Label:       "Filename to store the image under (optional — defaults to the image's own name)",
		Placeholder: "advert-summer.jpg"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "image_hash", Type: core.ConnectionTypeString, Label: "Image Hash"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "Image URL"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, secret, err := meta.GetAuth(inputs)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}
	account, err := meta.RequiredString("account_id", inputs)
	if err != nil {
		return meta.ErrorResult("an ad account ID is required"), nil
	}
	imageRef, err := meta.RequiredString("image", inputs)
	if err != nil {
		return meta.ErrorResult("an image is required"), nil
	}
	if flow == nil {
		return meta.ErrorResult("an execution context is required to read the image"), nil
	}

	path, mimeType, err := flow.ResolveToLocalFile(imageRef)
	if err != nil {
		return meta.ErrorResult("could not read the image: " + err.Error()), nil
	}

	// Meta rejects an upload whose filename has no image extension, and keys
	// its response by that name, so it must always be a real one.
	filename := core.UploadFilename(
		meta.OptionalString("filename", inputs), imageRef, mimeType, "image")

	info, err := os.Stat(path)
	if err != nil {
		return meta.ErrorResult("could not read the image: " + err.Error()), nil
	}
	if info.Size() > maxImageBytes {
		return meta.ErrorResult(fmt.Sprintf(
			"the image is %.1f MB, above Meta's ~8 MB limit for ad images — resize it before uploading",
			float64(info.Size())/(1<<20))), nil
	}

	raw, err := os.ReadFile(path) //nolint:gosec // path is confined to the execution workspace by ResolveToLocalFile
	if err != nil {
		return meta.ErrorResult("could not read the image: " + err.Error()), nil
	}

	p := url.Values{
		"bytes":    {base64.StdEncoding.EncodeToString(raw)},
		"filename": {filename},
	}

	resp, err := meta.NewClient(token, secret).Post(flow, meta.AccountPath(account)+"/adimages", p)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	// Graph keys the result by FILENAME rather than returning a flat object, so
	// the hash has to be dug out of a map whose key is not known in advance.
	hash, imageURL := "", ""
	if images, ok := resp["images"].(map[string]interface{}); ok {
		for _, v := range images {
			entry, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			hash, _ = entry["hash"].(string)
			imageURL, _ = entry["url"].(string)
			break
		}
	}
	if hash == "" {
		return meta.ErrorResult("the upload succeeded but Meta returned no image hash, so it cannot be used in a creative"), nil
	}

	return meta.OkResult(
		fmt.Sprintf("Uploaded %s (%.0f KB). Image hash: %s — pass this to Creatives: Create.",
			filename, float64(info.Size())/1024, hash),
		map[string]interface{}{"image_hash": hash, "url": imageURL},
	), nil
}
