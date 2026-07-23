// Package oracle_documentunderstanding_processor_job_create starts an asynchronous OCI Document
// Understanding processor job that reads a document from Object Storage, runs the requested analysis
// features (text / table / key-value / classification / language / elements) and writes the results
// back to an Object Storage prefix. The job comes back ACCEPTED — poll Get Processor Job until it is
// SUCCEEDED before reading the output.
package oracle_documentunderstanding_processor_job_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	du "flomation.app/automate/executor/actions/oracle/documentunderstanding"

	"github.com/oracle/oci-go-sdk/v65/aidocument"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Document Understanding: Create Processor Job"
	Description  = "Start an asynchronous processor job that analyses a document in Object Storage (text, table, key-value, classification, language or elements) and writes results to an output prefix. Returns the job in an ACCEPTED state — poll Get Processor Job until SUCCEEDED."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+image"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "input_namespace", Type: core.ConnectionTypeString, Label: "Input Namespace", Placeholder: "Object Storage namespace of the input document", Required: true},
	{Name: "input_bucket", Type: core.ConnectionTypeString, Label: "Input Bucket", Placeholder: "Bucket holding the input document", Required: true},
	{Name: "input_object", Type: core.ConnectionTypeString, Label: "Input Object", Placeholder: "Object (file) name of the document to analyse", Required: true},
	{Name: "output_namespace", Type: core.ConnectionTypeString, Label: "Output Namespace", Placeholder: "Object Storage namespace for the results", Required: true},
	{Name: "output_bucket", Type: core.ConnectionTypeString, Label: "Output Bucket", Placeholder: "Bucket to write results to", Required: true},
	{Name: "output_prefix", Type: core.ConnectionTypeString, Label: "Output Prefix", Placeholder: "Folder/prefix under which results are written", Required: true},
	{Name: "feature_types", Type: core.ConnectionTypeString, Label: "Feature Types (CSV)", Placeholder: "TEXT_EXTRACTION,TABLE_EXTRACTION,KEY_VALUE_EXTRACTION,DOCUMENT_CLASSIFICATION,LANGUAGE_CLASSIFICATION,DOCUMENT_ELEMENTS_EXTRACTION", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "processor_job", Type: core.ConnectionTypeObject, Label: "Processor Job"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Processor Job OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// buildFeature maps a validated feature-type enum to its concrete DocumentFeature.
func buildFeature(ft aidocument.DocumentFeatureFeatureTypeEnum) aidocument.DocumentFeature {
	switch ft {
	case aidocument.DocumentFeatureFeatureTypeTextExtraction:
		return aidocument.DocumentTextExtractionFeature{}
	case aidocument.DocumentFeatureFeatureTypeTableExtraction:
		return aidocument.DocumentTableExtractionFeature{}
	case aidocument.DocumentFeatureFeatureTypeKeyValueExtraction:
		return aidocument.DocumentKeyValueExtractionFeature{}
	case aidocument.DocumentFeatureFeatureTypeDocumentClassification:
		return aidocument.DocumentClassificationFeature{}
	case aidocument.DocumentFeatureFeatureTypeLanguageClassification:
		return aidocument.DocumentLanguageClassificationFeature{}
	case aidocument.DocumentFeatureFeatureTypeDocumentElementsExtraction:
		return aidocument.DocumentElementsExtractionFeature{}
	default:
		return nil
	}
}

func parseFeatures(raw string) ([]aidocument.DocumentFeature, error) {
	var features []aidocument.DocumentFeature
	seen := map[aidocument.DocumentFeatureFeatureTypeEnum]bool{}
	for _, part := range strings.Split(raw, ",") {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		ft, ok := aidocument.GetMappingDocumentFeatureFeatureTypeEnum(token)
		if !ok {
			return nil, fmt.Errorf("feature type %q is not recognised — expected one of %s", token, strings.Join(aidocument.GetDocumentFeatureFeatureTypeEnumStringValues(), ", "))
		}
		if seen[ft] {
			continue
		}
		seen[ft] = true
		features = append(features, buildFeature(ft))
	}
	if len(features) == 0 {
		return nil, fmt.Errorf("at least one feature type is required")
	}
	return features, nil
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

	inputNamespace, err := du.RequiredString("input_namespace", inputs)
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}
	inputBucket, err := du.RequiredString("input_bucket", inputs)
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}
	inputObject, err := du.RequiredString("input_object", inputs)
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}
	outputNamespace, err := du.RequiredString("output_namespace", inputs)
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}
	outputBucket, err := du.RequiredString("output_bucket", inputs)
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}
	outputPrefix, err := du.RequiredString("output_prefix", inputs)
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}
	rawFeatures, err := du.RequiredString("feature_types", inputs)
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}
	features, err := parseFeatures(rawFeatures)
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}

	details := aidocument.CreateProcessorJobDetails{
		CompartmentId: &compartment,
		InputLocation: aidocument.ObjectStorageLocations{
			ObjectLocations: []aidocument.ObjectLocation{{
				NamespaceName: &inputNamespace,
				BucketName:    &inputBucket,
				ObjectName:    &inputObject,
			}},
		},
		OutputLocation: &aidocument.OutputLocation{
			NamespaceName: &outputNamespace,
			BucketName:    &outputBucket,
			Prefix:        &outputPrefix,
		},
		ProcessorConfig: aidocument.GeneralProcessorConfig{
			Features: features,
		},
	}

	resp, err := client.CreateProcessorJob(du.Context(), aidocument.CreateProcessorJobRequest{CreateProcessorJobDetails: details})
	if err != nil {
		return du.ErrorResult(auth.OCIError(err)), nil
	}
	summary := du.SummariseProcessorJob(&resp.ProcessorJob)
	return du.Result(fmt.Sprintf("Started processor job %s — poll Get Processor Job until SUCCEEDED", du.Str(resp.Id)), map[string]interface{}{
		"processor_job":   summary,
		"id":              du.Str(resp.Id),
		"lifecycle_state": string(resp.LifecycleState),
	}), nil
}
