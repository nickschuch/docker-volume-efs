package main

import (
	"github.com/alecthomas/kingpin"
	"github.com/fsouza/go-dockerclient"
)

var (
	cliDocker = kingpin.Flag("docker", "The Docker endpoint.").Default("unix:///var/run/docker.sock").OverrideDefaultFromEnvar("DOCKER_HOST").String()
)

func GetContainerByMount(path string) ([]*docker.Container, error) {
	var containers []*docker.Container

	client, err := docker.NewClient(*cliDocker)
	if err != nil {
		return containers, err
	}

	list, err := client.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		return containers, err
	}

	for _, c := range list {
		container, err := client.InspectContainer(c.ID)
		if err != nil {
			continue
		}

		// Check if this mount implements our path.
		for _, m := range container.Config.Mounts {
			if m.Source == path {
				containers = append(containers, container)
				continue
			}
		}
	}

	return containers, nil
}
