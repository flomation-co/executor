// Package ukgov_ons_get_timeseries fetches an ONS economic timeseries (e.g. CPI
// inflation) by its series ID (CDID) and dataset. No authentication required.
//
// Note: the old api.ons.gov.uk host was decommissioned; this uses the
// www.ons.gov.uk timeseries data endpoint, which needs the topic "section"
// path (e.g. economy/inflationandpriceindices) in addition to the CDID/dataset.
package ukgov_ons_get_timeseries

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	ukgov_common "flomation.app/automate/executor/actions/ukgov"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Timeseries"
	Description  = "Fetch an ONS economic timeseries (inflation, GDP, etc.) by series ID and dataset (ONS)"
	Website      = "https://www.flomation.co"
	Icon         = "chart-line"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

// baseURL is the ONS website timeseries root. Package variable so tests can
// point it at a mock server.
var baseURL = "https://www.ons.gov.uk"

// defaultSection is the ONS topic path for the most common series (inflation).
const defaultSection = "economy/inflationandpriceindices"

var Inputs = [...]core.Connection{
	{Name: "cdid", Type: core.ConnectionTypeString, Label: "Series ID (CDID)", Placeholder: "e.g. L55O (CPIH), D7G7 (CPI)", Required: true},
	{Name: "dataset", Type: core.ConnectionTypeString, Label: "Dataset", Placeholder: "e.g. mm23", Required: true},
	{Name: "section", Type: core.ConnectionTypeString, Label: "Topic Path (optional)", Placeholder: defaultSection},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "latest_value", Type: core.ConnectionTypeString, Label: "Latest Value"},
	{Name: "latest_period", Type: core.ConnectionTypeString, Label: "Latest Period"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "unit", Type: core.ConnectionTypeString, Label: "Unit"},
	{Name: "recent", Type: core.ConnectionTypeObject, Label: "Recent Points"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type description struct {
	Title       string `json:"title"`
	CDID        string `json:"cdid"`
	Unit        string `json:"unit"`
	PreUnit     string `json:"preUnit"`
	Date        string `json:"date"`
	Number      string `json:"number"`
	DatasetID   string `json:"datasetId"`
	ReleaseDate string `json:"releaseDate"`
	NextRelease string `json:"nextRelease"`
}

type point struct {
	Date    string `json:"date"`
	Value   string `json:"value"`
	Year    string `json:"year"`
	Month   string `json:"month"`
	Quarter string `json:"quarter"`
}

type tsResponse struct {
	Description description `json:"description"`
	Months      []point     `json:"months"`
	Quarters    []point     `json:"quarters"`
	Years       []point     `json:"years"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	cdid, err := ukgov_common.RequiredString("cdid", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A series ID (CDID) is required.")
	}
	dataset, err := ukgov_common.RequiredString("dataset", inputs)
	if err != nil {
		return ukgov_common.ErrResult("A dataset is required.")
	}
	section := strings.Trim(strings.TrimSpace(ukgov_common.OptionalString("section", inputs)), "/")
	if section == "" {
		section = defaultSection
	}

	endpoint := fmt.Sprintf("%s/%s/timeseries/%s/%s/data", baseURL, section, strings.ToLower(cdid), strings.ToLower(dataset))

	ctx := context.Background()
	if flow != nil {
		ctx = flow.GoContext()
	}

	status, body, err := ukgov_common.Fetch(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ukgov_common.ErrResult("ONS request failed: %v", err)
	}
	if status == http.StatusNotFound {
		return ukgov_common.ErrResult("No ONS timeseries found for CDID %q in dataset %q (check the topic path).", cdid, dataset)
	}
	if status != http.StatusOK {
		return ukgov_common.ErrResult("ONS returned status %d", status)
	}

	var parsed tsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ukgov_common.ErrResult("Failed to parse ONS response: %v", err)
	}

	d := parsed.Description
	latestVal := d.PreUnit + d.Number + d.Unit
	recent := lastN(parsed.Months, 12)

	summary := fmt.Sprintf("%s — latest %s (%s).", d.Title, latestVal, d.Date)
	if d.NextRelease != "" {
		summary += " Next release: " + d.NextRelease + "."
	}

	return map[string]interface{}{
		"tool_result":   summary,
		"latest_value":  latestVal,
		"latest_period": d.Date,
		"title":         d.Title,
		"unit":          d.Unit,
		"recent":        recent,
		"success":       true,
		"error":         "",
	}, nil
}

// lastN returns the final n elements of points (ONS orders oldest-first, so the
// tail is the most recent data).
func lastN(points []point, n int) []point {
	if len(points) <= n {
		return points
	}
	return points[len(points)-n:]
}
