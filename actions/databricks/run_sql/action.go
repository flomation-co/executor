package databricks_run_sql

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	core "flomation.app/automate/executor"
	databricks "flomation.app/automate/executor/actions/databricks"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Flomation"
	Organisation = "Flomation"
	Name         = "Databricks Run SQL"
	Description  = "Execute a SQL statement against a Databricks SQL Warehouse and return the rows"
	Website      = "https://www.flomation.co"
	Icon         = "database+play"
	Date         = "24/06/2026"
	Type         = core.ActionTypeAction
)

const (
	// pollInterval is how long we wait between status checks once the initial
	// server-side wait (handled via wait_timeout) has elapsed.
	pollInterval = 2 * time.Second

	// defaultMaxWait bounds how long we block waiting for a statement to finish
	// before giving up and cancelling it, when no timeout_seconds is supplied.
	defaultMaxWait = 5 * time.Minute
)

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Workspace URL", Placeholder: "https://dbc-xxxxxxxx.cloud.databricks.com", Required: true},
	{Name: "token", Type: core.ConnectionTypeSecret, Label: "Access Token (PAT)", Placeholder: "dapi...", Required: true},
	{Name: "warehouse_id", Type: core.ConnectionTypeString, Label: "SQL Warehouse ID", Placeholder: "1234567890abcdef", Required: true},
	{Name: "statement", Type: core.ConnectionTypeText, Label: "SQL Statement", Placeholder: "SELECT * FROM samples.nyctaxi.trips LIMIT 100", Required: true},
	{Name: "catalog", Type: core.ConnectionTypeString, Label: "Catalog", Placeholder: "main"},
	{Name: "schema", Type: core.ConnectionTypeString, Label: "Schema", Placeholder: "default"},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "Default 300"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Results"},
	{Name: "row_count", Type: core.ConnectionTypeInteger, Label: "Row Count"},
	{Name: "columns", Type: core.ConnectionTypeObject, Label: "Columns"},
}

// statementResponse models the parts of the SQL Statement Execution API
// response we care about. See:
// https://docs.databricks.com/api/workspace/statementexecution
type statementResponse struct {
	StatementID string `json:"statement_id"`
	Status      struct {
		State string `json:"state"` // PENDING, RUNNING, SUCCEEDED, FAILED, CANCELED, CLOSED
		Error *struct {
			ErrorCode string `json:"error_code"`
			Message   string `json:"message"`
		} `json:"error"`
	} `json:"status"`
	Manifest *struct {
		Schema struct {
			Columns []columnInfo `json:"columns"`
		} `json:"schema"`
		TotalRowCount int64 `json:"total_row_count"`
	} `json:"manifest"`
	Result *resultChunk `json:"result"`
}

type columnInfo struct {
	Name     string `json:"name"`
	TypeText string `json:"type_text"`
	Position int    `json:"position"`
}

type resultChunk struct {
	RowCount              int64       `json:"row_count"`
	DataArray             [][]*string `json:"data_array"`
	NextChunkInternalLink string      `json:"next_chunk_internal_link"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	host, token, err := databricks.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	warehouseID, err := databricks.RequiredString("warehouse_id", inputs)
	if err != nil {
		return nil, err
	}
	statement, err := databricks.RequiredString("statement", inputs)
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"warehouse_id":    warehouseID,
		"statement":       statement,
		"wait_timeout":    "30s",
		"on_wait_timeout": "CONTINUE",
		"format":          "JSON_ARRAY",
		"disposition":     "INLINE",
	}
	if catalog := databricks.OptionalString("catalog", inputs); catalog != "" {
		body["catalog"] = catalog
	}
	if schema := databricks.OptionalString("schema", inputs); schema != "" {
		body["schema"] = schema
	}

	log.WithFields(log.Fields{
		"warehouse_id": warehouseID,
	}).Info("Submitting Databricks SQL statement")

	resp, err := databricks.ExecuteAPI(host, token, http.MethodPost, "/api/2.0/sql/statements/", body)
	if err != nil {
		return nil, err
	}
	if err := databricks.CheckResponse(resp); err != nil {
		return nil, err
	}

	var stmt statementResponse
	if err := json.Unmarshal(resp.Body, &stmt); err != nil {
		return nil, fmt.Errorf("failed to parse statement response: %w", err)
	}

	// Poll until the statement reaches a terminal state, bounded by an overall
	// deadline so a hung statement can never hang the executor goroutine.
	maxWait := defaultMaxWait
	if secs, ok := databricks.OptionalInt("timeout_seconds", inputs); ok && secs > 0 {
		maxWait = time.Duration(secs) * time.Second
	}
	deadline := time.Now().Add(maxWait)
	for stmt.Status.State == "PENDING" || stmt.Status.State == "RUNNING" {
		if time.Now().After(deadline) {
			// Best-effort cancel so we don't leave it running on the warehouse.
			_, _ = databricks.ExecuteAPI(host, token, http.MethodPost,
				"/api/2.0/sql/statements/"+stmt.StatementID+"/cancel", nil)
			return nil, fmt.Errorf("statement did not complete within %s", maxWait)
		}
		time.Sleep(pollInterval)

		pollResp, err := databricks.ExecuteAPI(host, token, http.MethodGet,
			"/api/2.0/sql/statements/"+stmt.StatementID, nil)
		if err != nil {
			return nil, err
		}
		if err := databricks.CheckResponse(pollResp); err != nil {
			return nil, err
		}
		stmt = statementResponse{}
		if err := json.Unmarshal(pollResp.Body, &stmt); err != nil {
			return nil, fmt.Errorf("failed to parse statement status: %w", err)
		}
	}

	if stmt.Status.State != "SUCCEEDED" {
		msg := stmt.Status.State
		if stmt.Status.Error != nil && stmt.Status.Error.Message != "" {
			msg = fmt.Sprintf("%s: %s", stmt.Status.State, stmt.Status.Error.Message)
		}
		return nil, fmt.Errorf("statement %s", msg)
	}

	// Build the column list and collect every result chunk into row maps.
	var columns []columnInfo
	if stmt.Manifest != nil {
		columns = stmt.Manifest.Schema.Columns
	}

	rows := []map[string]interface{}{}
	chunk := stmt.Result
	for chunk != nil {
		for _, raw := range chunk.DataArray {
			row := make(map[string]interface{}, len(columns))
			for i, col := range columns {
				var val interface{}
				if i < len(raw) && raw[i] != nil {
					val = *raw[i]
				}
				row[col.Name] = val
			}
			rows = append(rows, row)
		}

		if chunk.NextChunkInternalLink == "" {
			break
		}
		nextResp, err := databricks.ExecuteAPI(host, token, http.MethodGet, chunk.NextChunkInternalLink, nil)
		if err != nil {
			return nil, err
		}
		if err := databricks.CheckResponse(nextResp); err != nil {
			return nil, err
		}
		var next resultChunk
		if err := json.Unmarshal(nextResp.Body, &next); err != nil {
			return nil, fmt.Errorf("failed to parse result chunk: %w", err)
		}
		chunk = &next
	}

	cols := make([]map[string]interface{}, 0, len(columns))
	for _, c := range columns {
		cols = append(cols, map[string]interface{}{"name": c.Name, "type": c.TypeText})
	}

	summary := fmt.Sprintf("Statement succeeded — %d row(s) returned", len(rows))

	return map[string]interface{}{
		"tool_result": summary,
		"results":     rows,
		"row_count":   len(rows),
		"columns":     cols,
	}, nil
}
