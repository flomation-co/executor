// Package oracle_generativeai_rerank_text reranks a list of documents against a query using an OCI
// Generative AI rerank model, returning each document's index and relevance score, most relevant
// first.
package oracle_generativeai_rerank_text

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	gai "flomation.app/automate/executor/actions/oracle/generativeai"

	"github.com/oracle/oci-go-sdk/v65/generativeaiinference"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Generative AI: Rerank Text"
	Description  = "Rerank a list of documents by relevance to a query using an OCI Generative AI rerank model, returning each document's original index and relevance score."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+robot"
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
	{Name: "model_id", Type: core.ConnectionTypeString, Label: "Rerank Model", Placeholder: "OCID or name of a rerank model (e.g. cohere.rerank-multilingual-v3.1)", Required: true},
	{Name: "input", Type: core.ConnectionTypeText, Label: "Query", Placeholder: "The search query to rank the documents against", Required: true},
	{Name: "documents", Type: core.ConnectionTypeText, Label: "Documents", Placeholder: "One document per line — each line is ranked against the query", Required: true},
	{Name: "top_n", Type: core.ConnectionTypeString, Label: "Top N", Placeholder: "How many of the most relevant documents to return (optional, defaults to all)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "ranks", Type: core.ConnectionTypeObject, Label: "Document Ranks"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := gai.InferenceClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}
	modelID, err := gai.RequiredString("model_id", inputs)
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}
	query, err := gai.RequiredString("input", inputs)
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}

	rawDocs, err := gai.RequiredString("documents", inputs)
	if err != nil {
		return gai.ErrorResult(err.Error()), nil
	}
	var documents []string
	for _, line := range strings.Split(rawDocs, "\n") {
		if d := strings.TrimSpace(line); d != "" {
			documents = append(documents, d)
		}
	}
	if len(documents) == 0 {
		return gai.ErrorResult("documents is required — provide at least one non-empty line"), nil
	}

	details := generativeaiinference.RerankTextDetails{
		CompartmentId: &compartment,
		ServingMode:   generativeaiinference.OnDemandServingMode{ModelId: &modelID},
		Input:         &query,
		Documents:     documents,
	}
	if n, ok, err := gai.OptionalInt("top_n", inputs); err != nil {
		return gai.ErrorResult(err.Error()), nil
	} else if ok {
		details.TopN = &n
	}

	resp, err := client.RerankText(gai.Context(), generativeaiinference.RerankTextRequest{RerankTextDetails: details})
	if err != nil {
		return gai.ErrorResult(auth.OCIError(err)), nil
	}

	ranks := make([]map[string]interface{}, 0, len(resp.DocumentRanks))
	for _, r := range resp.DocumentRanks {
		entry := map[string]interface{}{
			"index":           gai.IntOrNil(r.Index),
			"relevance_score": floatOrNil(r.RelevanceScore),
		}
		if r.Document != nil {
			entry["text"] = gai.Str(r.Document.Text)
		}
		ranks = append(ranks, entry)
	}

	return gai.Result(
		fmt.Sprintf("Reranked %d document(s) against the query, returning %d ranked result(s)", len(documents), len(ranks)),
		map[string]interface{}{
			"ranks":         ranks,
			"id":            gai.Str(resp.Id),
			"model_id":      gai.Str(resp.ModelId),
			"model_version": gai.Str(resp.ModelVersion),
		},
	), nil
}

func floatOrNil(p *float64) interface{} {
	if p == nil {
		return nil
	}
	return *p
}
