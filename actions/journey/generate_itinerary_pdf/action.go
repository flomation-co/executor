package journey_generate_itinerary_pdf

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	journey "flomation.app/automate/executor/actions/journey"
	"github.com/go-pdf/fpdf"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Generate Itinerary PDF"
	Description  = "Build a printable PDF with a map, distance/duration summary, and turn-by-turn directions for a route."
	Website      = "https://www.flomation.co"
	Icon         = "route+file-pdf"
	Date         = "11/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "provider",
		Type:        core.ConnectionTypeString,
		Label:       "Map Provider",
		Placeholder: "google",
		Options: []core.ConnectionOption{
			{Name: "Google Maps", Value: "google"},
			{Name: "Mapbox", Value: "mapbox"},
		},
		Required: true,
	},
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeString,
		Label:       "Provider API Key",
		Placeholder: "${secrets.GOOGLE_MAPS_API_KEY}",
		Required:    true,
	},
	{
		Name:        "title",
		Type:        core.ConnectionTypeString,
		Label:       "Itinerary title",
		Placeholder: "London to Manchester",
	},
	{
		Name:        "polyline",
		Type:        core.ConnectionTypeString,
		Label:       "Encoded polyline",
		Placeholder: "${parent.polyline}",
		Required:    true,
	},
	{
		Name:        "distance_miles",
		Type:        core.ConnectionTypeString,
		Label:       "Distance (miles, from calculate_route)",
		Placeholder: "${parent.distance_miles}",
	},
	{
		Name:        "duration_friendly",
		Type:        core.ConnectionTypeString,
		Label:       "Duration (friendly, from calculate_route)",
		Placeholder: "${parent.duration_friendly}",
	},
	{
		Name:        "steps_json",
		Type:        core.ConnectionTypeText,
		Label:       "Steps JSON (from calculate_route.steps)",
		Placeholder: "${parent.steps}",
	},
	{
		Name:        "include_map",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Include map image",
	},
	{
		Name:        "include_directions",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Include turn-by-turn directions",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "pdf_base64", Type: core.ConnectionTypeString, Label: "PDF (base64)"},
	{Name: "page_count", Type: core.ConnectionTypeString, Label: "Page count"},
	{Name: "size_kb", Type: core.ConnectionTypeString, Label: "Size (KB)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

type stepItem struct {
	Instruction     string  `json:"instruction"`
	DistanceMetres  float64 `json:"distance_metres"`
	DurationSeconds float64 `json:"duration_seconds"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	provider, err := journey.ProviderInput(inputs)
	if err != nil {
		return nil, err
	}
	polyline, err := journey.RequiredString("polyline", inputs)
	if err != nil {
		return nil, err
	}

	title := journey.OptionalString("title", inputs)
	if title == "" {
		title = "Route Itinerary"
	}
	distance := journey.OptionalString("distance_miles", inputs)
	duration := journey.OptionalString("duration_friendly", inputs)
	stepsRaw := journey.OptionalString("steps_json", inputs)

	includeMap := optionalBool("include_map", inputs, true)
	includeDirections := optionalBool("include_directions", inputs, true)

	var imgBytes []byte
	if includeMap {
		img, _, mapErr := provider.RenderStaticMap(journey.StaticMapParams{
			Polyline: polyline,
			Width:    600,
			Height:   400,
		})
		if mapErr != nil {
			return errResult(title, mapErr)
		}
		imgBytes = img
	}

	var steps []stepItem
	if includeDirections && stepsRaw != "" {
		// Surface parse errors rather than silently dropping directions —
		// the previous "be lenient" approach hid the bug where upstream
		// substitution produced Go syntax instead of JSON. If a flow author
		// has wired steps_json incorrectly, they should know.
		if err := json.Unmarshal([]byte(stepsRaw), &steps); err != nil {
			return errResult(title, fmt.Errorf("steps_json is not valid JSON: %w (got %d bytes starting with %q)",
				err, len(stepsRaw), stepsRaw[:min(40, len(stepsRaw))]))
		}
	}

	pdf := buildPDF(title, distance, duration, imgBytes, steps)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return errResult(title, fmt.Errorf("write pdf: %w", err))
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	sizeKB := float64(buf.Len()) / 1024.0

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Generated %s PDF (%.1f KB, %d page(s)).", title, sizeKB, pdf.PageCount()),
		"pdf_base64":  encoded,
		"page_count":  fmt.Sprintf("%d", pdf.PageCount()),
		"size_kb":     fmt.Sprintf("%.1f", sizeKB),
		"success":     true,
		"error":       "",
	}, nil
}

func buildPDF(title, distance, duration string, mapImage []byte, steps []stepItem) *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 22)
	pdf.SetTextColor(70, 0, 112) // brand purple #460070
	pdf.CellFormat(0, 12, title, "", 1, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "", 11)
	pdf.SetTextColor(80, 80, 80)
	summary := []string{}
	if distance != "" {
		summary = append(summary, fmt.Sprintf("Distance: %s miles", distance))
	}
	if duration != "" {
		summary = append(summary, fmt.Sprintf("Duration: %s", duration))
	}
	if len(summary) > 0 {
		pdf.CellFormat(0, 6, strings.Join(summary, "   |   "), "", 1, "L", false, 0, "")
	}
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(6)

	if len(mapImage) > 0 {
		opt := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}
		pdf.RegisterImageOptionsReader("route_map", opt, bytes.NewReader(mapImage))
		// Image is 600x400 → aspect 1.5 → 190mm wide ⇒ ~127mm tall. Use
		// explicit height so the cursor lands at a predictable position
		// (auto-height + Ln() of a wrong constant is what put steps over
		// the map in the original build).
		const mapWidthMM = 190.0
		const mapHeightMM = 127.0
		mapY := pdf.GetY()
		pdf.ImageOptions("route_map", 10, mapY, mapWidthMM, mapHeightMM, false, opt, 0, "")
		pdf.SetY(mapY + mapHeightMM + 12) // 12mm breathing room below the map
	}

	if len(steps) > 0 {
		renderStepsTable(pdf, steps)
	}

	return pdf
}

// renderStepsTable lays out the turn-by-turn directions as a four-column
// table: Step | Direction | Distance | Duration. Direction wraps onto
// multiple lines when long; the other columns share the wrapped row height
// so cell borders align.
func renderStepsTable(pdf *fpdf.Fpdf, steps []stepItem) {
	const (
		leftMargin    = 10.0
		colStepW      = 14.0
		colDirW       = 108.0
		colDistW      = 28.0
		colDurW       = 40.0
		lineHeight    = 5.0
		headerHeight  = 7.0
		cellPad       = 1.5
		pageBottomY   = 280.0
	)

	pdf.SetFont("Helvetica", "B", 13)
	pdf.SetTextColor(0, 170, 156) // brand teal
	pdf.CellFormat(0, 8, "Turn-by-turn directions", "", 1, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(2)

	drawHeader := func() {
		pdf.SetFillColor(70, 0, 112) // brand purple
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 10)
		pdf.SetX(leftMargin)
		pdf.CellFormat(colStepW, headerHeight, "Step", "1", 0, "C", true, 0, "")
		pdf.CellFormat(colDirW, headerHeight, "Direction", "1", 0, "L", true, 0, "")
		pdf.CellFormat(colDistW, headerHeight, "Distance", "1", 0, "R", true, 0, "")
		pdf.CellFormat(colDurW, headerHeight, "Duration", "1", 1, "R", true, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Helvetica", "", 9)
	}

	drawHeader()

	for i, s := range steps {
		lines := wrapToWidth(pdf, s.Instruction, colDirW-2*cellPad)
		if len(lines) == 0 {
			lines = []string{""}
		}
		rowHeight := float64(len(lines)) * lineHeight
		if rowHeight < headerHeight {
			rowHeight = headerHeight
		}

		// New page if this row would overflow — redraw header so the table
		// continues with its column legend intact.
		if pdf.GetY()+rowHeight > pageBottomY {
			pdf.AddPage()
			drawHeader()
		}

		xStart := leftMargin
		yStart := pdf.GetY()

		// Step number — fixed-height single-line cell.
		pdf.SetXY(xStart, yStart)
		pdf.CellFormat(colStepW, rowHeight, fmt.Sprintf("%d", i+1), "1", 0, "C", false, 0, "")

		// Direction — MultiCell wraps onto N lines and advances the cursor
		// past the cell. We don't use that advance; we'll SetXY back.
		pdf.SetXY(xStart+colStepW, yStart)
		pdf.MultiCell(colDirW, lineHeight, strings.Join(lines, "\n"), "1", "L", false)

		// Distance / Duration — return to row Y, draw past the direction cell.
		distMi := s.DistanceMetres / journey.MetresPerMile
		distText := ""
		if s.DistanceMetres > 0 {
			distText = fmt.Sprintf("%.1f mi", distMi)
		}
		durText := ""
		if s.DurationSeconds > 0 {
			durText = journey.FriendlyDuration(s.DurationSeconds)
		}
		pdf.SetXY(xStart+colStepW+colDirW, yStart)
		pdf.CellFormat(colDistW, rowHeight, distText, "1", 0, "R", false, 0, "")
		pdf.CellFormat(colDurW, rowHeight, durText, "1", 0, "R", false, 0, "")

		// Advance to the next row.
		pdf.SetXY(xStart, yStart+rowHeight)
	}
}

// wrapToWidth splits text onto multiple lines so each line fits within the
// given width when rendered in the current font. fpdf's MultiCell handles
// wrapping internally, but we need to KNOW the line count up-front to size
// the sibling fixed-height cells.
func wrapToWidth(pdf *fpdf.Fpdf, text string, width float64) []string {
	if text == "" {
		return []string{""}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := words[0]
	for _, w := range words[1:] {
		candidate := current + " " + w
		if pdf.GetStringWidth(candidate) <= width {
			current = candidate
		} else {
			lines = append(lines, current)
			current = w
		}
	}
	lines = append(lines, current)
	return lines
}

func optionalBool(name string, inputs []*core.Connection, def bool) bool {
	c := core.FindConnection(name, inputs)
	if c == nil || c.Boolean() == nil {
		return def
	}
	return *c.Boolean()
}

func errResult(title string, err error) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Failed to generate %s PDF: %s", title, err.Error()),
		"success":     false,
		"error":       err.Error(),
	}, err
}
