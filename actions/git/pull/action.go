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
	Icon         = "code-branch+arrow-down"
	Date         = "06/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:  "auth_method",
		Type:  core.ConnectionTypeString,
		Label: "Authentication",
		Options: []core.ConnectionOption{
			{Name: "Anonymous", Value: "anonymous"},
			{Name: "SSH Key", Value: "ssh"},
			{Name: "HTTP (Username/Password)", Value: "http"},
			{Name: "Token", Value: "token"},
		},
	},
	{
		Name:     "ssh_key",
		Type:     core.ConnectionTypeText,
		Label:    "SSH Private Key",
		Required: true,
		Visible:  &core.VisibleWhen{Field: "auth_method", Values: []string{"ssh"}},
	},
	{
		Name:     "username",
		Type:        core.ConnectionTypeSecret,
		Label:    "Username",
		Required: true,
		Visible:  &core.VisibleWhen{Field: "auth_method", Values: []string{"http"}},
	},
	{
		Name:     "password",
		Type:     core.ConnectionTypeString,
		Label:    "Password / Token",
		Required: true,
		Visible:  &core.VisibleWhen{Field: "auth_method", Values: []string{"http", "token"}},
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
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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
