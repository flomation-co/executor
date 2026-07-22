// Package oracle_language_batch_detect_language_sentiments runs OCI Language pre-trained
// sentiment analysis over a single piece of text, returning the document-level sentiment and
// score plus any aspect- and sentence-level breakdown the service produced.
package oracle_language_batch_detect_language_sentiments

import (
	"fmt"

	core "flomation.app/automate/executor"
	lang "flomation.app/automate/executor/actions/oracle/language"

	"github.com/oracle/oci-go-sdk/v65/ailanguage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Language: Detect Sentiment"
	Description  = "Analyse the sentiment of a piece of text with OCI Language's pre-trained model — overall sentiment, confidence scores and any aspect/sentence breakdown."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+comments"
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
	{Name: "text", Type: core.ConnectionTypeString, Label: "Text", Placeholder: "The text to analyse for sentiment", Required: true},
	{Name: "language_code", Type: core.ConnectionTypeString, Label: "Language Code", Placeholder: "Optional — e.g. en, es, fr (defaults to auto-detect)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Sentiment Results"},
	{Name: "document_sentiment", Type: core.ConnectionTypeString, Label: "Document Sentiment"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := lang.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return lang.ErrorResult(err.Error()), nil
	}
	text, err := lang.RequiredString("text", inputs)
	if err != nil {
		return lang.ErrorResult(err.Error()), nil
	}
	key := "doc1"
	doc := ailanguage.TextDocument{Key: &key, Text: &text}
	if code := lang.OptionalString("language_code", inputs); code != "" {
		doc.LanguageCode = &code
	}

	resp, err := client.BatchDetectLanguageSentiments(lang.Context(), ailanguage.BatchDetectLanguageSentimentsRequest{
		BatchDetectLanguageSentimentsDetails: ailanguage.BatchDetectLanguageSentimentsDetails{
			CompartmentId: &compartment,
			Documents:     []ailanguage.TextDocument{doc},
		},
	})
	if err != nil {
		return lang.ErrorResult(auth.OCIError(err)), nil
	}

	results := make([]map[string]interface{}, 0, len(resp.Documents))
	for i := range resp.Documents {
		results = append(results, summariseSentimentDocument(&resp.Documents[i]))
	}
	errs := make([]map[string]interface{}, 0, len(resp.Errors))
	for i := range resp.Errors {
		errs = append(errs, map[string]interface{}{
			"key":     lang.Str(resp.Errors[i].Key),
			"message": documentErrorMessage(resp.Errors[i].Error),
		})
	}

	sentiment := ""
	if len(resp.Documents) > 0 {
		sentiment = lang.Str(resp.Documents[0].DocumentSentiment)
	}
	msg := "Sentiment analysis returned no document result"
	if sentiment != "" {
		msg = fmt.Sprintf("Sentiment: %s", sentiment)
	} else if len(errs) > 0 {
		msg = fmt.Sprintf("Sentiment analysis failed for the document: %s", errs[0]["message"])
	}

	return lang.Result(msg, map[string]interface{}{
		"results":            results,
		"errors":             errs,
		"document_sentiment": sentiment,
	}), nil
}

func summariseSentimentDocument(d *ailanguage.SentimentDocumentResult) map[string]interface{} {
	aspects := make([]map[string]interface{}, 0, len(d.Aspects))
	for i := range d.Aspects {
		aspects = append(aspects, map[string]interface{}{
			"offset":    lang.IntOrNil(d.Aspects[i].Offset),
			"length":    lang.IntOrNil(d.Aspects[i].Length),
			"text":      lang.Str(d.Aspects[i].Text),
			"sentiment": lang.Str(d.Aspects[i].Sentiment),
			"scores":    d.Aspects[i].Scores,
		})
	}
	sentences := make([]map[string]interface{}, 0, len(d.Sentences))
	for i := range d.Sentences {
		sentences = append(sentences, map[string]interface{}{
			"offset":    lang.IntOrNil(d.Sentences[i].Offset),
			"length":    lang.IntOrNil(d.Sentences[i].Length),
			"text":      lang.Str(d.Sentences[i].Text),
			"sentiment": lang.Str(d.Sentences[i].Sentiment),
			"scores":    d.Sentences[i].Scores,
		})
	}
	return map[string]interface{}{
		"key":                lang.Str(d.Key),
		"language_code":      lang.Str(d.LanguageCode),
		"document_sentiment": lang.Str(d.DocumentSentiment),
		"document_scores":    d.DocumentScores,
		"aspects":            aspects,
		"sentences":          sentences,
	}
}

func documentErrorMessage(e *ailanguage.ErrorDetails) string {
	if e == nil {
		return ""
	}
	return lang.Str(e.Message)
}
