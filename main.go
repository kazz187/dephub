package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/google/go-github/v45/github"
	sshagent "github.com/xanzy/ssh-agent"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
)

var cmd = NewCommand()
var slackCli = NewSlack(*cmd.ChanID)

func main() {
	if *cmd.Run {
		err := runCD(context.Background())
		if err != nil {
			if err := slackCli.Post("デプロイ失敗: " + err.Error()); err != nil {
				fmt.Println("failed to post to slackCli:", err)
			}
		} else {
			if err := slackCli.Post("デプロイ完了"); err != nil {
				fmt.Println("failed to post to slackCli:", err)
			}
		}
	}
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

	err := runCD(ctx)
	if err != nil {
		if err := slackCli.Post("デプロイ失敗: " + err.Error()); err != nil {
			fmt.Println("failed to post to slackCli:", err)
		}
		return errors.New("failed to run cd")
	} else {
		if err := slackCli.Post("デプロイ完了"); err != nil {
			fmt.Println("failed to post to slackCli:", err)
		}
	}
	return nil
}

func runCD(ctx context.Context) error {
	if err := slackCli.Post("ビルド開始"); err != nil {
		fmt.Println("failed to post to slackCli:", err)
	}

	// Git clone && pull
	auth, err := githubAuth()
	if err != nil {
		return fmt.Errorf("failed to create github auth: %w", err)
	}
	git := NewGit(*cmd.Dir, *cmd.Repository, *cmd.Remote, *cmd.Ref, auth)
	if err := git.Pull(ctx); err != nil {
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
	ghToken := os.Getenv("GITHUB_TOKEN")
	docker, err := NewDocker(ctx, *cmd.Context, *cmd.Dockerfile, *cmd.ImageName, *cmd.Tag, ghToken)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	if err := docker.Build(ctx); err != nil {
		return fmt.Errorf("failed to build docker image: %w", err)
	}

	if err := slackCli.Post("ビルド完了"); err != nil {
		fmt.Println("failed to post to slackCli:", err)
	}

	// Deploy to all targets
	eg := errgroup.Group{}
	for _, s := range *cmd.SSH {
		sshStr := s
		eg.Go(func() error {
			err := deployToTarget(ctx, docker, sshStr)
			if err != nil {
				return fmt.Errorf("failed to deploy docker image to %s: %w", sshStr, err)
			}
			return nil
		})
	}
	return eg.Wait()
}

func deployToTarget(ctx context.Context, docker *Docker, target string) error {
	// Docker save
	pipe, err := docker.Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to save docker image: %w", err)
	}

	a, _, err := sshagent.New()
	if err != nil {
		return fmt.Errorf("failed to create ssh agent: %w", err)
	}

	// Docker load over ssh
	s, err := NewSSHFromString(target, []ssh.AuthMethod{ssh.PublicKeysCallback(a.Signers)})
	if err != nil {
		return fmt.Errorf("failed to create ssh client: %w", err)
	}

	c := fmt.Sprintf("sudo docker rmi -f %s", docker.ImageTag())
	stdout, stderr, err := s.Run(c)
	if err != nil {
		return fmt.Errorf("failed to remove old docker image: %w", err)
	}
	printResult(s, c, stdout, stderr)

	c = "sudo docker load"
	stdout, stderr, err = s.RunWithPipe(c, pipe)
	if err != nil {
		return fmt.Errorf("failed to load docker image: %w", err)
	}
	printResult(s, c, stdout, stderr)

	// Run post command
	if cmd.Post == nil {
		return nil
	}
	stdout, stderr, err = s.Run(*cmd.Post)
	if err != nil {
		return fmt.Errorf("failed to run post command: %w", err)
	}
	printResult(s, *cmd.Post, stdout, stderr)
	return nil
}

func printResult(s *SSH, cmd, stdout, stderr string) {
	if stdout != "" {
		fmt.Printf("[%s]: %s: stdout: %s", s, cmd, stdout)
	}
	if stderr != "" {
		fmt.Printf("[%s]: %s: stderr: %s", s, cmd, stderr)
	}
}

func githubAuth() (transport.AuthMethod, error) {
	if strings.HasPrefix(*cmd.Repository, "https://") {
		ghToken := os.Getenv("GITHUB_TOKEN")
		if ghToken != "" {
			return nil, errors.New("GITHUB_TOKEN is not set")
		}
		log.Println("use github token auth")
		return &githttp.BasicAuth{
			Username: "user", // dummy
			Password: ghToken,
		}, nil
	}

	if strings.HasPrefix(*cmd.Repository, "git@") {
		pemPath := os.Getenv("GITHUB_PEM_PATH")
		pemPassphrase := os.Getenv("GITHUB_PEM_PASSPHRASE")
		if pemPath != "" {
			pem, err := os.ReadFile(pemPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read pem file: %w", err)
			}
			pk, err := gitssh.NewPublicKeys("git", pem, pemPassphrase)
			if err != nil {
				return nil, fmt.Errorf("failed to create public key auth: %w", err)
			}
			log.Println("use public key auth")
			return pk, err
		}
		pkc, err := gitssh.NewSSHAgentAuth("git")
		if err != nil {
			return nil, fmt.Errorf("failed to create ssh agent auth: %w", err)
		}
		log.Println("use ssh agent auth")
		return pkc, nil
	}
	return nil, errors.New("invalid repository url")
}
