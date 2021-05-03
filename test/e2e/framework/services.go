/*
© 2020 Red Hat, Inc.

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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	TestAppLabel = "test-app"
)

func (f *Framework) CreateTCPService(cluster ClusterIndex, selectorName string, port int) *corev1.Service {
	tcpService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("test-svc-%s", selectorName),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Port:       int32(port),
				TargetPort: intstr.FromInt(port),
				Protocol:   corev1.ProtocolTCP,
			}},
			Selector: map[string]string{
				TestAppLabel: selectorName,
			},
		},
	}

	services := KubeClients[cluster].CoreV1().Services(f.Namespace)

	return AwaitUntil("create service", func() (interface{}, error) {
		service, err := services.Create(context.TODO(), &tcpService, metav1.CreateOptions{})
		if errors.IsAlreadyExists(err) {
			err = services.Delete(context.TODO(), tcpService.Name, metav1.DeleteOptions{})
			if err != nil {
				return nil, err
			}

			service, err = services.Create(context.TODO(), &tcpService, metav1.CreateOptions{})
		}

		return service, err
	}, NoopCheckResult).(*corev1.Service)
}

func (f *Framework) NewNginxService(cluster ClusterIndex) *corev1.Service {
	var port int32 = 80
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
					Port:     port,
					Protocol: corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 8080,
					},
				},
			},
			Selector: map[string]string{
				"app": "nginx-demo",
			},
		},
	}

	sc := KubeClients[cluster].CoreV1().Services(f.Namespace)
	service := AwaitUntil("create service", func() (interface{}, error) {
		return sc.Create(context.TODO(), &nginxService, metav1.CreateOptions{})

	}, NoopCheckResult).(*corev1.Service)
	return service
}

func (f *Framework) DeleteService(cluster ClusterIndex, serviceName string) {
	By(fmt.Sprintf("Deleting service %q on %q", serviceName, TestContext.ClusterIDs[cluster]))
	AwaitUntil("delete service", func() (interface{}, error) {
		return nil, KubeClients[cluster].CoreV1().Services(f.Namespace).Delete(context.TODO(), serviceName, metav1.DeleteOptions{})
	}, NoopCheckResult)
}

// AwaitUntilAnnotationOnService queries the service and looks for the presence of annotation.
func (f *Framework) AwaitUntilAnnotationOnService(cluster ClusterIndex, annotation string, svcName string, namespace string) *v1.Service {
	return AwaitUntil("get"+annotation+" annotation for service "+svcName, func() (interface{}, error) {
		service, err := KubeClients[cluster].CoreV1().Services(namespace).Get(context.TODO(), svcName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return service, err
	}, func(result interface{}) (bool, string, error) {
		if result == nil {
			return false, "No Service found", nil
		}

		service := result.(*v1.Service)
		if service.GetAnnotations()[annotation] == "" {
			return false, fmt.Sprintf("Service %q does not have annotation %q yet", svcName, annotation), nil
		}
		return true, "", nil
	}).(*corev1.Service)
}
