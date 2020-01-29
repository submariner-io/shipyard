package deploy

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/streaming"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Resources deploys k8s resources
func Resources(cluster string, client client.Client, deploymentFile string, resourceName string) error {
	yamlDecoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	reader := json.YAMLFramer.NewFrameReader(ioutil.NopCloser(bytes.NewReader([]byte(deploymentFile))))
	decoder := streaming.NewDecoder(reader, yamlDecoder)
	for {
		obj, groupVersionKind, err := decoder.Decode(nil, nil)
		if err != nil {
			if err == io.EOF {
				break
			}

			return errors.Wrapf(err, "Error decoding YAML.")
		}

		err = client.Create(context.TODO(), obj)
		if err != nil {
			if !apierr.IsAlreadyExists(err) {
				return errors.Wrapf(err, "Error creating resource of type %q.", groupVersionKind)
			}
		} else {
			metadata, _ := meta.Accessor(obj)
			log.Infof("✔ %s %q was deployed in cluster %q.", groupVersionKind.Kind, metadata.GetName(), cluster)
		}
	}

	log.Infof("✔ All %s resources were deployed in cluster %q.", resourceName, cluster)
	return nil
}
