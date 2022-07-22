package main

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type Docker struct {
	contextPath    string
	dockerfilePath string
	imageName      string
	tag            string
	ghToken        string
	cli            *client.Client
}

func NewDocker(ctx context.Context, contextPath, dockerfilePath, imageName, tag, ghToken string) (*Docker, error) {
	dc, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	dc.NegotiateAPIVersion(ctx)
	return &Docker{
		contextPath:    contextPath,
		dockerfilePath: dockerfilePath,
		imageName:      imageName,
		tag:            tag,
		ghToken:        ghToken,
		cli:            dc,
	}, nil
}

func (d *Docker) Build(ctx context.Context) error {
	dc, err := d.Context()
	if err != nil {
		return fmt.Errorf("failed to create docker context: %w", err)
	}
	res, err := d.cli.ImageBuild(ctx, dc, types.ImageBuildOptions{
		Tags:       []string{fmt.Sprintf("%s:%s", d.imageName, d.tag)},
		Remove:     true,
		Dockerfile: d.dockerfilePath,
		BuildArgs:  map[string]*string{"GITHUB_TOKEN": &d.ghToken},
	})
	if err != nil {
		return fmt.Errorf("failed to build image: %w", err)
	}
	defer res.Body.Close()

	if _, err := io.Copy(os.Stdout, res.Body); err != nil {
		return fmt.Errorf("failed to print log: %w", err)
	}
	return nil
}

func (d *Docker) Save(ctx context.Context) (io.ReadCloser, error) {
	res, err := d.cli.ImageSave(ctx, []string{fmt.Sprintf("%s:%s", d.imageName, d.tag)})
	if err != nil {
		return nil, fmt.Errorf("failed to save image: %w", err)
	}
	return res, nil
}

func (d *Docker) Context() (*bytes.Reader, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	os.Chdir(d.contextPath)

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk: %w", err)
		}
		if info.IsDir() {
			return nil
		}
		if err := tw.WriteHeader(&tar.Header{
			Name:    path,
			Size:    info.Size(),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
		}); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer f.Close()

		b, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		if _, err := tw.Write(b); err != nil {
			return fmt.Errorf("failed to write tar body: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}
	return bytes.NewReader(buf.Bytes()), nil
}

func (d *Docker) ImageTag() string {
	return fmt.Sprintf("%s:%s", d.imageName, d.tag)
}
