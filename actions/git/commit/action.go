package git_commit

import (
	"time"

	core "flomation.app/automate/executor"
	git_common "flomation.app/automate/executor/actions/git"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Git Commit"
	Description  = "Create a Git commit with staged changes"
	Website      = "https://www.flomation.co"
	Icon         = "code-branch+check"
	Date         = "06/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "message",
		Type:        core.ConnectionTypeText,
		Label:       "Commit Message",
		Placeholder: "",
		Required:    true,
	},
	{
		Name:        "author_name",
		Type:        core.ConnectionTypeString,
		Label:       "Author Name",
		Placeholder: "",
		Required:    true,
	},
	{
		Name:        "author_email",
		Type:        core.ConnectionTypeString,
		Label:       "Author Email",
		Placeholder: "",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{
		Name: "commit_hash",
		Type: core.ConnectionTypeString,
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	repoPath := ""
	if repository := core.FindConnection("repository_path", inputs); repository != nil && repository.String() != nil {
		repoPath = *repository.String()
	}
	message := core.FindConnection("message", inputs)
	authorName := core.FindConnection("author_name", inputs)
	authorEmail := core.FindConnection("author_email", inputs)

	_, w, err := git_common.GetRepositoryAndWorktree(repoPath)
	if err != nil {
		return nil, err
	}

	hash, err := w.Commit(*message.String(), &git.CommitOptions{
		Author: &object.Signature{
			Name:  *authorName.String(),
			Email: *authorEmail.String(),
			When:  time.Now(),
		},
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"commit_hash": hash.String(),
	}, nil
}
