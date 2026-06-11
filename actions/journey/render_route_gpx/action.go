package journey_render_route_gpx

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	journey "flomation.app/automate/executor/actions/journey"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Export Route as GPX"
	Description  = "Convert a route polyline into GPX XML for import into Garmin, Strava, Komoot and other GPS apps."
	Website      = "https://www.flomation.co"
	Icon         = "route+file-export"
	Date         = "11/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "polyline",
		Type:        core.ConnectionTypeString,
		Label:       "Encoded polyline (from calculate_route)",
		Placeholder: "${parent.polyline}",
		Required:    true,
	},
	{
		Name:        "name",
		Type:        core.ConnectionTypeString,
		Label:       "Route name",
		Placeholder: "London to Manchester",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "gpx_xml", Type: core.ConnectionTypeText, Label: "GPX XML"},
	{Name: "gpx_base64", Type: core.ConnectionTypeString, Label: "GPX (base64)"},
	{Name: "point_count", Type: core.ConnectionTypeString, Label: "Decoded point count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// Local types kept inline because the GPX schema is small and unique to this
// action — no need to share via journey_common.

type gpxRoot struct {
	XMLName  xml.Name `xml:"gpx"`
	Version  string   `xml:"version,attr"`
	Creator  string   `xml:"creator,attr"`
	XMLNS    string   `xml:"xmlns,attr"`
	Metadata gpxMeta  `xml:"metadata"`
	Trk      gpxTrack `xml:"trk"`
}

type gpxMeta struct {
	Name string `xml:"name,omitempty"`
	Time string `xml:"time"`
}

type gpxTrack struct {
	Name string     `xml:"name,omitempty"`
	Seg  gpxSegment `xml:"trkseg"`
}

type gpxSegment struct {
	Points []gpxPoint `xml:"trkpt"`
}

type gpxPoint struct {
	Lat float64 `xml:"lat,attr"`
	Lon float64 `xml:"lon,attr"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	polyline, err := journey.RequiredString("polyline", inputs)
	if err != nil {
		return nil, err
	}
	name := journey.OptionalString("name", inputs)

	coords := journey.DecodePolyline(polyline)
	if len(coords) == 0 {
		errMsg := "polyline decoded to zero points — input may be invalid"
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("Failed to export GPX: %s", errMsg),
			"success":     false,
			"error":       errMsg,
		}, fmt.Errorf("journey: %s", errMsg)
	}

	pts := make([]gpxPoint, 0, len(coords))
	for _, c := range coords {
		pts = append(pts, gpxPoint{Lat: c.Lat, Lon: c.Lng})
	}

	root := gpxRoot{
		Version: "1.1",
		Creator: "Flomation",
		XMLNS:   "http://www.topografix.com/GPX/1/1",
		Metadata: gpxMeta{
			Name: name,
			Time: time.Now().UTC().Format(time.RFC3339),
		},
		Trk: gpxTrack{
			Name: name,
			Seg:  gpxSegment{Points: pts},
		},
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(root); err != nil {
		return nil, fmt.Errorf("journey: encode gpx: %w", err)
	}
	gpxXML := strings.TrimRight(buf.String(), "\n") + "\n"
	gpxB64 := base64.StdEncoding.EncodeToString([]byte(gpxXML))

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Generated GPX with %d track points (%d bytes).", len(pts), len(gpxXML)),
		"gpx_xml":     gpxXML,
		"gpx_base64":  gpxB64,
		"point_count": fmt.Sprintf("%d", len(pts)),
		"success":     true,
		"error":       "",
	}, nil
}
