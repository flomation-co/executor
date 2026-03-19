package git_branch

import (
	core "flomation.app/automate/executor"
	git_common "flomation.app/automate/executor/actions/git"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Git Branch"
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
	{
		Name:        "branch_name",
		Type:        core.ConnectionTypeString,
		Label:       "Branch Name",
		Placeholder: "",
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
	branchName := core.FindConnection("branch_name", inputs)

	_, w, err := git_common.GetRepositoryAndWorktree(*repository.String())
	if err != nil {
		return nil, err
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(*branchName.String()),
		Create: true,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"repository_path": *repository.String(),
	}, nil
}
