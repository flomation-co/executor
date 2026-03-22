package git_poll

import (
	core "flomation.app/automate/executor"

	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Git Poll Trigger"
	Description  = "Triggers a flow when changes are detected in a Git repository"
	Website      = "https://www.flomation.co"
	Icon         = "code-branch"
	Date         = "22/03/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{
		Name:        "repository_url",
		Type:        core.ConnectionTypeString,
		Label:       "Repository URL",
		Placeholder: "git@github.com:org/repo.git",
		Required:    true,
	},
	{
		Name:        "ssh_key",
		Type:        core.ConnectionTypeText,
		Label:       "SSH Private Key",
		Placeholder: "Optional SSH key for authentication",
	},
	{
		Name:        "branch_regex",
		Type:        core.ConnectionTypeString,
		Label:       "Branch Regex",
		Placeholder: "e.g. ^main$ or ^feature/.*",
	},
	{
		Name:        "poll_interval",
		Type:        core.ConnectionTypeString,
		Label:       "Poll Interval",
		Placeholder: "e.g. 60s, 5m",
	},
}

var Outputs = [...]core.Connection{
	{
		Name:  "branch",
		Type:  core.ConnectionTypeString,
		Label: "Branch",
	},
	{
		Name:  "commit_hash",
		Type:  core.ConnectionTypeString,
		Label: "Commit Hash",
	},
	{
		Name:  "commit_message",
		Type:  core.ConnectionTypeString,
		Label: "Commit Message",
	},
	{
		Name:  "repository_url",
		Type:  core.ConnectionTypeString,
		Label: "Repository URL",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	log.Debug("Executing git poll trigger")

	result := make(map[string]interface{})

	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}

	return result, nil
}
