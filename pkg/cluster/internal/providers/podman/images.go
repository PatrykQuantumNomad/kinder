/*
Copyright 2019 The Kubernetes Authors.

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

package podman

import (
	"fmt"
	"strings"
	"time"

	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/cluster/internal/providers/common"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
	"sigs.k8s.io/kind/pkg/internal/cli"
)

// ensureNodeImages ensures that the node images used by the create
// configuration are present
func ensureNodeImages(logger log.Logger, status *cli.Status, cfg *config.Cluster) error {
	if cfg.AirGapped {
		return checkAllImagesPresent(logger, status, cfg)
	}
	// pull each required image
	for _, image := range common.RequiredNodeImages(cfg).List() {
		// prints user friendly message
		friendlyImageName, image := sanitizeImage(image)
		status.Start(fmt.Sprintf("Ensuring node image (%s) 🖼", friendlyImageName))
		if _, err := pullIfNotPresent(logger, image, 4); err != nil {
			status.End(false)
			return err
		}
	}
	return nil
}

// checkAllImagesPresent verifies all required images exist locally.
// In air-gapped mode, no pulls are attempted. Returns a single error listing
// every missing image so the user can pre-load them all in one pass.
func checkAllImagesPresent(logger log.Logger, status *cli.Status, cfg *config.Cluster) error {
	status.Start("Checking local images (air-gapped mode) 🔒")
	allRequired := common.RequiredAllImages(cfg)
	var missing []string
	for _, image := range allRequired.List() {
		cmd := exec.Command("podman", "inspect", "--type=image", image)
		if err := cmd.Run(); err != nil {
			missing = append(missing, image)
		}
	}
	if len(missing) > 0 {
		status.End(false)
		return formatMissingImagesError(missing)
	}
	status.End(true)
	logger.V(0).Info(" * All required images present locally")
	return nil
}

// formatMissingImagesError returns a human-readable error listing all missing
// images and providing pre-load instructions for podman.
func formatMissingImagesError(missing []string) error {
	var b strings.Builder
	b.WriteString("air-gapped mode: the following required images are not present locally:\n\n")
	for _, img := range missing {
		b.WriteString("  ")
		b.WriteString(img)
		b.WriteString("\n")
	}
	b.WriteString("\nPre-load these images before using --air-gapped. For example:\n")
	b.WriteString("  podman pull <image>        # on a machine with internet access\n")
	b.WriteString("  podman save <image> | gzip > image.tar.gz\n")
	b.WriteString("  podman load < image.tar.gz # on the air-gapped machine\n")
	return errors.New(b.String())
}

// pullIfNotPresent will pull an image if it is not present locally
// retrying up to retries times
// it returns true if it attempted to pull, and any errors from pulling
func pullIfNotPresent(logger log.Logger, image string, retries int) (pulled bool, err error) {
	// TODO(bentheelder): switch most (all) of the logging here to debug level
	// once we have configurable log levels
	// if this did not return an error, then the image exists locally
	cmd := exec.Command("podman", "inspect", "--type=image", image)
	if err := cmd.Run(); err == nil {
		logger.V(1).Infof("Image: %s present locally", image)
		return false, nil
	}
	// otherwise try to pull it
	return true, pull(logger, image, retries)
}

// pull pulls an image, retrying up to retries times
func pull(logger log.Logger, image string, retries int) error {
	logger.V(1).Infof("Pulling image: %s ...", image)
	err := exec.Command("podman", "pull", image).Run()
	// retry pulling up to retries times if necessary
	if err != nil {
		for i := 0; i < retries; i++ {
			time.Sleep(time.Second * time.Duration(i+1))
			logger.V(1).Infof("Trying again to pull image: %q ... %v", image, err)
			// TODO(bentheelder): add some backoff / sleep?
			err = exec.Command("podman", "pull", image).Run()
			if err == nil {
				break
			}
		}
	}
	return errors.Wrapf(err, "failed to pull image %q", image)
}

// sanitizeImage is a helper to return human readable image name and
// the podman pullable image name from the provided image
func sanitizeImage(image string) (friendlyImageName, pullImageName string) {
	const (
		defaultDomain    = "docker.io/"
		officialRepoName = "library"
	)

	var remainder string

	if strings.Contains(image, "@sha256:") {
		splits := strings.Split(image, "@sha256:")
		friendlyImageName = splits[0]
		remainder = strings.Split(splits[0], ":")[0] + "@sha256:" + splits[1]
	} else {
		friendlyImageName = image
		remainder = image
	}

	if !strings.ContainsRune(remainder, '/') {
		remainder = officialRepoName + "/" + remainder
	}

	i := strings.IndexRune(friendlyImageName, '/')
	if i == -1 || (!strings.ContainsAny(friendlyImageName[:i], ".:") && friendlyImageName[:i] != "localhost") {
		pullImageName = defaultDomain + remainder
	} else {
		pullImageName = remainder
	}

	return
}
