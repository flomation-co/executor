// Package oracle_vision_analyze_image runs OCI Vision's synchronous image analysis over a single
// inline (base64) image, returning any combination of classification labels, detected objects and
// OCR text depending on the requested feature types.
package oracle_vision_analyze_image

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	vis "flomation.app/automate/executor/actions/oracle/vision"

	"github.com/oracle/oci-go-sdk/v65/aivision"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vision: Analyze Image"
	Description  = "Analyze a base64-encoded image with OCI Vision — request any mix of classification labels, object detection and text (OCR) detection, and get the labels, objects and recognised text back."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+eye"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "image_base64", Type: core.ConnectionTypeText, Label: "Image (base64)", Placeholder: "Base64-encoded image bytes (a data: URI prefix is accepted and stripped)", Required: true},
	{Name: "feature_types", Type: core.ConnectionTypeString, Label: "Feature Types", Placeholder: "Comma-separated: CLASSIFY, DETECT_OBJECTS, DETECT_TEXT", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Analysis result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := vis.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return vis.ErrorResult(err.Error()), nil
	}

	imgB64, err := vis.RequiredString("image_base64", inputs)
	if err != nil {
		return vis.ErrorResult(err.Error()), nil
	}
	// Strip an optional data: URI prefix (e.g. "data:image/png;base64,....").
	if strings.HasPrefix(imgB64, "data:") {
		if i := strings.IndexByte(imgB64, ','); i >= 0 {
			imgB64 = imgB64[i+1:]
		}
	}
	imgB64 = strings.TrimSpace(imgB64)
	data, err := base64.StdEncoding.DecodeString(imgB64)
	if err != nil {
		return vis.ErrorResult("image_base64 is not valid base64 — supply the base64-encoded image bytes"), nil
	}
	if len(data) == 0 {
		return vis.ErrorResult("image_base64 decoded to no bytes"), nil
	}

	featRaw, err := vis.RequiredString("feature_types", inputs)
	if err != nil {
		return vis.ErrorResult(err.Error()), nil
	}
	var features []aivision.ImageFeature
	for _, tok := range strings.Split(featRaw, ",") {
		t := strings.ToUpper(strings.TrimSpace(tok))
		if t == "" {
			continue
		}
		switch t {
		case "CLASSIFY", "IMAGE_CLASSIFICATION", "CLASSIFICATION":
			features = append(features, aivision.ImageClassificationFeature{})
		case "DETECT_OBJECTS", "OBJECT_DETECTION", "OBJECTS":
			features = append(features, aivision.ImageObjectDetectionFeature{})
		case "DETECT_TEXT", "TEXT_DETECTION", "TEXT", "OCR":
			features = append(features, aivision.ImageTextDetectionFeature{})
		default:
			return vis.ErrorResult(fmt.Sprintf("unknown feature type %q — use CLASSIFY, DETECT_OBJECTS or DETECT_TEXT", tok)), nil
		}
	}
	if len(features) == 0 {
		return vis.ErrorResult("feature_types is required — a comma-separated list of CLASSIFY, DETECT_OBJECTS, DETECT_TEXT"), nil
	}

	resp, err := client.AnalyzeImage(vis.Context(), aivision.AnalyzeImageRequest{
		AnalyzeImageDetails: aivision.AnalyzeImageDetails{
			CompartmentId: &compartment,
			Features:      features,
			Image:         aivision.InlineImageDetails{Data: data},
		},
	})
	if err != nil {
		return vis.ErrorResult(auth.OCIError(err)), nil
	}

	// Flatten the full result to a generic map for the Object output, and add friendly counts.
	rawJSON, err := json.Marshal(resp.AnalyzeImageResult)
	if err != nil {
		return vis.ErrorResult(fmt.Sprintf("image analyzed but its result could not be serialised: %s", err.Error())), nil
	}
	var rawMap map[string]interface{}
	if err := json.Unmarshal(rawJSON, &rawMap); err != nil {
		return vis.ErrorResult(fmt.Sprintf("image analyzed but its result could not be parsed: %s", err.Error())), nil
	}

	wordCount, lineCount := 0, 0
	if resp.ImageText != nil {
		wordCount = len(resp.ImageText.Words)
		lineCount = len(resp.ImageText.Lines)
	}
	result := map[string]interface{}{
		"label_count":  len(resp.Labels),
		"object_count": len(resp.ImageObjects),
		"word_count":   wordCount,
		"line_count":   lineCount,
		"raw":          rawMap,
	}

	return vis.Result(
		fmt.Sprintf("Analyzed image — %d label(s), %d object(s), %d text line(s)", len(resp.Labels), len(resp.ImageObjects), lineCount),
		map[string]interface{}{"result": result},
	), nil
}
