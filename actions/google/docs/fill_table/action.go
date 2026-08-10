// Package fill_table writes values into the cells of a table inside a native
// Google Doc — the Docs analog of writing to Sheet cells. It edits the live
// document in place via the Docs API (collaborative, no download/re-upload),
// so it's ideal for populating a question/answer grid such as a tender ITT.
package fill_table

import (
	"encoding/json"
	"fmt"
	"sort"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Fill Document Table"
	Description  = "Write values into the cells of a table in a Google Doc (by row and column)"
	Website      = "https://www.flomation.co"
	Icon         = "table+pencil"
	Date         = "10/08/2026"
	Type         = core.ActionTypeAction

	docsAPI = "https://docs.googleapis.com/v1/documents"
)

var Inputs = [...]core.Connection{
	{Name: "document_id", Type: core.ConnectionTypeString, Label: "Document ID", Required: true},
	{Name: "table_index", Type: core.ConnectionTypeInteger, Label: "Which table (0-based, in document order)"},
	{Name: "cells", Type: core.ConnectionTypeText, Label: "Cells to fill: JSON array of {\"row\":N,\"column\":N,\"text\":\"...\"} (0-based)", Required: true},
	{Name: "clear_existing", Type: core.ConnectionTypeBoolean, Label: "Clear each cell's current text before writing"},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "cells_written", Type: core.ConnectionTypeInteger, Label: "Cells Written"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// cellTarget is one requested edit from the caller.
type cellTarget struct {
	Row    int    `json:"row"`
	Column int    `json:"column"`
	Text   string `json:"text"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	docID := google.OptStr("document_id", inputs)
	if docID == "" {
		return google.ErrorResult("document_id is required")
	}
	cellsRaw := google.OptStr("cells", inputs)
	if cellsRaw == "" {
		return google.ErrorResult("cells is required")
	}
	var targets []cellTarget
	if err := json.Unmarshal([]byte(cellsRaw), &targets); err != nil {
		return google.ErrorResult(fmt.Sprintf("cells must be a JSON array of {row,column,text}: %v", err))
	}
	if len(targets) == 0 {
		return google.ErrorResult("cells is empty")
	}
	tableIndex := google.OptInt("table_index", inputs, 0)
	// OptInt clamps <=0 to the default; table_index 0 is legitimate, so read it
	// directly to allow an explicit 0.
	if idx := core.FindConnection("table_index", inputs); idx != nil && idx.Number() != nil {
		tableIndex = int(*idx.Number())
	}
	clearExisting := google.OptBool("clear_existing", inputs)

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

	// Read the current document so we can resolve cell indices.
	status, body, err := google.DoRequest(flow, "GET", fmt.Sprintf("%s/%s", docsAPI, docID), token.AccessToken, nil)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			google.HandleAuthError(flow, token.Email, status)
		}
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(body)))
	}

	edits, err := resolveCellEdits(body, tableIndex, targets)
	if err != nil {
		return google.ErrorResult(err.Error())
	}

	requests := buildRequests(edits, clearExisting)
	payload, _ := json.Marshal(map[string]interface{}{"requests": requests})

	endpoint := fmt.Sprintf("%s/%s:batchUpdate", docsAPI, docID)
	status, body, err = google.DoRequest(flow, "POST", endpoint, token.AccessToken, payload)
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
		"tool_result":   fmt.Sprintf("Wrote %d cell(s) into table %d of document %s", len(edits), tableIndex, docID),
		"cells_written": int64(len(edits)),
		"success":       true,
		"error":         "",
	}, nil
}

// ─── pure, unit-testable index resolution ──────────────────────────────

type cellEdit struct {
	Start int
	End   int
	Text  string
}

type sElem struct {
	StartIndex int `json:"startIndex"`
	EndIndex   int `json:"endIndex"`
	Table      *struct {
		TableRows []struct {
			TableCells []struct {
				Content []sElem `json:"content"`
			} `json:"tableCells"`
		} `json:"tableRows"`
	} `json:"table,omitempty"`
}

// resolveCellEdits maps (row,column) targets in the tableIndex-th table to the
// concrete character ranges the Docs API needs. Cells are located from the
// document's structural indices; the returned edits are sorted by DESCENDING
// start index so they can be applied in one batchUpdate without earlier edits
// shifting the indices of later ones.
func resolveCellEdits(docJSON []byte, tableIndex int, targets []cellTarget) ([]cellEdit, error) {
	var doc struct {
		Body struct {
			Content []sElem `json:"content"`
		} `json:"body"`
	}
	if err := json.Unmarshal(docJSON, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse document: %v", err)
	}

	// Collect tables in document order.
	var tables []*struct {
		TableRows []struct {
			TableCells []struct {
				Content []sElem `json:"content"`
			} `json:"tableCells"`
		} `json:"tableRows"`
	}
	for i := range doc.Body.Content {
		if doc.Body.Content[i].Table != nil {
			tables = append(tables, doc.Body.Content[i].Table)
		}
	}
	if tableIndex < 0 || tableIndex >= len(tables) {
		return nil, fmt.Errorf("table %d not found — the document has %d table(s)", tableIndex, len(tables))
	}
	tbl := tables[tableIndex]

	edits := make([]cellEdit, 0, len(targets))
	for _, t := range targets {
		if t.Row < 0 || t.Row >= len(tbl.TableRows) {
			return nil, fmt.Errorf("row %d out of range (table has %d row(s))", t.Row, len(tbl.TableRows))
		}
		cells := tbl.TableRows[t.Row].TableCells
		if t.Column < 0 || t.Column >= len(cells) {
			return nil, fmt.Errorf("column %d out of range (row %d has %d column(s))", t.Column, t.Row, len(cells))
		}
		content := cells[t.Column].Content
		if len(content) == 0 {
			return nil, fmt.Errorf("cell (%d,%d) has no content index", t.Row, t.Column)
		}
		start := content[0].StartIndex
		end := content[len(content)-1].EndIndex
		edits = append(edits, cellEdit{Start: start, End: end, Text: t.Text})
	}

	// Highest index first so a batchUpdate applies cleanly.
	sort.Slice(edits, func(i, j int) bool { return edits[i].Start > edits[j].Start })
	return edits, nil
}

// buildRequests turns resolved edits into Docs batchUpdate requests. When
// clearing, the cell's existing text is deleted first but its mandatory
// trailing newline (End-1) is preserved — deleting it would corrupt the table.
func buildRequests(edits []cellEdit, clearExisting bool) []map[string]interface{} {
	var requests []map[string]interface{}
	for _, e := range edits {
		if clearExisting && e.End-1 > e.Start {
			requests = append(requests, map[string]interface{}{
				"deleteContentRange": map[string]interface{}{
					"range": map[string]interface{}{
						"startIndex": e.Start,
						"endIndex":   e.End - 1,
					},
				},
			})
		}
		requests = append(requests, map[string]interface{}{
			"insertText": map[string]interface{}{
				"location": map[string]interface{}{"index": e.Start},
				"text":     e.Text,
			},
		})
	}
	return requests
}
