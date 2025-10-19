/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package containers

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/blang/semver/v4"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerfilters "github.com/docker/docker/api/types/filters"
	dockerimage "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockersystem "github.com/docker/docker/api/types/system"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/test/infrastructure/kind"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type dockerHelper struct {
	dockerClient *dockerclient.Client

	cluster        *clusterv1beta1.Cluster
	virtualMachine client.Object
}

func (h *dockerHelper) GetContainer(ctx context.Context) (*dockercontainer.Summary, error) {
	listOptions := dockercontainer.ListOptions{
		All:     true,
		Limit:   -1,
		Filters: dockerfilters.NewArgs(),
	}
	listOptions.Filters.Add("name", h.containerName())

	dockerContainers, err := h.dockerClient.ContainerList(ctx, listOptions)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list containers")
	}
	if len(dockerContainers) == 1 {
		return &dockerContainers[0], nil
	}
	return nil, nil
}

func (h *dockerHelper) CreateContainer(ctx context.Context, kubernetesVersion string) error {
	name := h.containerName()

	semVer, err := semver.ParseTolerant(kubernetesVersion)
	if err != nil {
		return errors.Wrap(err, "failed to parse DockerMachine version")
	}
	kindMapping := kind.GetMapping(semVer, "")
	image := kindMapping.Image

	containerConfig := dockercontainer.Config{
		Tty:      true, // allocate a tty for entrypoint logs
		Hostname: name, // make hostname match container name
		Image:    image,
		Volumes:  map[string]struct{}{},
	}

	hostConfig := dockercontainer.HostConfig{
		// Running containers in a container requires privileges.
		// NOTE: we could try to replicate this with --cap-add, and use less
		// privileges, but this flag also changes some mounts that are necessary
		// including some ones docker would otherwise do by default.
		// for now this is what we want. in the future we may revisit this.
		Privileged:  true,
		SecurityOpt: []string{"seccomp=unconfined", "apparmor=unconfined"}, // ignore seccomp
		NetworkMode: dockercontainer.NetworkMode("kind"),
		Tmpfs: map[string]string{
			"/tmp": "", // various things depend on working /tmp
			"/run": "", // systemd wants a writable /run
		},
		PortBindings:  nat.PortMap{},
		RestartPolicy: dockercontainer.RestartPolicy{Name: "on-failure", MaximumRetryCount: 1},
		Init:          ptr.To(false),

		// starting from Kind 0.20 kind requires CgroupnsMode to be set to private.
		CgroupnsMode: "private",
	}
	networkConfig := network.NetworkingConfig{}

	info, err := h.dockerClient.Info(ctx)
	if err != nil {
		return errors.Wrapf(err, "unable to get Docker engine info, failed to create container %q", name)
	}

	// mount /dev/mapper if docker storage driver if Btrfs or ZFS
	// https://github.com/kubernetes-sigs/kind/pull/1464
	if info.Driver == "btrfs" || info.Driver == "zfs" {
		hostConfig.Binds = append(hostConfig.Binds, "/dev/mapper:/dev/mapper:ro")
	}

	// runtime persistent storage
	// this ensures that E.G. pods, logs etc. are not on the container
	// filesystem.
	// Some k8s things want to read /lib/modules
	seLinux := isSELinuxEnforcing()
	if seLinux {
		hostConfig.Binds = append(hostConfig.Binds, fmt.Sprintf("%s:%s:z", "/var", ""))
		hostConfig.Binds = append(hostConfig.Binds, fmt.Sprintf("%s:%s:%s", "/lib/modules", "/lib/modules", "Z,ro"))
	} else {
		hostConfig.Binds = append(hostConfig.Binds, fmt.Sprintf("%s:%s", "/var", ""))
		hostConfig.Binds = append(hostConfig.Binds, fmt.Sprintf("%s:%s:%s", "/lib/modules", "/lib/modules", "ro"))
	}

	if usernsRemap(info) {
		// We need this argument in order to make this command work
		// in systems that have userns-remap enabled on the docker daemon
		hostConfig.UsernsMode = "host"
	}

	// enable /dev/fuse explicitly for fuse-overlayfs
	// (Rootless Docker does not automatically mount /dev/fuse with --privileged)
	if mountFuse(info) {
		hostConfig.Devices = append(hostConfig.Devices, dockercontainer.DeviceMapping{PathOnHost: "/dev/fuse"})
	}

	// Make sure we have the image
	if err := h.pullContainerImageIfNotExists(ctx, image); err != nil {
		return errors.Wrapf(err, "error pulling container image %s", image)
	}

	// Create the container using our settings
	resp, err := h.dockerClient.ContainerCreate(
		ctx,
		&containerConfig,
		&hostConfig,
		&networkConfig,
		nil,
		name,
	)
	if err != nil {
		return errors.Wrapf(err, "error creating container %q", name)
	}

	// Actually start the container
	if err := h.dockerClient.ContainerStart(ctx, resp.ID, dockercontainer.StartOptions{}); err != nil {
		err := errors.Wrapf(err, "error starting container %q", name)
		// Delete the container and retry later on. This helps getting around the race
		// condition where of hitting "port is already allocated" issues.
		if reterr := h.dockerClient.ContainerRemove(ctx, resp.ID, dockercontainer.RemoveOptions{Force: true, RemoveVolumes: true}); reterr != nil {
			return kerrors.NewAggregate([]error{err, errors.Wrapf(reterr, "error deleting container")})
		}
		return err
	}

	containerJSON, err := h.dockerClient.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return fmt.Errorf("error inspecting container %s: %v", resp.ID, err)
	}

	if containerJSON.State.ExitCode != 0 {
		return fmt.Errorf("error container run failed with exit code %d", containerJSON.State.ExitCode)
	}

	return nil
}

