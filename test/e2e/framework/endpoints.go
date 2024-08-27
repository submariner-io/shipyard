/*
SPDX-License-Identifier: Apache-2.0

Copyright Contributors to the Submariner project.

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

package framework

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func (f *Framework) CreateTCPEndpoints(cluster ClusterIndex, epName, portName, address string, port int32) *corev1.Endpoints {
	endpointsSpec := corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: epName,
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{IP: address},
				},
				Ports: []corev1.EndpointPort{
					{
						Name:     portName,
						Port:     port,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		},
	}

	ec := KubeClients[cluster].CoreV1().Endpoints(f.Namespace)

	return createEndpoints(ec, &endpointsSpec)
}

func createEndpoints(ec typedv1.EndpointsInterface, endpointsSpec *corev1.Endpoints) *corev1.Endpoints {
	return AwaitUntil("create endpoints", func() (interface{}, error) {
		ep, err := ec.Create(context.TODO(), endpointsSpec, metav1.CreateOptions{})
		if errors.IsAlreadyExists(err) {
			err = ec.Delete(context.TODO(), endpointsSpec.Name, metav1.DeleteOptions{})
			if err != nil {
				return nil, err
			}

			ep, err = ec.Create(context.TODO(), endpointsSpec, metav1.CreateOptions{})
		}

		return ep, err
	}, NoopCheckResult).(*corev1.Endpoints)
}

func (f *Framework) DeleteEndpoints(cluster ClusterIndex, endpointsName string) {
	By(fmt.Sprintf("Deleting endpoints %q on %q", endpointsName, TestContext.ClusterIDs[cluster]))
	AwaitUntil("delete endpoints", func() (interface{}, error) {
		return nil, KubeClients[cluster].CoreV1().Endpoints(f.Namespace).Delete(context.TODO(), endpointsName, metav1.DeleteOptions{})
	}, NoopCheckResult)
}
