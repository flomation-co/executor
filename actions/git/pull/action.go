package git_pull

import (
	core "flomation.app/automate/executor"
	git_common "flomation.app/automate/executor/actions/git"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Git Pull"
	Description  = "Pull latest changes from a remote"
	Website      = "https://www.flomation.co"
	Icon         = "code-branch"
	Date         = "06/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	git_common.AuthInputs[0], // auth_method
	git_common.AuthInputs[1], // ssh_key
	git_common.AuthInputs[2], // username
	git_common.AuthInputs[3], // password
	{
		Name:        "remote_name",
		Type:        core.ConnectionTypeString,
		Label:       "Remote Name",
		Placeholder: "origin",
	},
	{
		Name:        "branch",
		Type:        core.ConnectionTypeString,
		Label:       "Branch",
		Placeholder: "",
	},
}

var Outputs = [...]core.Connection{
	{
		Name:  "success",
		Type:  core.ConnectionTypeBoolean,
		Label: "Success",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	repoPath := ""
	if rp := core.FindConnection("repository_path", inputs); rp != nil && rp.String() != nil {
		repoPath = *rp.String()
	}
	remoteName := core.FindConnection("remote_name", inputs)
	branch := core.FindConnection("branch", inputs)

	auth, err := git_common.GetAuthFromInputs(inputs)
	if err != nil {
		return nil, err
	}

	_, w, err := git_common.GetRepositoryAndWorktree(repoPath)
	if err != nil {
		return nil, err
	}

	pullOpts := &git.PullOptions{
		Auth: auth,
	}

	if remoteName != nil && remoteName.String() != nil && *remoteName.String() != "" {
		pullOpts.RemoteName = *remoteName.String()
	}

	if branch != nil && branch.String() != nil && *branch.String() != "" {
		pullOpts.ReferenceName = plumbing.NewBranchReferenceName(*branch.String())
	}

	err = w.Pull(pullOpts)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return nil, err
	}

	return map[string]interface{}{"success": true}, nil
}
