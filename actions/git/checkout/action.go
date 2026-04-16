package git_checkout

import (
	core "flomation.app/automate/executor"
	git_common "flomation.app/automate/executor/actions/git"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Git Checkout"
	Description  = "Check out a branch or commit in a Git repository"
	Website      = "https://www.flomation.co"
	Icon         = "code-branch"
	Date         = "06/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	core.Connection{
		Name:        "repository_path",
		Type:        core.ConnectionTypeString,
		Label:       "Repository Path",
		Placeholder: "",
		Required:    true,
	},
	core.Connection{
		Name:        "branch",
		Type:        core.ConnectionTypeString,
		Label:       "Branch",
		Placeholder: "main",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	core.Connection{
		Name: "repository_path",
		Type: core.ConnectionTypeString,
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	repoPath := ""
	if rp := core.FindConnection("repository_path", inputs); rp != nil && rp.String() != nil {
		repoPath = *rp.String()
	}
	branch := core.FindConnection("branch", inputs)

	w, err := git_common.GetWorktree(repoPath)
	if err != nil {
		return nil, err
	}

	if err := w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(*branch.String()),
	}); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"repository_path": repoPath,
	}, nil
}
