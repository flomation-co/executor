package git_add

import (
	core "flomation.app/automate/executor"
	git_common "flomation.app/automate/executor/actions/git"
	"github.com/go-git/go-git/v6"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Git Add"
	Description  = "Git Actions"
	Website      = "https://www.flomation.co"
	Icon         = "code-branch"
	Date         = "06/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "repository_path",
		Type:        core.ConnectionTypeString,
		Label:       "Repository Path",
		Placeholder: "",
		Required:    true,
	},
	{
		Name:        "path",
		Type:        core.ConnectionTypeString,
		Label:       "File Path",
		Placeholder: ".",
	},
}

var Outputs = [...]core.Connection{
	{
		Name: "repository_path",
		Type: core.ConnectionTypeString,
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	repository := core.FindConnection("repository_path", inputs)
	path := core.FindConnection("path", inputs)

	w, err := git_common.GetWorktree(*repository.String())
	if err != nil {
		return nil, err
	}

	pathStr := "."
	if path != nil && path.String() != nil && *path.String() != "" {
		pathStr = *path.String()
	}

	if pathStr == "." {
		err = w.AddWithOptions(&git.AddOptions{All: true})
	} else {
		_, err = w.Add(pathStr)
	}
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"repository_path": *repository.String(),
	}, nil
}
