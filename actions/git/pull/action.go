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
	Description  = "Git Actions"
	Website      = "https://www.flomation.co"
	Icon         = "code-branch"
	Date         = "06/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "ssh_key",
		Type:        core.ConnectionTypeText,
		Label:       "SSH Private Key",
		Placeholder: "",
	},
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
		Name: "success",
		Type: core.ConnectionTypeBoolean,
		Label: "Success",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	repoPath := ""
	if repository := core.FindConnection("repository_path", inputs); repository != nil && repository.String() != nil {
		repoPath = *repository.String()
	}
	sshKey := core.FindConnection("ssh_key", inputs)
	remoteName := core.FindConnection("remote_name", inputs)
	branch := core.FindConnection("branch", inputs)

	_, w, err := git_common.GetRepositoryAndWorktree(repoPath)
	if err != nil {
		return nil, err
	}

	auth, err := git_common.GetSSHAuth(*sshKey.String())
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
