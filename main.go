package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v45/github"
)

var ghToken string
var cmd = NewCommand()

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Println("GITHUB_TOKEN is not set")
		os.Exit(1)
	}
	ghToken = token
	if err := http.ListenAndServe(fmt.Sprintf(":%d", *cmd.Port), NewWebhookServer(pushEventHandler)); err != nil {
		fmt.Println("finished server:", err)
	}
}

var mu sync.Mutex

func pushEventHandler(ctx context.Context, event *github.PushEvent) error {
	mu.Lock()
	defer mu.Unlock()

	if event.Ref == nil {
		return errors.New("ref is nil")
	}

	if *event.Ref != *cmd.Ref {
		return errors.New("unsupported ref: " + *event.Ref)
	}

	// Git clone && pull
	auth := &githttp.BasicAuth{
		Username: "user", // dummy
		Password: ghToken,
	}
	git := NewGit(*cmd.Dir, *cmd.Repository, *cmd.Remote, *cmd.Ref, auth)
	err := git.Pull(ctx)
	if err != nil {
		return fmt.Errorf("failed to pull repository: %w", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	if err := os.Chdir(git.ReposPath()); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}
	defer os.Chdir(wd)

	// Docker build
	docker, err := NewDocker(ctx, *cmd.Context, *cmd.Dockerfile, *cmd.ImageName, *cmd.Tag, ghToken)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	if err := docker.Build(ctx); err != nil {
		return fmt.Errorf("failed to build docker image: %w", err)
	}

	return nil
}
