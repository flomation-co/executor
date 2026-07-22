// Package oracle_vision_analyze_document runs OCI Vision Document AI over an inline document
// (a base64-encoded image or PDF), returning extracted text, tables, key-value pairs and/or a
// document classification depending on the requested feature types.
package oracle_vision_analyze_document

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
	Name         = "OCI Vision: Analyze Document"
	Description  = "Run OCI Vision Document AI over a base64 document (image or PDF) to extract text, tables and key-value pairs, or classify the document type."
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
	{Name: "document_base64", Type: core.ConnectionTypeText, Label: "Document (base64)", Placeholder: "Base64-encoded document bytes (image or PDF)", Required: true},
	{Name: "feature_types", Type: core.ConnectionTypeText, Label: "Feature Types (CSV)", Placeholder: "Comma-separated: TEXT_DETECTION, TABLE_DETECTION, KEY_VALUE_DETECTION, DOCUMENT_CLASSIFICATION", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Analysis Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// buildFeature constructs the concrete polymorphic DocumentFeature for one SDK feature-type value.
func buildFeature(featureType aivision.DocumentFeatureFeatureTypeEnum) aivision.DocumentFeature {
	switch featureType {
	case aivision.DocumentFeatureFeatureTypeTextDetection:
		return aivision.DocumentTextDetectionFeature{}
	case aivision.DocumentFeatureFeatureTypeTableDetection:
		return aivision.DocumentTableDetectionFeature{}
	case aivision.DocumentFeatureFeatureTypeKeyValueDetection:
		return aivision.DocumentKeyValueDetectionFeature{}
	case aivision.DocumentFeatureFeatureTypeDocumentClassification:
		return aivision.DocumentClassificationFeature{}
	case aivision.DocumentFeatureFeatureTypeLanguageClassification:
		return aivision.DocumentLanguageClassificationFeature{}
	default:
		return nil
	}
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
	docB64, err := vis.RequiredString("document_base64", inputs)
	if err != nil {
		return vis.ErrorResult(err.Error()), nil
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(docB64))
	if err != nil {
		return vis.ErrorResult(fmt.Sprintf("document_base64 must be valid base64: %s", err.Error())), nil
	}
	if len(data) == 0 {
		return vis.ErrorResult("document_base64 decodes to an empty document"), nil
	}

	rawFeatures, err := vis.RequiredString("feature_types", inputs)
	if err != nil {
		return vis.ErrorResult(err.Error()), nil
	}
	var features []aivision.DocumentFeature
	seen := map[aivision.DocumentFeatureFeatureTypeEnum]bool{}
	for _, part := range strings.Split(rawFeatures, ",") {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		ft, ok := aivision.GetMappingDocumentFeatureFeatureTypeEnum(token)
		if !ok {
			return vis.ErrorResult(fmt.Sprintf("unsupported feature type %q — expected one of: %s", token, strings.Join(aivision.GetDocumentFeatureFeatureTypeEnumStringValues(), ", "))), nil
		}
		if seen[ft] {
			continue
		}
		seen[ft] = true
		features = append(features, buildFeature(ft))
	}
	if len(features) == 0 {
		return vis.ErrorResult("at least one feature type is required"), nil
	}

	details := aivision.AnalyzeDocumentDetails{
		Features:      features,
		Document:      aivision.InlineDocumentDetails{Data: data},
		CompartmentId: &compartment,
	}

	resp, err := client.AnalyzeDocument(vis.Context(), aivision.AnalyzeDocumentRequest{AnalyzeDocumentDetails: details})
	if err != nil {
		return vis.ErrorResult(auth.OCIError(err)), nil
	}

	// Flatten the full result into a generic map so operators get every extracted field.
	result := map[string]interface{}{}
	if b, mErr := json.Marshal(resp.AnalyzeDocumentResult); mErr == nil {
		_ = json.Unmarshal(b, &result)
	}

	pageCount := 0
	mimeType := ""
	if resp.DocumentMetadata != nil {
		if resp.DocumentMetadata.PageCount != nil {
			pageCount = *resp.DocumentMetadata.PageCount
		}
		mimeType = vis.Str(resp.DocumentMetadata.MimeType)
	}

	summary := fmt.Sprintf("Analyzed %s document (%d page(s)) with %d feature(s)", strings.TrimSpace(mimeType), pageCount, len(features))
	if n := len(resp.DetectedDocumentTypes); n > 0 {
		summary += fmt.Sprintf("; %d document type(s) detected", n)
	}

	return vis.Result(summary, map[string]interface{}{
		"result": result,
	}), nil
}
