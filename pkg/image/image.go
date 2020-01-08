package image

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"

	dockerclient "github.com/docker/docker/client"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/submariner-io/armada/pkg/cluster"
	kind "sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
)

// GetLocalID returns local image id by name/reference
func GetLocalID(ctx context.Context, dockerCli *dockerclient.Client, imageName string) (string, error) {
	imageFilter := filters.NewArgs()
	imageFilter.Add("reference", imageName)
	result, err := dockerCli.ImageList(ctx, types.ImageListOptions{
		All:     false,
		Filters: imageFilter,
	})
	if err != nil {
		return "", err
	}
	if len(result) == 0 {
		return "", errors.Errorf("Image %s not found locally.", imageName)
	}
	return result[0].ID, nil
}

// GetNodesWithout return a list of nodes that don't have the image for multiple clusters
func GetNodesWithout(provider *kind.Provider, imageName, localImageID string, clusters []string) ([]nodes.Node, error) {
	var selectedNodes []nodes.Node
	for _, clName := range clusters {
		known, err := cluster.IsKnown(clName, provider)
		if err != nil {
			return nil, err
		}
		if known {
			nodeList, err := provider.ListInternalNodes(clName)
			if err != nil {
				return nil, err
			}
			if len(nodeList) == 0 {
				return nil, errors.Errorf("no nodes found for cluster %q", clName)
			}
			// pick only the nodes that don't have the image
			for _, node := range nodeList {
				nodeImageID, err := nodeutils.ImageID(node, imageName)
				if err != nil || nodeImageID != localImageID {
					selectedNodes = append(selectedNodes, node)
					log.Debugf("%s: image: %q with ID %q not present on node %q", clName, imageName, localImageID, node.String())
				}
				if nodeImageID == localImageID {
					log.Infof("%s: ✔ image with ID %q already present on node %q", clName, nodeImageID, node.String())
				}
			}
		} else {
			return selectedNodes, errors.Errorf("cluster %q not found.", clName)
		}
	}
	return selectedNodes, nil
}

// Save saves the image to tar and returns temp file location
func Save(ctx context.Context, dockerCli *dockerclient.Client, imageName string) (string, error) {
	// Create temp dor to images tar
	tempDirName, err := ioutil.TempDir("", "image-tar")
	if err != nil {
		log.Fatal(err)
	}
	// on macOS $TMPDIR is typically /var/..., which is not mountable
	// /private/var/... is the mountable equivalent
	if runtime.GOOS == "darwin" && strings.HasPrefix(tempDirName, "/var/") {
		tempDirName = filepath.Join("/private", tempDirName)
	}

	tmpFilePath, err := ioutil.TempFile(tempDirName, "image_")
	if err != nil {
		return "", err
	}

	imageBody, err := dockerCli.ImageSave(ctx, []string{imageName})
	if err != nil {
		return "", err
	}
	defer imageBody.Close()

	_, err = io.Copy(tmpFilePath, imageBody)
	defer tmpFilePath.Close()

	if err != nil {
		return "", err
	}
	return tmpFilePath.Name(), nil
}

// LoadToNode loads an image to kubernetes node
func LoadToNode(imageTarPath, imageName string, node nodes.Node, wg *sync.WaitGroup) error {
	f, err := os.Open(imageTarPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open an image: %s, location: %q", imageName, imageTarPath)
	}
	defer f.Close()

	log.Debugf("loading image: %q, path: %q to node: %q ...", imageName, imageTarPath, node.String())
	err = nodeutils.LoadImageArchive(node, f)
	if err != nil {
		return errors.Wrapf(err, "failed to loading image: %q, node %q", imageName, node.String())
	}
	log.Infof("✔ image: %q was loaded to node: %q.", imageName, node.String())
	wg.Done()
	return nil
}
