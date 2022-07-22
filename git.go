package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

type Git struct {
	dir        string
	repository string
	remote     string
	ref        plumbing.ReferenceName
	auth       transport.AuthMethod

	r *git.Repository
}

func NewGit(dir, repository, remote, ref string, auth transport.AuthMethod) *Git {
	return &Git{
		dir:        dir,
		repository: repository,
		remote:     remote,
		ref:        plumbing.ReferenceName(ref),
		auth:       auth,
	}
}

func (g *Git) ReposPath() string {
	dirName := filepath.Base(g.repository)
	if strings.HasPrefix(dirName, "git@") {
		dirName = strings.TrimSuffix(dirName, ".git")
	}
	return g.dir + string(filepath.Separator) + dirName
}

func (g *Git) Clone(ctx context.Context) error {
	var err error
	g.r, err = git.PlainCloneContext(ctx, g.ReposPath(), false, &git.CloneOptions{
		URL:           g.repository,
		RemoteName:    g.remote,
		ReferenceName: g.ref,
		Auth:          g.auth,
		Progress:      os.Stdout,
	})
	if err == nil {
		return nil
	}
	if err != git.ErrRepositoryAlreadyExists {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	g.r, err = git.PlainOpen(g.ReposPath())
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}
	return nil
}

func (g *Git) Pull(ctx context.Context) error {
	if g.r == nil {
		if err := g.Clone(ctx); err != nil {
			return fmt.Errorf("failed to pull repository: %w", err)
		}
	}

	if g.r == nil {
		fmt.Println(g)
		return nil
	}
	w, err := g.r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	err = w.PullContext(ctx, &git.PullOptions{
		RemoteName:    g.remote,
		ReferenceName: g.ref,
		Auth:          g.auth,
		Progress:      os.Stdout,
		Force:         true,
	})
	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			fmt.Println("already up to date")
			return nil
		}
		return fmt.Errorf("failed to pull repository: %w", err)
	}
	return nil
}
