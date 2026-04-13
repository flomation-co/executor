package git_status

import (
	"fmt"

	core "flomation.app/automate/executor"
	git_common "flomation.app/automate/executor/actions/git"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Git Status"
	Description  = "Show the working tree status of a Git repository"
	Website      = "https://www.flomation.co"
	Icon         = "code-branch"
	Date         = "19/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{}

var Outputs = [...]core.Connection{
	{
		Name: "is_clean",
		Type: core.ConnectionTypeString,
	},
	{
		Name: "status",
		Type: core.ConnectionTypeString,
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	repoPath := ""
	if repository := core.FindConnection("repository_path", inputs); repository != nil && repository.String() != nil {
		repoPath = *repository.String()
	}

	w, err := git_common.GetWorktree(repoPath)
	if err != nil {
		return nil, err
	}

	s, err := w.Status()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"is_clean": fmt.Sprintf("%t", s.IsClean()),
		"status":   s.String(),
	}, nil
}
