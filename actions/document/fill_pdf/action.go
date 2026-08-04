// Package fillpdf populates the AcroForm fields of an uploaded fillable PDF with
// values supplied by the flow (e.g. from a Form trigger) and outputs the
// completed PDF as a media reference. It's the "populate an existing template
// and export a PDF" building block.
//
// The template is a `file` input, so the editor renders an upload widget and the
// author drops in a fillable PDF (its bytes become a flo:blob: reference the
// engine resolves via ResolveToLocalFile). Filling is done with pdfcpu (pure Go,
// no external binary): export the form's field structure, set each field's value
// by name, then fill; optionally lock the fields so the result isn't editable.
package fillpdf

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfmodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Fill PDF Form"
	Description  = "Populate an uploaded fillable PDF's form fields and output the completed PDF"
	Website      = "https://www.flomation.co"
	Icon         = "file-export"
	Date         = "04/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "template", Type: core.ConnectionTypeFile, Label: "PDF template", Placeholder: "Upload a fillable PDF", Required: true},
	{Name: "fields", Type: core.ConnectionTypeKeyValueArray, Label: "Field values", Placeholder: "PDF field name = value"},
	{Name: "flatten", Type: core.ConnectionTypeBoolean, Label: "Lock fields (make the result non-editable)"},
	{Name: "filename", Type: core.ConnectionTypeString, Label: "Output filename", Placeholder: "completed.pdf"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "pdf", Type: core.ConnectionTypeString, Label: "Completed PDF (media reference)"},
	{Name: "filename", Type: core.ConnectionTypeString, Label: "Filename"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": "Error: " + msg,
		"success":     false,
		"error":       msg,
	}, nil
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	tc := core.FindConnection("template", inputs)
	if tc == nil || tc.String() == nil || strings.TrimSpace(*tc.String()) == "" {
		return errResult("no PDF template uploaded")
	}
	templateRef := strings.TrimSpace(*tc.String())

	// Field name -> value map from the key-value input (values may be ${...}).
	values := map[string]string{}
	if fc := core.FindConnection("fields", inputs); fc != nil {
		for _, kv := range fc.KeyValuePairs() {
			if k := strings.TrimSpace(kv.Key); k != "" {
				values[k] = kv.Value
			}
		}
	}

	flatten := false
	if bc := core.FindConnection("flatten", inputs); bc != nil && bc.Boolean() != nil {
		flatten = *bc.Boolean()
	}

	filename := "completed.pdf"
	if nc := core.FindConnection("filename", inputs); nc != nil && nc.String() != nil && strings.TrimSpace(*nc.String()) != "" {
		filename = strings.TrimSpace(*nc.String())
	}

	// 1. Resolve the uploaded template to a local workspace file.
	templatePath, _, err := flow.ResolveToLocalFile(templateRef)
	if err != nil {
		return errResult("could not read the PDF template: " + err.Error())
	}

	// 2. Export the form's field structure to JSON, then merge the values in.
	jsonPath, err := flow.MediaScratchFile(".json")
	if err != nil {
		return errResult("scratch: " + err.Error())
	}
	if err := pdfapi.ExportFormFile(templatePath, jsonPath, pdfmodel.NewDefaultConfiguration()); err != nil {
		return errResult("the uploaded PDF has no fillable form fields (" + err.Error() + ")")
	}
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		return errResult("read form data: " + err.Error())
	}
	merged, filledCount, err := mergeFormValues(raw, values)
	if err != nil {
		return errResult(err.Error())
	}
	if err := os.WriteFile(jsonPath, merged, 0o600); err != nil {
		return errResult("write form data: " + err.Error())
	}

	// 3. Fill the form from the merged JSON.
	outPath, err := flow.MediaScratchFile(".pdf")
	if err != nil {
		return errResult("scratch: " + err.Error())
	}
	if err := pdfapi.FillFormFile(templatePath, jsonPath, outPath, pdfmodel.NewDefaultConfiguration()); err != nil {
		return errResult("fill PDF: " + err.Error())
	}

	// 4. Optionally lock every field so the completed PDF isn't editable.
	if flatten {
		lockedPath, err := flow.MediaScratchFile(".pdf")
		if err != nil {
			return errResult("scratch: " + err.Error())
		}
		if err := pdfapi.LockFormFieldsFile(outPath, lockedPath, nil, pdfmodel.NewDefaultConfiguration()); err != nil {
			return errResult("lock fields: " + err.Error())
		}
		outPath = lockedPath
	}

	// 5. Emit as a media reference (a previewable blob when small enough).
	ref, err := flow.EmitMediaFile(outPath)
	if err != nil {
		return errResult("emit PDF: " + err.Error())
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Filled %d field(s) into the PDF.", filledCount),
		"pdf":         ref,
		"filename":    filename,
		"success":     true,
	}, nil
}

// mergeFormValues takes the pdfcpu-exported form JSON and sets each field's value
// from `values` (matched by field name), coercing to the field's own value type
// (bool for checkboxes, []string for list boxes, else string). Fields absent from
// `values` keep their existing/default value. Returns the modified JSON and how
// many fields were populated.
func mergeFormValues(raw []byte, values map[string]string) ([]byte, int, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, 0, fmt.Errorf("parse form data: %w", err)
	}
	forms, _ := doc["forms"].([]interface{})
	filled := 0
	for _, f := range forms {
		form, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		// Each value is a field-type array (textfield, checkbox, datefield, …).
		for _, arr := range form {
			list, ok := arr.([]interface{})
			if !ok {
				continue
			}
			for _, e := range list {
				field, ok := e.(map[string]interface{})
				if !ok {
					continue
				}
				name, _ := field["name"].(string)
				incoming, present := values[name]
				if name == "" || !present {
					continue
				}
				setFieldValue(field, incoming)
				filled++
			}
		}
	}
	out, err := json.Marshal(doc)
	return out, filled, err
}

// setFieldValue writes `incoming` into a field's "value", coercing to match the
// field's existing value type so pdfcpu accepts it (checkbox -> bool, list box ->
// []string, everything else -> string).
func setFieldValue(field map[string]interface{}, incoming string) {
	switch field["value"].(type) {
	case bool:
		field["value"] = parseBool(incoming)
	case []interface{}:
		field["value"] = []string{incoming}
	default:
		field["value"] = incoming
	}
}

func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes", "on", "checked", "y":
		return true
	case "false", "0", "no", "off", "unchecked", "n", "":
		return false
	}
	if b, err := strconv.ParseBool(strings.TrimSpace(s)); err == nil {
		return b
	}
	return false
}
