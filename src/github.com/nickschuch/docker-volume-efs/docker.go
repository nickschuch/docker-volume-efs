package main

import (
	"strings"

	"github.com/alecthomas/kingpin"
	"github.com/fsouza/go-dockerclient"
)

var (
	cliDocker = kingpin.Flag("docker", "The Docker endpoint.").Default("unix:///var/run/docker.sock").OverrideDefaultFromEnvar("DOCKER_HOST").String()
)

func GetDockerBinds() ([]string, error) {
	var binds []string

	client, err := docker.NewClient(*cliDocker)
	if err != nil {
		return binds, err
	}

	list, err := client.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		return binds, err
	}

	for _, c := range list {
		container, err := client.InspectContainer(c.ID)
		if err != nil {
			continue
		}

		for _, b := range container.HostConfig.Binds {
			s := strings.Split(b, ":")
			binds = append(binds, s[0])
		}
	}

	return binds, nil
}
