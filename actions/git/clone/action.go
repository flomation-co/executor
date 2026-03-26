package git_clone

import (
	"io/fs"
	"os"
	"path/filepath"

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
		Name: "branch",
		Type: core.ConnectionTypeString,
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	repository := core.FindConnection("repository_url", inputs)
	sshKey := core.FindConnection("ssh_key", inputs)

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

	// Clone into a temp directory then move contents into the execution directory
	tmpDir, err := os.MkdirTemp("", "flomation-clone-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	repo, err := git.PlainClone(tmpDir, cloneOpts)
	if err != nil {
		return nil, err
	}

	branch := ""
	head, err := repo.Head()
	if err == nil && head != nil {
		branch = head.Name().Short()
	}

	// Move all cloned files into the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	err = filepath.WalkDir(tmpDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(tmpDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		dest := filepath.Join(cwd, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		return os.Rename(path, dest)
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"branch": branch,
	}, nil
}
