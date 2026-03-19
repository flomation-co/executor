package git_common

import (
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
)

func GetRepository(repositoryPath string) (*git.Repository, error) {
	return git.PlainOpen(repositoryPath)
}

func GetRepositoryAndWorktree(repositoryPath string) (*git.Repository, *git.Worktree, error) {
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
