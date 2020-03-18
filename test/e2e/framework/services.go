package framework

import (
	"fmt"

	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

	services := f.ClusterClients[cluster].CoreV1().Services(f.Namespace)

	return AwaitUntil("create service", func() (interface{}, error) {
		service, err := services.Create(&tcpService)
		if errors.IsAlreadyExists(err) {
			err = services.Delete(tcpService.Name, &metav1.DeleteOptions{})
			if err != nil {
				return nil, err
			}

			service, err = services.Create(&tcpService)
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
						IntVal: 80,
					},
				},
			},
			Selector: map[string]string{
				"app": "nginx-demo",
			},
		},
	}

	sc := f.ClusterClients[cluster].CoreV1().Services(f.Namespace)
	service := AwaitUntil("create service", func() (interface{}, error) {
		return sc.Create(&nginxService)

	}, NoopCheckResult).(*corev1.Service)
	return service
}

func (f *Framework) DeleteService(cluster ClusterIndex, serviceName string) {
	ginkgo.By(fmt.Sprintf("Deleting service %q on %q", serviceName, TestContext.ClusterIDs[cluster]))
	AwaitUntil("delete service", func() (interface{}, error) {
		return nil, f.ClusterClients[cluster].CoreV1().Services(f.Namespace).Delete(serviceName, &metav1.DeleteOptions{})
	}, NoopCheckResult)
}
