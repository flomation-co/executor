// Package wait suspends the flow execution for a specified duration.
// Unlike sleep (which blocks the runner), wait frees the runner by
// suspending the execution and setting a resume_at timestamp. The API
// automatically resumes the execution when the time arrives.
package wait

import (
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Wait"
	Description  = "Suspend execution for a duration, then auto-resume"
	Website      = "https://www.flomation.co"
	Icon         = "clock+play"
	Date         = "01/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "duration",
		Type:        core.ConnectionTypeString,
		Label:       "Duration",
		Placeholder: "5m, 1h, 2d",
	},
	{
		Name:        "resume_at",
		Type:        core.ConnectionTypeString,
		Label:       "Resume At (ISO 8601)",
		Placeholder: "2026-06-15T14:00:00Z",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "suspended", Type: core.ConnectionTypeBoolean, Label: "Was Suspended"},
	{Name: "resume_at", Type: core.ConnectionTypeString, Label: "Resume At"},
	{Name: "duration", Type: core.ConnectionTypeString, Label: "Duration"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	// On resume, pass through without suspending again
	if flow.IsResumedNode(node.ID) {
		return map[string]interface{}{
			"tool_result": "Resumed after wait",
			"suspended":   false,
			"resume_at":   "",
			"duration":    "",
		}, nil
	}

	durationStr := optStr("duration", inputs)
	resumeAtStr := optStr("resume_at", inputs)

	var resumeAt time.Time

	if resumeAtStr != "" {
		parsed, err := time.Parse(time.RFC3339, resumeAtStr)
		if err != nil {
			return map[string]interface{}{
				"tool_result": fmt.Sprintf("Invalid resume_at format: %v", err),
				"suspended":   false,
				"resume_at":   "",
				"duration":    "",
			}, fmt.Errorf("invalid resume_at: %w", err)
		}
		resumeAt = parsed
	} else if durationStr != "" {
		dur, err := parseDuration(durationStr)
		if err != nil {
			return map[string]interface{}{
				"tool_result": fmt.Sprintf("Invalid duration: %v", err),
				"suspended":   false,
				"resume_at":   "",
				"duration":    "",
			}, fmt.Errorf("invalid duration: %w", err)
		}
		resumeAt = time.Now().UTC().Add(dur)
	} else {
		return map[string]interface{}{
			"tool_result": "Either duration or resume_at is required",
			"suspended":   false,
			"resume_at":   "",
			"duration":    "",
		}, fmt.Errorf("either duration or resume_at is required")
	}

	flow.Suspend(&core.SuspendInfo{
		NodeID:   node.ID,
		Reason:   "wait",
		ResumeAt: &resumeAt,
	})

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Waiting until %s", resumeAt.Format(time.RFC3339)),
		"suspended":   true,
		"resume_at":   resumeAt.Format(time.RFC3339),
		"duration":    durationStr,
	}, core.ErrSuspended
}

// parseDuration handles extended durations beyond Go's time.ParseDuration
// (which only supports up to hours). Supports: 5s, 5m, 1h, 2d, 1w.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// Try Go's built-in parser first (handles s, m, h)
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// Handle days and weeks
	if strings.HasSuffix(s, "d") {
		s = strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(s, "%d", &days); err == nil && days > 0 {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	if strings.HasSuffix(s, "w") {
		s = strings.TrimSuffix(s, "w")
		var weeks int
		if _, err := fmt.Sscanf(s, "%d", &weeks); err == nil && weeks > 0 {
			return time.Duration(weeks) * 7 * 24 * time.Hour, nil
		}
	}

	return 0, fmt.Errorf("unsupported duration format: %s (use s, m, h, d, or w)", s)
}

func optStr(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}
