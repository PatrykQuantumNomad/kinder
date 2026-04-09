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

// Package images implements the `load images` command
package images

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/fs"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/internal/runtime"
)

type (
	imageTagFetcher func(nodes.Node, string) (map[string]bool, error)
)

type flagpole struct {
	Name  string
	Nodes []string
}

// NewCommand returns a new cobra.Command for loading images into a cluster
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	cmd := &cobra.Command{
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("a list of image names is required")
			}
			return nil
		},
		Use:   "images <IMAGE> [IMAGE...]",
		Short: "Loads images from host into nodes",
		Long:  "Loads images from host into all or specified nodes using the active provider's native image export",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.OverrideDefaultName(cmd.Flags())
			return runE(logger, flags, args)
		},
	}
	cmd.Flags().StringVarP(
		&flags.Name,
		"name",
		"n",
		cluster.DefaultName,
		"the cluster context name",
	)
	cmd.Flags().StringSliceVar(
		&flags.Nodes,
		"nodes",
		nil,
		"comma separated list of nodes to load images into",
	)
	return cmd
}

// providerBinaryName returns the binary name for host-side image operations.
// For docker and podman, the provider name IS the binary. For nerdctl variants
// (finch, nerdctl.lima), the actual binary differs from provider.Name() which
// always returns "nerdctl".
func providerBinaryName(provider *cluster.Provider) string {
	name := provider.Name()
	if name != "nerdctl" {
		return name // "docker" or "podman"
	}
	// For nerdctl provider, read the env var to get the actual binary
	if p := os.Getenv("KIND_EXPERIMENTAL_PROVIDER"); p == "finch" || p == "nerdctl.lima" || p == "nerdctl" {
		return p
	}
	return "nerdctl" // default fallback
}

func runE(logger log.Logger, flags *flagpole, args []string) error {
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
		runtime.GetDefault(logger),
	)

	binaryName := providerBinaryName(provider)

	// Check that the images exist locally and get their IDs
	imageNames := removeDuplicates(args)
	var imageIDs []string
	for _, imageName := range imageNames {
		id, err := imageID(binaryName, imageName)
		if err != nil {
			return fmt.Errorf("image: %q not present locally", imageName)
		}
		imageIDs = append(imageIDs, id)
	}

	// Check if the cluster nodes exist
	nodeList, err := provider.ListInternalNodes(flags.Name)
	if err != nil {
		return err
	}
	if len(nodeList) == 0 {
		return fmt.Errorf("no nodes found for cluster %q", flags.Name)
	}

	// map cluster nodes by their name
	nodesByName := map[string]nodes.Node{}
	for _, node := range nodeList {
		nodesByName[node.String()] = node
	}

	// pick only the user selected nodes and ensure they exist
	candidateNodes := nodeList
	if len(flags.Nodes) > 0 {
		candidateNodes = []nodes.Node{}
		for _, name := range flags.Nodes {
			node, ok := nodesByName[name]
			if !ok {
				return fmt.Errorf("unknown node: %q", name)
			}
			candidateNodes = append(candidateNodes, node)
		}
	}

	// pick only the nodes that don't have the image (LOAD-04: smart-load)
	selectedNodes := map[string]nodes.Node{}
	fns := []func() error{}
	for i, imageName := range imageNames {
		imageID := imageIDs[i]
		processed := false
		for _, node := range candidateNodes {
			exists, reTagRequired, sanitizedImageName := checkIfImageReTagRequired(node, imageID, imageName, nodeutils.ImageTags)
			if exists && !reTagRequired {
				continue
			}

			if reTagRequired {
				logger.V(0).Infof("Image with ID: %s already present on the node %s but is missing the tag %s. re-tagging...", imageID, node.String(), sanitizedImageName)
				if err := nodeutils.ReTagImage(node, imageID, sanitizedImageName); err != nil {
					logger.Errorf("failed to re-tag image on the node %s due to an error %s. Will load it instead...", node.String(), err)
					selectedNodes[node.String()] = node
				} else {
					processed = true
				}
				continue
			}
			id, err := nodeutils.ImageID(node, imageName)
			if err != nil || id != imageID {
				selectedNodes[node.String()] = node
				logger.V(0).Infof("Image: %q with ID %q not yet present on node %q, loading...", imageName, imageID, node.String())
			}
			continue
		}
		if len(selectedNodes) == 0 && !processed {
			logger.V(0).Infof("Image: %q with ID %q found to be already present on all nodes.", imageName, imageID)
		}
	}

	// return early if no node needs the image
	if len(selectedNodes) == 0 {
		return nil
	}

	// Setup the tar path where the images will be saved
	dir, err := fs.TempDir("", "images-tar")
	if err != nil {
		return errors.Wrap(err, "failed to create tempdir")
	}
	defer os.RemoveAll(dir) //nolint:errcheck
	imagesTarPath := filepath.Join(dir, "images.tar")

	// Save the images into a tar using the provider binary
	err = save(binaryName, imageNames, imagesTarPath)
	if err != nil {
		return err
	}

	// Load the images on the selected nodes using fallback-capable import
	for _, selectedNode := range selectedNodes {
		selectedNode := selectedNode // capture loop variable
		fns = append(fns, func() error {
			return loadImage(imagesTarPath, selectedNode)
		})
	}
	return errors.UntilErrorConcurrent(fns)
}

