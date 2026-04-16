package git_tag

import (
	core "flomation.app/automate/executor"
	git_common "flomation.app/automate/executor/actions/git"
	"github.com/go-git/go-git/v6"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Git Tag"
	Description  = "Create or list tags in a Git repository"
	Website      = "https://www.flomation.co"
	Icon         = "code-branch"
	Date         = "19/03/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "tag_name",
		Type:        core.ConnectionTypeString,
		Label:       "Tag Name",
		Placeholder: "",
	},
	{
		Name:        "message",
		Type:        core.ConnectionTypeText,
		Label:       "Tag Message",
		Placeholder: "",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{
		Name: "tag_name",
		Type: core.ConnectionTypeString,
	},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	repoPath := ""
	if repository := core.FindConnection("repository_path", inputs); repository != nil && repository.String() != nil {
		repoPath = *repository.String()
	}
	tagName := core.FindConnection("tag_name", inputs)
	message := core.FindConnection("message", inputs)

	r, err := git_common.GetRepository(repoPath)
	if err != nil {
		return nil, err
	}

	head, err := r.Head()
	if err != nil {
		return nil, err
	}

	var tagOpts *git.CreateTagOptions
	if message != nil && message.String() != nil && *message.String() != "" {
		tagOpts = &git.CreateTagOptions{
			Message: *message.String(),
		}
	}

	_, err = r.CreateTag(*tagName.String(), head.Hash(), tagOpts)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tag_name": *tagName.String(),
	}, nil
}
