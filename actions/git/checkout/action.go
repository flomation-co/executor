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
	Description  = "Git Actions"
	Website      = "https://www.flomation.co"
	Icon         = "code-branch"
	Date         = "06/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "branch",
		Type:        core.ConnectionTypeString,
		Label:       "Branch",
		Placeholder: "main",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	repoPath := ""
	if repository := core.FindConnection("repository_path", inputs); repository != nil && repository.String() != nil {
		repoPath = *repository.String()
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

	return map[string]interface{}{}, nil
}
