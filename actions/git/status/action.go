package git_status

import (
	core "flomation.app/automate/executor"
	git_common "flomation.app/automate/executor/actions/git"
	"fmt"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Git Status"
	Description  = "Git Actions"
	Website      = "https://www.flomation.co"
	Icon         = "code-branch"
	Date         = "19/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "repository_path",
		Type:        core.ConnectionTypeString,
		Label:       "Repository Path",
		Placeholder: "",
	},
}

var Outputs = [...]core.Connection{
	{
		Name: "repository_path",
		Type: core.ConnectionTypeString,
	},
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
	repository := core.FindConnection("repository_path", inputs)

	w, err := git_common.GetWorktree(*repository.String())
	if err != nil {
		return nil, err
	}

	s, err := w.Status()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"repository_path": *repository.String(),
		"is_clean":        fmt.Sprintf("%t", s.IsClean()),
		"status":          s.String(),
	}, nil
}
