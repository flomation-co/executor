package git_clone

import (
	"io/fs"
	"os"
	"path/filepath"

	core "flomation.app/automate/executor"
	git_common "flomation.app/automate/executor/actions/git"
	"github.com/go-git/go-git/v6"
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
		Type:     core.ConnectionTypeString,
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{
		Name:  "repository_path",
		Type:  core.ConnectionTypeString,
		Label: "Repository Path",
	},
	{
		Name:  "branch",
		Type:  core.ConnectionTypeString,
		Label: "Branch",
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	repository := core.FindConnection("repository_url", inputs)

	auth, err := git_common.GetAuthFromInputs(inputs)
	if err != nil {
		return nil, err
	}

	// Clone into a temp directory then move contents into the execution directory
	tmpDir, err := os.MkdirTemp("", "flomation-clone-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	cloneOpts := &git.CloneOptions{
		URL:  *repository.String(),
		Auth: auth,
	}

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
		"repository_path": cwd,
		"branch":          branch,
	}, nil
}
