package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"strconv"
	"strings"
	"time"
)

type SSH struct {
	host string
	port int
	user string
	auth []ssh.AuthMethod
}

func NewSSH(host string, port int, user string, auth []ssh.AuthMethod) *SSH {
	return &SSH{
		host: host,
		port: port,
		user: user,
		auth: auth,
	}
}

// NewSSHFromString str: user@host:port
func NewSSHFromString(str string, auth []ssh.AuthMethod) (*SSH, error) {
	var host, user string
	var port int
	uh := strings.Split(str, "@")
	if len(uh) == 2 {
		user = uh[0]
		hp := strings.Split(uh[1], ":")
		if len(hp) == 2 {
			host = hp[0]
			var err error
			port, err = strconv.Atoi(hp[1])
			if err != nil {
				return nil, fmt.Errorf("failed to parse port: %w", err)
			}
		} else {
			host = hp[0]
			port = 22
		}
	} else {
		return nil, fmt.Errorf("invalid ssh string: %s", str)
	}
	return &SSH{
		host: host,
		port: port,
		user: user,
		auth: auth,
	}, nil
}

func (s *SSH) String() string {
	return fmt.Sprintf("%s@%s:%d", s.user, s.host, s.port)
}

func (s *SSH) Run(cmd string) (string, string, error) {
	fmt.Printf("[%s] run command: %s\n", s, cmd)
	client, err := s.newClient()
	if err != nil {
		return "", "", fmt.Errorf("failed to dial ssh: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("failed to create ssh session: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := session.Run(cmd); err != nil {
		return "", "", fmt.Errorf("failed to run command (%s) on ssh: %w", cmd, err)
	}

	o, err := io.ReadAll(stdout)
	if err != nil {
		return "", "", fmt.Errorf("failed to read stdout: %w", err)
	}
	e, err := io.ReadAll(stderr)
	if err != nil {
		return string(o), "", fmt.Errorf("failed to read stderr: %w", err)
	}

	return string(o), string(e), nil
}

func (s *SSH) RunWithPipe(cmd string, reader io.ReadCloser) (string, string, error) {
	fmt.Printf("[%s] run command: %s\n", s, cmd)
	defer reader.Close()

	client, err := s.newClient()
	if err != nil {
		return "", "", fmt.Errorf("failed to dial ssh: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("failed to create ssh session: %w", err)
	}
	defer session.Close()

	writer, err := session.StdinPipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	defer writer.Close()
	stdout, err := session.StdoutPipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := session.Start(cmd); err != nil {
		return "", "", fmt.Errorf("failed to run command (%s) on ssh: %w", cmd, err)
	}

	if _, err := io.Copy(writer, reader); err != nil {
		return "", "", fmt.Errorf("failed to write to stdin: %w", err)
	}
	session.Close()

	o, err := io.ReadAll(stdout)
	if err != nil {
		return "", "", fmt.Errorf("failed to read stdout: %w", err)
	}
	e, err := io.ReadAll(stderr)
	if err != nil {
		return string(o), "", fmt.Errorf("failed to read stderr: %w", err)
	}
	return string(o), string(e), nil
}

func (s *SSH) newClient() (*ssh.Client, error) {
	return ssh.Dial("tcp", fmt.Sprintf("%s:%d", s.host, s.port), &ssh.ClientConfig{
		User:            s.user,
		Auth:            s.auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         1 * time.Minute,
	})
}
