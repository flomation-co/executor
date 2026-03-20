package git_clone

import (
	core "flomation.app/automate/executor"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
	"os"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Git Clone"
	Description  = "Git Actions"
	Website      = "https://www.flomation.co"
	Icon         = "code-branch"
	Date         = "06/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	core.Connection{
		Name:        "repository_url",
		Type:        core.ConnectionTypeString,
		Label:       "Repository URL",
		Placeholder: "",
		Required:    true,
	},
	core.Connection{
		Name:        "ssh_key",
		Type:        core.ConnectionTypeText,
		Label:       "SSH Private Key",
		Placeholder: "",
	},
}

var Outputs = [...]core.Connection{
	core.Connection{
		Name: "repository_path",
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

	pk, err := ssh.NewPublicKeys("git", []byte(*sshKey.String()), "")
	if err != nil {
		return nil, err
	}

	_, err = git.PlainClone(f, &git.CloneOptions{
		URL:      *repository.String(),
		Auth:     pk,
		Progress: os.Stdout,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success":         true,
		"repository_path": f,
	}, nil
}