// loadImage loads an image tarball onto a node using the fallback-capable
// import that handles Docker Desktop 27+ containerd image store.
func loadImage(imageTarPath string, node nodes.Node) error {
	return nodeutils.LoadImageArchiveWithFallback(node, func() (io.ReadCloser, error) {
		return os.Open(imageTarPath)
	})
}

// save saves images to dest using the active provider binary
func save(binaryName string, images []string, dest string) error {
	commandArgs := append([]string{"save", "-o", dest}, images...)
	return exec.Command(binaryName, commandArgs...).Run()
}

// imageID returns the ID of a container image via the active provider binary
func imageID(binaryName, containerNameOrID string) (string, error) {
	cmd := exec.Command(binaryName, "image", "inspect",
		"-f", "{{ .Id }}",
		containerNameOrID,
	)
	lines, err := exec.OutputLines(cmd)
	if err != nil {
		return "", err
	}
	if len(lines) != 1 {
		return "", errors.Errorf("image ID should only be one line, got %d lines", len(lines))
	}
	return lines[0], nil
}

// removeDuplicates removes duplicates from a string slice
func removeDuplicates(slice []string) []string {
	result := []string{}
	seenKeys := make(map[string]struct{})
	for _, k := range slice {
		if _, seen := seenKeys[k]; !seen {
			result = append(result, k)
			seenKeys[k] = struct{}{}
		}
	}
	return result
}

// checkIfImageReTagRequired checks if an image exists on the node and whether
// it needs re-tagging (image ID present but tag missing)
func checkIfImageReTagRequired(node nodes.Node, imageID, imageName string, tagFetcher imageTagFetcher) (exists, reTagRequired bool, sanitizedImage string) {
	tags, err := tagFetcher(node, imageID)
	if len(tags) == 0 || err != nil {
		exists = false
		return
	}
	exists = true
	sanitizedImage = sanitizeImage(imageName)
	if ok := tags[sanitizedImage]; ok {
		reTagRequired = false
		return
	}
	reTagRequired = true
	return
}

// sanitizeImage normalizes an image reference to its canonical form
func sanitizeImage(image string) (sanitizedName string) {
	const (
		defaultDomain    = "docker.io/"
		officialRepoName = "library"
	)
	sanitizedName = image

	if !strings.ContainsRune(image, '/') {
		sanitizedName = officialRepoName + "/" + image
	}

	i := strings.IndexRune(sanitizedName, '/')
	if i == -1 || (!strings.ContainsAny(sanitizedName[:i], ".:") && sanitizedName[:i] != "localhost") {
		sanitizedName = defaultDomain + sanitizedName
	}

	i = strings.IndexRune(sanitizedName, ':')
	if i == -1 {
		sanitizedName += ":latest"
	}

	return
}
