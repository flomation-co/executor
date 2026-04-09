package git_common

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/transport"
	githttp "github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
)

// AuthInputs returns the standard authentication input connections
// used by clone, pull, and push actions.
var AuthInputs = [...]core.Connection{
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
		Name:        "ssh_key",
		Type:        core.ConnectionTypeText,
		Label:       "SSH Private Key",
		Placeholder: "Paste your private key",
		Required:    true,
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"ssh"}},
	},
	{
		Name:        "username",
		Type:        core.ConnectionTypeString,
		Label:       "Username",
		Placeholder: "",
		Required:    true,
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"http"}},
	},
	{
		Name:        "password",
		Type:        core.ConnectionTypeString,
		Label:       "Password / Token",
		Placeholder: "",
		Required:    true,
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"http", "token"}},
	},
}

// GetAuthFromInputs resolves the authentication method from the standard
// auth inputs. Returns nil for anonymous access.
func GetAuthFromInputs(inputs []*core.Connection) (transport.AuthMethod, error) {
	method := "anonymous"
	if mc := core.FindConnection("auth_method", inputs); mc != nil && mc.String() != nil && *mc.String() != "" {
		method = strings.TrimSpace(*mc.String())
	}

	switch method {
	case "anonymous":
		return nil, nil

	case "ssh":
		sshKey := core.FindConnection("ssh_key", inputs)
		if sshKey == nil || sshKey.String() == nil || *sshKey.String() == "" {
			return nil, fmt.Errorf("SSH private key is required for SSH authentication")
		}
		return ssh.NewPublicKeys("git", []byte(*sshKey.String()), "")

	case "http":
		user := core.FindConnection("username", inputs)
		pass := core.FindConnection("password", inputs)
		if user == nil || user.String() == nil || pass == nil || pass.String() == nil {
			return nil, fmt.Errorf("username and password are required for HTTP authentication")
		}
		return &githttp.BasicAuth{
			Username: *user.String(),
			Password: *pass.String(),
		}, nil

	case "token":
		token := core.FindConnection("password", inputs)
		if token == nil || token.String() == nil || *token.String() == "" {
			return nil, fmt.Errorf("token is required for token authentication")
		}
		// Use oauth2 as username — works with GitLab, GitHub, and Bitbucket tokens
		return &githttp.BasicAuth{
			Username: "oauth2",
			Password: *token.String(),
		}, nil

	default:
		return nil, fmt.Errorf("unknown authentication method: %s", method)
	}
}

func GetRepository(repositoryPath string) (*git.Repository, error) {
	if repositoryPath == "" {
		repositoryPath = "."
	}
	return git.PlainOpen(repositoryPath)
}

func GetRepositoryAndWorktree(repositoryPath string) (*git.Repository, *git.Worktree, error) {
	if repositoryPath == "" {
		repositoryPath = "."
	}
	r, err := git.PlainOpen(repositoryPath)
	if err != nil {
		return nil, nil, err
	}

	w, err := r.Worktree()
	if err != nil {
		return nil, nil, err
	}

	return r, w, nil
}

func GetWorktree(repositoryPath string) (*git.Worktree, error) {
	if repositoryPath == "" {
		repositoryPath = "."
	}
	r, err := git.PlainOpen(repositoryPath)
	if err != nil {
		return nil, err
	}

	w, err := r.Worktree()
	if err != nil {
		return nil, err
	}

	return w, nil
}

func GetSSHAuth(sshKey string) (transport.AuthMethod, error) {
	return ssh.NewPublicKeys("git", []byte(sshKey), "")
}
