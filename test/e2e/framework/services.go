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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	TestAppLabel = "test-app"
)

func (f *Framework) NewService(name, portName string, port int, protocol corev1.Protocol, selector map[string]string,
	isHeadless bool,
) *corev1.Service {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Port:       int32(port),
				Name:       portName,
				TargetPort: intstr.FromInt(port),
				Protocol:   protocol,
			}},
		},
	}

	if selector != nil {
		service.Spec.Selector = selector
	}

	if isHeadless {
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.ClusterIP = corev1.ClusterIPNone
	}

	return &service
}

func (f *Framework) CreateTCPService(cluster ClusterIndex, selectorName string, port int) *corev1.Service {
	tcpService := f.NewService(fmt.Sprintf("test-svc-%s", selectorName), "tcp", port, corev1.ProtocolTCP,
		map[string]string{TestAppLabel: selectorName}, false)
	sc := KubeClients[cluster].CoreV1().Services(f.Namespace)

	return f.CreateService(sc, tcpService)
}

func (f *Framework) CreateHeadlessTCPService(cluster ClusterIndex, selectorName string, port int) *corev1.Service {
	tcpService := f.NewService(fmt.Sprintf("test-svc-%s", selectorName), "tcp", port, corev1.ProtocolTCP,
		map[string]string{TestAppLabel: selectorName}, true)
	sc := KubeClients[cluster].CoreV1().Services(f.Namespace)

	return f.CreateService(sc, tcpService)
}

func (f *Framework) NewNginxService(cluster ClusterIndex) *corev1.Service {
	var tcpPort int32 = 80
	var metricsPort int32 = 8183
	nginxService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-demo",
			Labels: map[string]string{
				"app": "nginx-demo",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []corev1.ServicePort{
				{
					Port:     tcpPort,
					Name:     "http",
					Protocol: corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 8080,
					},
				},
				{
					Port:     metricsPort,
					Name:     "metrics",
					Protocol: corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 8082,
					},
				},
			},
			Selector: map[string]string{
				"app": "nginx-demo",
			},
		},
	}

	sc := KubeClients[cluster].CoreV1().Services(f.Namespace)

	return f.CreateService(sc, &nginxService)
}

func (f *Framework) CreateTCPServiceWithoutSelector(cluster ClusterIndex, svcName, portName string, port int) *corev1.Service {
	serviceSpec := f.NewService(svcName, portName, port, corev1.ProtocolTCP, nil, false)
	sc := KubeClients[cluster].CoreV1().Services(f.Namespace)

	return f.CreateService(sc, serviceSpec)
}

func (f *Framework) CreateService(sc typedv1.ServiceInterface, serviceSpec *corev1.Service) *corev1.Service {
	return AwaitUntil("create service", func() (interface{}, error) {
		service, err := sc.Create(context.TODO(), serviceSpec, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			err = sc.Delete(context.TODO(), serviceSpec.Name, metav1.DeleteOptions{})
			if err != nil {
				return nil, err
			}

			service, err = sc.Create(context.TODO(), serviceSpec, metav1.CreateOptions{})
		}

		return service, err
	}, NoopCheckResult).(*corev1.Service)
}

func (f *Framework) DeleteService(cluster ClusterIndex, serviceName string) {
	By(fmt.Sprintf("Deleting service %q on %q", serviceName, TestContext.ClusterIDs[cluster]))
	AwaitUntil("delete service", func() (interface{}, error) {
		return nil, KubeClients[cluster].CoreV1().Services(f.Namespace).Delete(context.TODO(), serviceName, metav1.DeleteOptions{})
	}, NoopCheckResult)
}
