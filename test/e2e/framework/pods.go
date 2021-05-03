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

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// AwaitPodsByAppLabel finds pods in a given cluster whose 'app' label value matches a specified value. If the specified
// expectedCount >= 0, the function waits until the number of pods equals the expectedCount.
func (f *Framework) AwaitPodsByAppLabel(cluster ClusterIndex, appName string, namespace string, expectedCount int) *v1.PodList {
	return AwaitUntil("find pods for app "+appName, func() (interface{}, error) {
		return KubeClients[cluster].CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "app=" + appName,
		})
	}, func(result interface{}) (bool, string, error) {
		pods := result.(*v1.PodList)
		if expectedCount >= 0 && len(pods.Items) != expectedCount {
			return false, fmt.Sprintf("Actual pod count %d does not match the expected pod count %d", len(pods.Items), expectedCount), nil
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase != v1.PodRunning {
				return false, fmt.Sprintf("Status for pod %q is %v", pod.Name, pod.Status.Phase), nil
			}
		}

		return true, "", nil
	}).(*v1.PodList)
}

// AwaitSubmarinerGatewayPod finds the submariner gateway pod in a given cluster, waiting if necessary for a period of time
// for the pod to materialize.
func (f *Framework) AwaitSubmarinerGatewayPod(cluster ClusterIndex) *v1.Pod {
	return &f.AwaitPodsByAppLabel(cluster, SubmarinerGateway, TestContext.SubmarinerNamespace, 1).Items[0]
}

// DeletePod deletes the pod for the given name and namespace.
func (f *Framework) DeletePod(cluster ClusterIndex, podName string, namespace string) {
	AwaitUntil("delete pod", func() (interface{}, error) {
		return nil, KubeClients[cluster].CoreV1().Pods(namespace).Delete(context.TODO(), podName, metav1.DeleteOptions{})
	}, NoopCheckResult)
}

// AwaitUntilAnnotationOnPod queries the Pod and looks for the presence of annotation.
func (f *Framework) AwaitUntilAnnotationOnPod(cluster ClusterIndex, annotation string, podName string, namespace string) *v1.Pod {
	return AwaitUntil("get "+annotation+" annotation for pod "+podName, func() (interface{}, error) {
		pod, err := KubeClients[cluster].CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return pod, err
	}, func(result interface{}) (bool, string, error) {
		if result == nil {
			return false, "No Pod found", nil
		}

		pod := result.(*v1.Pod)
		if pod.GetAnnotations()[annotation] == "" {
			return false, fmt.Sprintf("Pod %q does not have annotation %q yet", podName, annotation), nil
		}
		return true, "", nil
	}).(*v1.Pod)
}

// AwaitRouteAgentPodOnNode finds the route agent pod on a given node in a cluster, waiting if necessary for a period of time
// for the pod to materialize. If prevPodUID is non-empty, the found pod's UID must not match it.
func (f *Framework) AwaitRouteAgentPodOnNode(cluster ClusterIndex, nodeName string, prevPodUID types.UID) *v1.Pod {
	var found *v1.Pod

	AwaitUntil(fmt.Sprintf("find route agent pod on node %q", nodeName), func() (interface {
	}, error) {
		return KubeClients[cluster].CoreV1().Pods(TestContext.SubmarinerNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "app=" + RouteAgent,
		})
	}, func(result interface{}) (bool, string, error) {
		pods := result.(*v1.PodList)
		var podNodes []string
		for i := range pods.Items {
			pod := &pods.Items[i]
			if pod.Spec.NodeName == nodeName {
				if pod.Status.Phase != v1.PodRunning {
					return false, fmt.Sprintf("Found pod %q but its Status is %v", pod.Name, pod.Status.Phase), nil
				}

				if prevPodUID != "" && pod.UID == prevPodUID {
					return false, fmt.Sprintf("Expecting new route agent pod (UID %q matches previous instance)", prevPodUID), nil
				}

				found = pod
				return true, "", nil
			}

			podNodes = append(podNodes, pod.Spec.NodeName)
		}

		return false, fmt.Sprintf("Found pods on nodes %v", podNodes), nil
	})

	return found
}
