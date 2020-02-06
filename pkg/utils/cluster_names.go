package utils

import (
	"io/ioutil"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/submariner-io/armada/pkg/defaults"
)

var logFatal = log.Fatal

// ClusterName returns the canonical cluster name based on the given number
func ClusterName(clusterNum int) string {
	return defaults.ClusterNameBase + strconv.Itoa(clusterNum)
}

// ClusterNamesFromFiles will return all clusters from the existing kind files.
// An error is returned if there's a failure to read the config directory.
func ClusterNamesFromFiles() ([]string, error) {
	var clusters []string
	files, err := ioutil.ReadDir(defaults.KindConfigDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		clusterName := strings.FieldsFunc(file.Name(), func(r rune) bool { return strings.ContainsRune(" -.", r) })[2]
		clusters = append(clusters, clusterName)
	}

	return clusters, nil
}

// ClusterNamesOrAll will return the cluster names sent to it, if there are any.
// In case the slice is empty, it will read the cluster names from the existing kind files.
// Should the read fail, it will fatally log the error (causing the process to abort).
func ClusterNamesOrAll(clusters []string) []string {
	if len(clusters) > 0 {
		return clusters
	}

	clusters, err := ClusterNamesFromFiles()
	if err != nil {
		logFatal(err)
	}

	return clusters
}
