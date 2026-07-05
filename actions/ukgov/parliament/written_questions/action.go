// Package ukgov_parliament_written_questions searches UK Parliament written
// questions and their answers. No authentication required.
//
// Note: the writtenquestions-api host now redirects to
// questions-statements-api.parliament.uk, which is used directly here.
package ukgov_parliament_written_questions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Written Questions"
	Description  = "Search UK Parliament written questions and answers by keyword (UK Parliament)"
	Website      = "https://www.flomation.co"
	Icon         = "landmark+comment"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var baseURL = "https://questions-statements-api.parliament.uk"

var Inputs = [...]core.Connection{
	{Name: "query", Type: core.ConnectionTypeString, Label: "Search Term", Placeholder: "e.g. NHS waiting times", Required: true},
	{Name: "max_results", Type: core.ConnectionTypeInteger, Label: "Maximum Results (1-20)", Placeholder: "20"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "questions", Type: core.ConnectionTypeObject, Label: "Written Questions"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Returned Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total Matches"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type questionValue struct {
	ID                int    `json:"id"`
	UIN               string `json:"uin"`
	House             string `json:"house"`
	DateTabled        string `json:"dateTabled"`
	QuestionText      string `json:"questionText"`
	AnsweringBodyName string `json:"answeringBodyName"`
	AnswerText        string `json:"answerText"`
	DateAnswered      string `json:"dateAnswered"`
}

type questionItem struct {
	Value questionValue `json:"value"`
}

type searchResponse struct {
	TotalResults int            `json:"totalResults"`
	Results      []questionItem `json:"results"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	query, err := ukgov_common.RequiredString("query", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A search term is required.")
	}
	maxResults := ukgov_common.OptionalInt("max_results", inputs, 20)
	if maxResults <= 0 {
		maxResults = 20
	}
	if maxResults > 20 {
		maxResults = 20
	}

	q := url.Values{}
	q.Set("searchTerm", query)
	q.Set("take", fmt.Sprintf("%d", maxResults))
	endpoint := baseURL + "/api/writtenquestions/questions?" + q.Encode()

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := ukgov_common.Fetch(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ukgov_common.ErrResult("UK Parliament request failed: %v", err)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("UK Parliament returned status %d", status)
	}

	var parsed searchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse UK Parliament response: %v", err)
	}

	return map[string]interface{}{
		"tool_result": summarise(parsed.Results, parsed.TotalResults, query),
		"questions":   parsed.Results,
		"count":       len(parsed.Results),
		"total":       parsed.TotalResults,
		"success":     true,
		"error":       "",
	}, nil
}

func summarise(results []questionItem, total int, query string) string {
	if len(results) == 0 {
		return fmt.Sprintf("No written questions found matching %q.", query)
	}
	limit := len(results)
	if limit > 3 {
		limit = 3
	}
	parts := make([]string, 0, limit)
	for _, it := range results[:limit] {
		v := it.Value
		parts = append(parts, fmt.Sprintf("[%s] %q (tabled %s)", v.AnsweringBodyName, truncate(v.QuestionText, 100), dateOnly(v.DateTabled)))
	}
	return fmt.Sprintf("Found %d written question(s) matching %q. Top: %s.", total, query, strings.Join(parts, "; "))
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "…"
}

func dateOnly(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
