// Package read reads the content of a Google Docs document.
package read

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	google "flomation.app/automate/executor/actions/google"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Read Document"
	Description  = "Read the content of a Google Docs document"
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+eye"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction

	docsAPI = "https://docs.googleapis.com/v1/documents"
)

var Inputs = [...]core.Connection{
	{Name: "document_id", Type: core.ConnectionTypeString, Label: "Document ID", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Google Account (email)"},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google OAuth Credential", Placeholder: "${credentials.GOOGLE_DRIVE}"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Document Text (paragraphs and tables)"},
	{Name: "tables", Type: core.ConnectionTypeObject, Label: "Tables (JSON array of rows of cell text)"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "document", Type: core.ConnectionTypeString, Label: "Full Document (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	docID := google.OptStr("document_id", inputs)
	if docID == "" {
		return google.ErrorResult("document_id is required")
	}

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

	endpoint := fmt.Sprintf("%s/%s", docsAPI, docID)

	status, body, err := google.DoRequest(flow, "GET", endpoint, token.AccessToken, nil)
	if err != nil {
		return google.ErrorResult(err.Error())
	}
	if status < 200 || status >= 300 {
		if status == 401 || status == 403 {
			google.HandleAuthError(flow, token.Email, status)
		}
		// A 400 here usually means the ID points at an uploaded .docx (or other
		// non-native file), which the Docs API can't read. Steer the caller to
		// convert it to a native Google Doc first rather than dead-end.
		if status == 400 {
			return google.ErrorResult(fmt.Sprintf(
				"Google API returned 400: %s. If this is an uploaded .docx or other Office file, convert it to a native Google Doc first with the 'Convert to Google Doc' action, then read the returned document_id.",
				google.TruncateBody(body)))
		}
		return google.ErrorResult(fmt.Sprintf("Google API returned %d: %s", status, google.TruncateBody(body)))
	}

	var doc struct {
		Title string `json:"title"`
		Body  struct {
			Content []structuralElement `json:"content"`
		} `json:"body"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return google.ErrorResult(fmt.Sprintf("failed to parse document: %v", err))
	}

	// Walk paragraphs AND tables so table content (e.g. a question/answer
	// grid) is included — the previous version only read top-level paragraphs
	// and silently dropped every table.
	var textBuilder strings.Builder
	var tables [][][]string
	renderContent(doc.Body.Content, &textBuilder, &tables)
	text := strings.TrimSpace(textBuilder.String())

	return map[string]interface{}{
		"tool_result": text,
		"content":     text,
		"tables":      tables,
		"title":       doc.Title,
		"document":    string(body),
		"success":     true,
		"error":       "",
	}, nil
}

// Minimal Google Docs document model — enough to extract text from paragraphs
// and from tables (which nest arbitrarily deep via cell content).
type structuralElement struct {
	Paragraph *struct {
		Elements []struct {
			TextRun *struct {
				Content string `json:"content"`
			} `json:"textRun,omitempty"`
		} `json:"elements"`
	} `json:"paragraph,omitempty"`
	Table *struct {
		TableRows []struct {
			TableCells []struct {
				Content []structuralElement `json:"content"`
			} `json:"tableCells"`
		} `json:"tableRows"`
	} `json:"table,omitempty"`
}

// renderContent appends readable text for a run of structural elements and
// collects every table it encounters as rows of plain-text cells.
func renderContent(content []structuralElement, out *strings.Builder, tables *[][][]string) {
	for _, el := range content {
		if el.Paragraph != nil {
			for _, elem := range el.Paragraph.Elements {
				if elem.TextRun != nil {
					out.WriteString(elem.TextRun.Content)
				}
			}
		}
		if el.Table != nil {
			var rows [][]string
			for _, row := range el.Table.TableRows {
				var cells []string
				for _, cell := range row.TableCells {
					cells = append(cells, strings.TrimSpace(cellText(cell.Content)))
				}
				rows = append(rows, cells)
				out.WriteString(strings.Join(cells, " | "))
				out.WriteString("\n")
			}
			*tables = append(*tables, rows)
		}
	}
}

// cellText extracts the plain text of a single table cell (which may itself
// contain paragraphs and nested tables).
func cellText(content []structuralElement) string {
	var b strings.Builder
	var ignore [][][]string
	renderContent(content, &b, &ignore)
	return b.String()
}
