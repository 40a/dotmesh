package main

// docker client for finding containers which are using dm volumes, and
// stopping and starting containers.

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/fsouza/go-dockerclient"
)

type DockerContainer struct {
	Name string
	Id   string
}

type DockerClient struct {
	// TODO move the map to fsMachine
	client            *docker.Client
	containersStopped map[string]map[string]string
}

type NotLocked struct {
	volumeName string
}

func (n NotLocked) Error() string {
	return fmt.Sprintf("%s not locked when tried to unlock", n.volumeName)
}

type AlreadyLocked struct {
	volumeName string
}

func (a AlreadyLocked) Error() string {
	return fmt.Sprintf("%s already locked when tried to lock", a.volumeName)
}

// AllRelated returns every running container that is using any datamesh
// filesystem, as a map from filesystem ids to lists of such containers
func (d *DockerClient) AllRelated() (map[string][]DockerContainer, error) {
	relatedContainers := map[string][]DockerContainer{}
	containers, err := d.client.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		return relatedContainers, err
	}
	log.Printf("[AllRelated] got containers = %v", containers)
	for _, c := range containers {
		container, err := d.client.InspectContainer(c.ID)
		if err != nil {
			return relatedContainers, err
		}
		log.Printf("[AllRelated] inspect %v = %v", c, container)
		if container.State.Running {
			filesystems, err := d.relatedFilesystems(container)
			if err != nil {
				return map[string][]DockerContainer{}, err
			}
			for _, filesystem := range filesystems {
				_, ok := relatedContainers[filesystem]
				if !ok {
					relatedContainers[filesystem] = []DockerContainer{}
				}
				relatedContainers[filesystem] = append(
					relatedContainers[filesystem],
					DockerContainer{Id: container.ID, Name: container.Name},
				)
			}
		}
	}
	return relatedContainers, nil
}

func containerRelated(volumeName string, container *docker.Container) bool {
	for _, m := range container.Mounts {
		log.Printf("comparing volume %s with mount name %s", volumeName, m.Name)
		// split off "@" in mount name, in case of pinned branches. we want to
		// stop those containers too. (and later we may want to be more precise
		// about not stopping containers that are currently using other
		// branches)
		mountName := m.Name
		if strings.Contains(mountName, "@") {
			shrapnel := strings.Split(mountName, "@")
			mountName = shrapnel[0]
		}
		if m.Driver == "dm" && mountName == volumeName {
			return true
		}
	}
	return false
}

// Given a container, return a list of filesystem ids of datamesh volumes that
// are currently in-use by it (by resolving the symlinks of its mount sources).
func (d *DockerClient) relatedFilesystems(container *docker.Container) ([]string, error) {
	result := []string{}
	for _, mount := range container.Mounts {
		if mount.Driver != "dm" {
			continue
		}
		target, err := os.Readlink(mount.Source)
		if err != nil {
			log.Printf("Error trying to read symlink '%s', skipping: %s", mount.Source, err)
			continue
		}
		// target will be like
		// /var/lib/docker/datamesh/mnt/dmfs/9e394010-0f2b-481d-779d-d81c2d4f51fb
		log.Printf("[relatedFilesystems] target = %s\n", target)
		shrapnel := strings.Split(target, "/")
		if len(shrapnel) > 1 {
			filesystemId := shrapnel[len(shrapnel)-1]
			result = append(result, filesystemId)
		}
	}
	return result, nil
}

func NewDockerClient() (*DockerClient, error) {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	stopped := map[string]map[string]string{}
	return &DockerClient{client, stopped}, nil
}

func (d *DockerClient) Related(volumeName string) ([]DockerContainer, error) {
	related := []DockerContainer{}
	cs, err := d.client.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		return related, err
	}
	for _, c := range cs {
		container, err := d.client.InspectContainer(c.ID)
		if err != nil {
			return related, err
		}
		if container.State.Running && containerRelated(volumeName, container) {
			related = append(
				related, DockerContainer{
					Id: container.ID, Name: container.Name,
				},
			)
		}
	}
	return related, nil
}

func (d *DockerClient) SwitchSymlinks(volumeName, toFilesystemIdPath string) error {
	// iterate over all the containers, finding mounts where the name of the
	// mount is volumeName. assuming the container is stopped, unlink the
	// symlink and create a new one, with the same filename, pointing to
	// toFilesystemIdPath
	containers, err := d.client.ListContainers(
		docker.ListContainersOptions{All: true},
	)
	if err != nil {
		return err
	}
	// TODO something like the following structure (+locking) can also be used
	// for "cleaning up" stale /datamesh mount symlinks, say every 60 seconds,
	// or when docker says unmount/remove.

	for _, c := range containers {
		log.Printf("SwitchSymlinks inspecting container %s", c)
		container, err := d.client.InspectContainer(c.ID)
		if err != nil {
			return err
		}
		for _, mount := range container.Mounts {
			if mount.Driver == "dm" {
				// TODO the only purpose for this Readlink call is to check
				// whether it's a symlink before trying os.Remove. maybe we can
				// check whether it's a symlink with Stat instead.
				_, err := os.Readlink(mount.Source)
				if err != nil {
					log.Printf("Error trying to read symlink '%s', skipping: %s", mount.Source, err)
					continue
				}
				if mount.Name == volumeName {
					// TODO could also check whether containerMnt(volumeName) == mount.Source. should we?
					if container.State.Running {
						return fmt.Errorf(
							"Container %s was running when you asked me to switch its symlinks (%s => %s)",
							c.ID, mount.Source, toFilesystemIdPath,
						)
					}
					// ok, container is not running, and we know the symlink to
					// switcharoo.
					log.Printf("Switching %s to %s", mount.Source, toFilesystemIdPath)
					if err := os.Remove(mount.Source); err != nil {
						return err
					}
					if err := os.Symlink(toFilesystemIdPath, mount.Source); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (d *DockerClient) Start(volumeName string) error {
	stopped, ok := d.containersStopped[volumeName]
	if !ok {
		return NotLocked{volumeName: volumeName}
	}
	for containerId := range stopped {
		err := d.client.StartContainer(containerId, nil)
		if err != nil {
			return err
		}
	}
	delete(d.containersStopped, volumeName)
	return nil
}

func (d *DockerClient) Stop(volumeName string) error {
	_, ok := d.containersStopped[volumeName]
	if ok {
		return AlreadyLocked{volumeName: volumeName}
	}
	relatedContainers, err := d.Related(volumeName)
	if err != nil {
		return err
	}
	d.containersStopped[volumeName] = map[string]string{}

	for _, container := range relatedContainers {
		err = func() error {
			var err error
			for i := 0; i < 10; i++ {
				err = d.client.StopContainer(container.Id, 10)
				if err == nil {
					return nil
				}
			}
			return err
		}()
		if err != nil {
			return err
		}
		d.containersStopped[volumeName][container.Id] = container.Name
	}
	return nil
}
