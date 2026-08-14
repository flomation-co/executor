// Package screen_info reports the desktop geometry and active window of a
// desktop VM, so an agent can confirm that the coordinates it reads off a
// screenshot are the coordinates a click will actually use.
package screen_info

import (
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	desktop "flomation.app/automate/executor/actions/desktop"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Desktop Screen Info"
	Description  = "Report a desktop VM's screen size and active window, and whether screenshot coordinates match."
	Website      = "https://www.flomation.co"
	Icon         = "expand"
	Date         = "14/08/2026"
	Type         = core.ActionTypeAction
)

// visionLongEdgeLimit is the long-edge size, in pixels, above which vision
// models downscale a submitted image before they ever see it.
//
// This is what makes a large display quietly hostile to a computer-use agent.
// The screenshot is captured at full size and the model is shown a shrunk copy,
// so every coordinate it reads is in the shrunk space while every coordinate a
// click consumes is in screen space. Nothing errors: clicks simply land short
// of where the agent aimed, by a factor it cannot observe, and the failure
// looks like "clicking does not work on this VM" rather than a unit mismatch.
const visionLongEdgeLimit = 1568

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Host", Placeholder: "10.0.0.5", Required: true},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "22"},
	{Name: "os", Type: core.ConnectionTypeString, Label: "Operating System", Options: []core.ConnectionOption{{Name: "Linux", Value: "linux"}, {Name: "Windows", Value: "windows"}}},
	{Name: "display", Type: core.ConnectionTypeString, Label: "X Display (Linux)", Placeholder: ":0"},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Required: true, Options: []core.ConnectionOption{{Name: "Private Key", Value: "key"}, {Name: "Password", Value: "password"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "SSH Private Key", Placeholder: "${secrets.DesktopVMKey}", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"key"}}},
	{Name: "passphrase", Type: core.ConnectionTypeSecret, Label: "Key Passphrase", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"key"}}},
	{Name: "password", Type: core.ConnectionTypeSecret, Label: "Password", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"password"}}},
	{Name: "host_fingerprint", Type: core.ConnectionTypeString, Label: "Host Key Fingerprint", Placeholder: "SHA256:… (optional but recommended)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "width", Type: core.ConnectionTypeInteger, Label: "Screen Width"},
	{Name: "height", Type: core.ConnectionTypeInteger, Label: "Screen Height"},
	{Name: "active_window", Type: core.ConnectionTypeString, Label: "Active Window Title"},
	{Name: "coordinates_match_screenshot", Type: core.ConnectionTypeBoolean, Label: "Screenshot Coordinates Match Screen"},
	{Name: "coordinate_multiplier", Type: core.ConnectionTypeString, Label: "Screenshot → Screen Multiplier"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	conn, err := desktop.ResolveConn(inputs)
	if err != nil {
		return desktop.ErrResult(err.Error()), nil
	}

	stdout, stderr, exit, rerr := conn.Run(desktop.ScreenInfoCmd(conn.OS, conn.Display))
	if rerr != nil {
		return desktop.ErrResult(rerr.Error()), nil
	}
	if exit != 0 {
		return desktop.ErrResult("screen info failed: " + strings.TrimSpace(stderr)), nil
	}

	width, height, activeWindow, perr := ParseScreenInfo(stdout)
	if perr != nil {
		return desktop.ErrResult(perr.Error()), nil
	}

	scale := ScreenshotScale(width, height)
	matches := scale == 1
	// What the agent actually needs is the screenshot -> screen conversion,
	// which is the reciprocal of the shrink ratio.
	multiplier := 1 / scale

	summary := fmt.Sprintf("Screen is %dx%d.", width, height)
	if matches {
		summary += " Screenshots are shown at full size, so coordinates read off a screenshot can be used directly for clicks."
	} else {
		summary += fmt.Sprintf(
			" WARNING: the screen is larger than %dpx on its long edge, so screenshots are shrunk to %.3f of their size before being viewed."+
				" Coordinates read off a screenshot will NOT match the screen — multiply both x and y by %.4f before clicking,"+
				" or reduce the VM's display resolution to %dpx or less on the long edge.",
			visionLongEdgeLimit, scale, multiplier, visionLongEdgeLimit)
	}
	if activeWindow != "" {
		summary += " Active window: " + activeWindow + "."
	}

	return desktop.OkResult(summary, map[string]interface{}{
		"width":                        width,
		"height":                       height,
		"active_window":                activeWindow,
		"coordinates_match_screenshot": matches,
		"coordinate_multiplier":        strconv.FormatFloat(multiplier, 'f', -1, 64),
	}), nil
}

// ParseScreenInfo reads the two-line output of desktop.ScreenInfoCmd: a
// "<width> <height>" line, then an optional active-window title. The title may
// be absent (a bare desktop with nothing focused), which is not an error.
func ParseScreenInfo(stdout string) (width, height int64, activeWindow string, err error) {
	lines := strings.Split(strings.ReplaceAll(stdout, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return 0, 0, "", fmt.Errorf("could not read screen geometry — is a desktop session running on this display?")
	}

	fields := strings.Fields(strings.TrimSpace(lines[0]))
	if len(fields) < 2 {
		return 0, 0, "", fmt.Errorf("unexpected screen geometry %q", strings.TrimSpace(lines[0]))
	}
	width, err = strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("unexpected screen width %q", fields[0])
	}
	height, err = strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("unexpected screen height %q", fields[1])
	}

	if len(lines) > 1 {
		activeWindow = strings.TrimSpace(strings.Join(lines[1:], " "))
	}
	return width, height, activeWindow, nil
}

// ScreenshotScale returns the factor a screenshot of this screen is shrunk by
// before a vision model sees it — 1 when the screen is small enough to be shown
// untouched. Multiplying a screenshot coordinate by the reciprocal gives the
// screen coordinate.
func ScreenshotScale(width, height int64) float64 {
	long := width
	if height > long {
		long = height
	}
	if long <= visionLongEdgeLimit {
		return 1
	}
	return float64(visionLongEdgeLimit) / float64(long)
}
