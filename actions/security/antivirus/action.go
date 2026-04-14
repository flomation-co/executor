package antivirus

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Scan with Antivirus"
	Description  = "Scan a file or URL for malware using ClamAV"
	Website      = "https://www.flomation.co"
	Icon         = "shield-virus"
	Date         = "20/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "scan_path",
		Type:        core.ConnectionTypeString,
		Label:       "Scan Path",
		Placeholder: "",
		Required:    true,
	},
	{
		Name:        "recursive",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Recursive Scan",
		Placeholder: "",
	},
}

var Outputs = [...]core.Connection{
	{
		Name: "is_clean",
		Type: core.ConnectionTypeBoolean,
	},
	{
		Name: "infected_count",
		Type: core.ConnectionTypeInteger,
	},
	{
		Name: "scanned_count",
		Type: core.ConnectionTypeInteger,
	},
	{
		Name: "scan_output",
		Type: core.ConnectionTypeString,
	},
}

var (
	infectedRegexp = regexp.MustCompile(`Infected files:\s*(\d+)`)
	scannedRegexp  = regexp.MustCompile(`Scanned files:\s*(\d+)`)
)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	scanPathConn := core.FindConnection("scan_path", inputs)
	if scanPathConn == nil || scanPathConn.String() == nil {
		return nil, fmt.Errorf("scan_path is required")
	}
	scanPath := *scanPathConn.String()

	if _, err := os.Stat(scanPath); err != nil {
		return nil, fmt.Errorf("scan path does not exist: %s", scanPath)
	}

	args := []string{"--no-summary=no"}

	recursiveConn := core.FindConnection("recursive", inputs)
	if recursiveConn != nil && recursiveConn.Boolean() != nil && *recursiveConn.Boolean() {
		args = append(args, "--recursive")
	}

	args = append(args, scanPath)

	cmd := exec.Command("clamscan", args...)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			if exitCode >= 2 {
				return nil, fmt.Errorf("clamscan error (exit code %d): %s", exitCode, outputStr)
			}
			// Exit code 1 means virus found — this is a valid result
		} else {
			return nil, fmt.Errorf("failed to execute clamscan: %w", err)
		}
	}

	infectedCount := parseSummaryValue(outputStr, infectedRegexp)
	scannedCount := parseSummaryValue(outputStr, scannedRegexp)
	isClean := infectedCount == 0

	return map[string]interface{}{
		"is_clean":       isClean,
		"infected_count": infectedCount,
		"scanned_count":  scannedCount,
		"scan_output":    strings.TrimSpace(outputStr),
	}, nil
}

func parseSummaryValue(output string, re *regexp.Regexp) int {
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return 0
	}

	val, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}

	return val
}
