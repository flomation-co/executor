// Package burn_text burns a text caption onto a video, backed by ffmpeg's drawtext
// filter. The caption is passed via a file (textfile=) so no escaping/injection is
// possible, and a system font is located automatically.
package burn_text

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	core "flomation.app/automate/executor"
	vc "flomation.app/automate/executor/actions/video"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Burn Text onto Video"
	Description  = "Overlay a text caption onto a video, burned into the picture"
	Website      = "https://www.flomation.co"
	Icon         = "i-cursor"
	Date         = "15/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (file/blob reference)", Placeholder: "flo:file:… or flo:blob:…", Required: true},
	{Name: "text", Type: core.ConnectionTypeText, Label: "Caption text", Placeholder: "Your caption", Required: true},
	{Name: "font_size", Type: core.ConnectionTypeInteger, Label: "Font size (px)", Value: 32},
	{Name: "colour", Type: core.ConnectionTypeString, Label: "Text colour", Value: "white", Placeholder: "white, #ff0000"},
	{
		Name: "position", Type: core.ConnectionTypeString, Label: "Position", Value: "bottom",
		Options: []core.ConnectionOption{
			{Name: "Top", Value: "top"},
			{Name: "Centre", Value: "center"},
			{Name: "Bottom", Value: "bottom"},
		},
	},
	{Name: "font_file", Type: core.ConnectionTypeString, Label: "Font file path (optional override)", Placeholder: "/path/to/font.ttf"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "video", Type: core.ConnectionTypeString, Label: "Video (media reference)"},
	{Name: "size_bytes", Type: core.ConnectionTypeInteger, Label: "Size (bytes)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// commonFonts are searched (in order) when no font_file override is given.
var commonFonts = []string{
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
	"/Library/Fonts/Arial.ttf",
	"/System/Library/Fonts/Supplemental/Arial.ttf",
	"/System/Library/Fonts/Helvetica.ttc",
}

func findFont(override string) (string, bool) {
	if override != "" {
		if _, err := os.Stat(override); err == nil {
			return override, true
		}
		return "", false
	}
	for _, f := range commonFonts {
		if _, err := os.Stat(f); err == nil {
			return f, true
		}
	}
	return "", false
}

// yExpr maps a vertical position to the drawtext y expression (x is always centred).
func yExpr(position string) string {
	switch position {
	case "top":
		return "40"
	case "center":
		return "(h-text_h)/2"
	default: // bottom
		return "h-text_h-40"
	}
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	video, err := vc.RequiredString("video", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	text, err := vc.RequiredString("text", inputs)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	fontSize := vc.OptionalInt("font_size", 32, inputs)
	if fontSize < 1 {
		fontSize = 32
	}
	colour := vc.OptionalStringDefault("colour", "white", inputs)
	position := vc.OptionalStringDefault("position", "bottom", inputs)

	font, ok := findFont(vc.OptionalString("font_file", inputs))
	if !ok {
		return vc.ErrResult("no usable font found — install a TTF font on the runner or set font_file")
	}

	inPath, _, err := flow.ResolveToLocalFile(video)
	if err != nil {
		return vc.ErrResult("could not read the input video: " + err.Error())
	}
	ext := filepath.Ext(inPath)
	if ext == "" {
		ext = ".mp4"
	}

	// The caption goes in a file so drawtext never has to parse/escape it.
	textPath, err := flow.MediaScratchFile("txt")
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	if err := os.WriteFile(textPath, []byte(text), 0o600); err != nil {
		return vc.ErrResult(err.Error())
	}
	outPath, err := flow.MediaScratchFile(ext)
	if err != nil {
		return vc.ErrResult(err.Error())
	}

	ctx, cancel := context.WithTimeout(flow.GoContext(), vc.MaxTimeout)
	defer cancel()

	drawtext := fmt.Sprintf("drawtext=textfile='%s':fontfile='%s':fontsize=%d:fontcolor=%s:x=(w-text_w)/2:y=%s:box=1:boxcolor=black@0.4:boxborderw=10",
		textPath, font, fontSize, colour, yExpr(position))
	stderr, err := vc.RunFFmpeg(ctx, "-y", "-i", inPath, "-vf", drawtext, "-c:a", "copy", outPath)
	if err != nil {
		return vc.ErrResult(fmt.Sprintf("ffmpeg failed: %v: %s", err, vc.Tail(stderr, 400)))
	}

	ref, err := flow.EmitMediaFile(outPath)
	if err != nil {
		return vc.ErrResult(err.Error())
	}
	var size int64
	if fi, e := os.Stat(outPath); e == nil {
		size = fi.Size()
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Burned the caption at the %s (%d bytes).", position, size),
		"video":       ref,
		"size_bytes":  size,
		"success":     true,
	}, nil
}
