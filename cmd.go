package main

import (
	"os"

	"gopkg.in/alecthomas/kingpin.v2"
)

type Command struct {
	Dir        *string
	Repository *string
	Remote     *string
	Ref        *string
	Port       *uint
	Context    *string
	Dockerfile *string
	ImageName  *string
	Tag        *string
	Run        *bool
	SSH        *[]string
	Post       *string
	ChanID     *string
}

func NewCommand() *Command {
	app := kingpin.New("dephub", "A tool to deploy a git repository")
	cmd := &Command{
		Dir:        app.Flag("dir", "The directory to clone the repository to").Default("./").String(),
		Repository: app.Flag("repos_url", "The repository URL to clone").Required().String(),
		Remote:     app.Flag("remote", "The remote name to deploy").Default("origin").String(),
		Ref:        app.Flag("ref", "The reference name to deploy").Default("refs/heads/main").String(),
		Port:       app.Flag("port", "The port to listen on").Default("8080").Uint(),
		Context:    app.Flag("context", "The context path for docker build").Short('c').Default(".").String(),
		Dockerfile: app.Flag("dockerfile", "The Dockerfile path for docker build").Short('f').Default("Dockerfile").String(),
		ImageName:  app.Flag("image_name", "The image name for docker build").Short('i').Required().String(),
		Tag:        app.Flag("tag", "The tag name for docker build").Short('t').Default("latest").String(),
		Run:        app.Flag("run", "Run a build on starting").Default("false").Bool(),
		SSH:        app.Flag("ssh", "Deploy target (\"user@host:port\")").Strings(),
		Post:       app.Flag("post", "Post command to run after loading docker image").String(),
		ChanID:     app.Flag("ch", "The slack channel ID to post to").String(),
	}
	kingpin.MustParse(app.Parse(os.Args[1:]))
	return cmd
}
