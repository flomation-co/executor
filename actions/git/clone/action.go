package git_clone

import (
	"os"

	core "flomation.app/automate/executor"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Git Clone"
	Description  = "Clone a Git repository"
	Website      = "https://www.flomation.co"
	Icon         = "code-branch"
	Date         = "06/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "repository_url",
		Type:        core.ConnectionTypeString,
		Label:       "Repository URL",
		Placeholder: "",
		Required:    true,
	},
	{
		Name:        "ssh_key",
		Type:        core.ConnectionTypeText,
		Label:       "SSH Private Key",
		Placeholder: "",
	},
}

var Outputs = [...]core.Connection{
	{
		Name: "repository_path",
		Type: core.ConnectionTypeString,
	},
	{
		Name: "branch",
		Type: core.ConnectionTypeString,
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	repository := core.FindConnection("repository_url", inputs)
	sshKey := core.FindConnection("ssh_key", inputs)

	f, err := os.MkdirTemp(".", "")
	if err != nil {
		return nil, err
	}

	cloneOpts := &git.CloneOptions{
		URL: *repository.String(),
	}

	if sshKey != nil && sshKey.String() != nil && *sshKey.String() != "" {
		pk, err := ssh.NewPublicKeys("git", []byte(*sshKey.String()), "")
		if err != nil {
			return nil, err
		}
		cloneOpts.Auth = pk
	}

	repo, err := git.PlainClone(f, cloneOpts)
	if err != nil {
		return nil, err
	}

	branch := ""
	head, err := repo.Head()
	if err == nil && head != nil {
		branch = head.Name().Short()
	}

	return map[string]interface{}{
		"repository_path": f,
		"branch":          branch,
	}, nil
}