func (h *dockerHelper) containerName() string {
	return strings.ReplaceAll(fmt.Sprintf("%s-%s", h.virtualMachine.GetNamespace(), h.virtualMachine.GetName()), ".", "_")
}

func (h *dockerHelper) pullContainerImageIfNotExists(ctx context.Context, image string) error {
	imageExistsLocally, err := h.imageExistsLocally(ctx, image)
	if err != nil {
		return errors.Wrapf(err, "failure determining if the image exists in local cache: %s", image)
	}
	if imageExistsLocally {
		return nil
	}

	return h.pullContainerImage(ctx, image)
}

func (h *dockerHelper) pullContainerImage(ctx context.Context, image string) error {
	pullResp, err := h.dockerClient.ImagePull(ctx, image, dockerimage.PullOptions{})
	if err != nil {
		return fmt.Errorf("failure pulling container image: %v", err)
	}
	defer pullResp.Close()

	// Clients must read the ImagePull response to EOF to complete the pull
	// operation or errors can occur.
	if _, err = io.ReadAll(pullResp); err != nil {
		return fmt.Errorf("error while reading container image: %v", err)
	}

	return nil
}

func (h *dockerHelper) imageExistsLocally(ctx context.Context, image string) (bool, error) {
	filters := dockerfilters.NewArgs()
	filters.Add("reference", image)
	images, err := h.dockerClient.ImageList(ctx, dockerimage.ListOptions{
		Filters: filters,
	})
	if err != nil {
		return false, errors.Wrapf(err, "failure listing container image: %s", image)
	}
	if len(images) > 0 {
		return true, nil
	}
	return false, nil
}

// usernsRemap checks if userns-remap is enabled in dockerd.
func usernsRemap(info dockersystem.Info) bool {
	for _, secOpt := range info.SecurityOptions {
		if strings.Contains(secOpt, "name=userns") {
			return true
		}
	}
	return false
}

// rootless: use fuse-overlayfs by default
// https://github.com/kubernetes-sigs/kind/issues/2275
func mountFuse(info dockersystem.Info) bool {
	for _, o := range info.SecurityOptions {
		// o is like "name=seccomp,profile=default", or "name=rootless",
		csvReader := csv.NewReader(strings.NewReader(o))
		sliceSlice, err := csvReader.ReadAll()
		if err != nil {
			return false
		}
		for _, f := range sliceSlice {
			for _, ff := range f {
				if ff == "name=rootless" {
					return true
				}
			}
		}
	}
	return false
}

func isSELinuxEnforcing() bool {
	dat, err := os.ReadFile("/sys/fs/selinux/enforce")
	if err != nil {
		return false
	}
	return string(dat) == "1"
}
