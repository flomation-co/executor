package tofu

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
)

// ChangeSummary holds the add/change/destroy counts OpenTofu reports.
type ChangeSummary struct {
	Add     int
	Change  int
	Destroy int
}

// ParsePlanSummary scans `tofu plan -json` newline-delimited output for the
// change_summary message and returns the counts. found is false when no summary
// line was present (e.g. an error before planning completed). This describes
// planned intent — for what an apply/destroy actually did, use ParseApplyOutcome.
func ParsePlanSummary(jsonStream string) (summary ChangeSummary, found bool) {
	sc := bufio.NewScanner(strings.NewReader(jsonStream))
	// Plan output can contain long single lines (full resource diffs); raise the
	// scanner ceiling well above the default 64 KiB.
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	for sc.Scan() {
		var line struct {
			Type    string `json:"type"`
			Changes struct {
				Add    int `json:"add"`
				Change int `json:"change"`
				Remove int `json:"remove"`
			} `json:"changes"`
		}
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			continue // non-JSON or unrelated line
		}
		if line.Type == "change_summary" {
			summary = ChangeSummary{
				Add:     line.Changes.Add,
				Change:  line.Changes.Change,
				Destroy: line.Changes.Remove,
			}
			found = true
		}
	}
	return summary, found
}

func (s ChangeSummary) HasChanges() bool { return s.Add+s.Change+s.Destroy > 0 }

func (s ChangeSummary) String() string {
	return fmt.Sprintf("+%d ~%d -%d", s.Add, s.Change, s.Destroy)
}

// ApplyOutcome counts the resources actually acted on during an apply/destroy,
// derived from the per-resource apply_complete hooks in `-json` output. Unlike
// the plan-phase change_summary, this reflects what really happened, so it is
// the correct source for a destroy's resource count.
type ApplyOutcome struct {
	Added     int
	Changed   int
	Destroyed int
}

// ParseApplyOutcome counts apply_complete hooks by action from `tofu apply -json`
// or `tofu destroy -json` output.
func ParseApplyOutcome(jsonStream string) ApplyOutcome {
	var out ApplyOutcome
	sc := bufio.NewScanner(strings.NewReader(jsonStream))
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	for sc.Scan() {
		var line struct {
			Type string `json:"type"`
			Hook struct {
				Action string `json:"action"`
			} `json:"hook"`
		}
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			continue
		}
		if line.Type != "apply_complete" {
			continue
		}
		switch line.Hook.Action {
		case "create":
			out.Added++
		case "update":
			out.Changed++
		case "delete":
			out.Destroyed++
		}
	}
	return out
}
