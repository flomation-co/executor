package git_push

import (
	core "flomation.app/automate/executor"
	git_common "flomation.app/automate/executor/actions/git"
	"github.com/go-git/go-git/v6"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Git Push"
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
	},
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
}

var Outputs = [...]core.Connection{
	{
		Name: "repository_path",
		Type: core.ConnectionTypeString,
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	repository := core.FindConnection("repository_path", inputs)
	sshKey := core.FindConnection("ssh_key", inputs)
	remoteName := core.FindConnection("remote_name", inputs)

	r, err := git_common.GetRepository(*repository.String())
	if err != nil {
		return nil, err
	}

	auth, err := git_common.GetSSHAuth(*sshKey.String())
	if err != nil {
		return nil, err
	}

	pushOpts := &git.PushOptions{
		Auth: auth,
	}

	if remoteName != nil && remoteName.String() != nil && *remoteName.String() != "" {
		pushOpts.RemoteName = *remoteName.String()
	}

	err = r.Push(pushOpts)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return nil, err
	}

	return map[string]interface{}{
		"repository_path": *repository.String(),
	}, nil
}
