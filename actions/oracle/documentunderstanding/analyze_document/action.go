// Package oracle_documentunderstanding_analyze_document runs a synchronous Document AI analysis on
// an inline (base64) document — extracting text, tables, form key-values, and/or classifying the
// document type — and returns the raw analysis result.
package oracle_documentunderstanding_analyze_document

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	du "flomation.app/automate/executor/actions/oracle/documentunderstanding"

	"github.com/oracle/oci-go-sdk/v65/aidocument"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Document Understanding: Analyze Document"
	Description  = "Synchronously analyze an inline (base64) document — extract text, tables and form key-values, and/or classify the document type — returning the raw analysis result."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+image"
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
	{Name: "document_base64", Type: core.ConnectionTypeText, Label: "Document (Base64)", Placeholder: "The document bytes, Base64-encoded (PDF, TIFF, PNG or JPEG). A data: URI prefix is accepted.", Required: true},
	{Name: "feature_types", Type: core.ConnectionTypeString, Label: "Feature Types", Placeholder: "Comma-separated: TEXT_EXTRACTION, TABLE_EXTRACTION, KEY_VALUE_EXTRACTION, DOCUMENT_CLASSIFICATION", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Analysis result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// buildFeature maps a normalised feature-type token to its concrete polymorphic feature.
func buildFeature(token string) (aidocument.DocumentFeature, bool) {
	switch token {
	case "TEXT_EXTRACTION":
		return aidocument.DocumentTextExtractionFeature{}, true
	case "TABLE_EXTRACTION":
		return aidocument.DocumentTableExtractionFeature{}, true
	case "KEY_VALUE_EXTRACTION":
		return aidocument.DocumentKeyValueExtractionFeature{}, true
	case "DOCUMENT_CLASSIFICATION":
		return aidocument.DocumentClassificationFeature{}, true
	default:
		return nil, false
	}
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := du.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}

	// Document: strip an optional data: URI prefix, then Base64-decode into raw bytes so the SDK
	// re-encodes them exactly once on the wire.
	docB64, err := du.RequiredString("document_base64", inputs)
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}
	if i := strings.Index(docB64, "base64,"); i >= 0 {
		docB64 = docB64[i+len("base64,"):]
	}
	docB64 = strings.Join(strings.Fields(docB64), "")
	data, err := base64.StdEncoding.DecodeString(docB64)
	if err != nil {
		return du.ErrorResult("document_base64 is not valid Base64: " + err.Error()), nil
	}
	if len(data) == 0 {
		return du.ErrorResult("document_base64 decoded to zero bytes"), nil
	}

	// Features: at least one required.
	rawFeatures, err := du.RequiredString("feature_types", inputs)
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}
	var features []aidocument.DocumentFeature
	for _, part := range strings.Split(rawFeatures, ",") {
		token := strings.ToUpper(strings.TrimSpace(part))
		if token == "" {
			continue
		}
		feat, ok := buildFeature(token)
		if !ok {
			return du.ErrorResult(fmt.Sprintf("unsupported feature type %q — expected one or more of TEXT_EXTRACTION, TABLE_EXTRACTION, KEY_VALUE_EXTRACTION, DOCUMENT_CLASSIFICATION", token)), nil
		}
		features = append(features, feat)
	}
	if len(features) == 0 {
		return du.ErrorResult("at least one feature type is required"), nil
	}

	req := aidocument.AnalyzeDocumentRequest{
		AnalyzeDocumentDetails: aidocument.AnalyzeDocumentDetails{
			CompartmentId: &compartment,
			Features:      features,
			Document:      aidocument.InlineDocumentDetails{Data: data},
		},
	}

	resp, err := client.AnalyzeDocument(du.Context(), req)
	if err != nil {
		return du.ErrorResult(auth.OCIError(err)), nil
	}

	// No summariser fits the free-form analysis payload — surface the raw result as a generic object.
	rawResult, mErr := json.Marshal(resp.AnalyzeDocumentResult)
	if mErr != nil {
		return du.ErrorResult("document analyzed but its result could not be serialised: " + mErr.Error()), nil
	}
	var result map[string]interface{}
	if uErr := json.Unmarshal(rawResult, &result); uErr != nil {
		return du.ErrorResult("document analyzed but its result could not be parsed: " + uErr.Error()), nil
	}

	msg := fmt.Sprintf("Analyzed document: %d page(s), %d feature(s)", len(resp.Pages), len(features))
	return du.Result(msg, map[string]interface{}{"result": result}), nil
}
