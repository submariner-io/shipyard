package wait

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/submariner-io/armada/pkg/defaults"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ForPodsRunning waits for pods to be running
func ForPodsRunning(clName string, c kubernetes.Interface, namespace, selector string, replicas int) error {
	ctx := context.Background()
	log.Debugf("Waiting for pods to be running. label: %q, namespace: %q, replicas: %v, duration: %v, *types: %s.", selector, namespace, replicas, defaults.WaitDurationResources, clName)
	podsContext, cancel := context.WithTimeout(ctx, defaults.WaitDurationResources)
	wait.Until(func() {
		podList, err := c.CoreV1().Pods(namespace).List(metav1.ListOptions{
			LabelSelector: selector,
			FieldSelector: "status.phase=Running",
		})
		if err == nil && len(podList.Items) == replicas {
			log.Infof("✔ All pods with label: %q are running for %s.", selector, clName)
			cancel()
		} else {
			log.Debugf("Still waiting for pods. label: %q, namespace: %q, replicas: %v, cluster: %s.", selector, namespace, replicas, clName)
		}
	}, 15*time.Second, podsContext.Done())

	err := podsContext.Err()
	if err != nil && err != context.Canceled {
		return errors.Wrap(err, "Error waiting for pods to be running.")
	}
	return nil
}

// ForDeploymentReady waits for deployment roll out
func ForDeploymentReady(clName string, c kubernetes.Interface, namespace, deploymentName string) error {
	ctx := context.Background()
	log.Debugf("Waiting up to %v for %s deployment roll out %s ...", defaults.WaitDurationResources, deploymentName, clName)
	deploymentContext, cancel := context.WithTimeout(ctx, defaults.WaitDurationResources)
	wait.Until(func() {
		deployment, err := c.AppsV1().Deployments(namespace).Get(deploymentName, metav1.GetOptions{})
		if err == nil {
			if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
				log.Infof("✔ %s successfully deployed to %s, ready replicas: %v", deploymentName, clName, deployment.Status.ReadyReplicas)
				cancel()
			} else {
				log.Infof("Still waiting for %s deployment for %s, ready replicas: %v", deploymentName, clName, deployment.Status.ReadyReplicas)
			}
		} else {
			log.Debugf("Still waiting for %s deployment roll out %s ...", deploymentName, clName)
		}
	}, 15*time.Second, deploymentContext.Done())
	err := deploymentContext.Err()
	if err != nil && err != context.Canceled {
		return errors.Wrapf(err, "Error waiting for %s deployment roll out.", deploymentName)
	}
	return nil
}

// ForDaemonSetReady waits for daemon set roll out
func ForDaemonSetReady(clName string, c kubernetes.Interface, namespace, daemonSetName string) error {
	ctx := context.Background()
	log.Debugf("Waiting up to %v for %s daemon set roll out %s ...", defaults.WaitDurationResources, daemonSetName, clName)
	deploymentContext, cancel := context.WithTimeout(ctx, defaults.WaitDurationResources)
	wait.Until(func() {
		daemonSet, err := c.AppsV1().DaemonSets(namespace).Get(daemonSetName, metav1.GetOptions{})
		if err == nil && daemonSet.Status.CurrentNumberScheduled > 0 {
			if daemonSet.Status.NumberReady == daemonSet.Status.DesiredNumberScheduled {
				log.Infof("✔ %s successfully rolled out to %s, ready replicas: %v", daemonSetName, clName, daemonSet.Status.NumberReady)
				cancel()
			} else {
				log.Infof("Still waiting for %s daemon set roll out for %s, ready replicas: %v", daemonSetName, clName, daemonSet.Status.NumberReady)
			}
		} else {
			log.Debugf("Still waiting for %s daemon set roll out %s ...", daemonSetName, clName)
		}
	}, 5*time.Second, deploymentContext.Done())
	err := deploymentContext.Err()
	if err != nil && err != context.Canceled {
		return errors.Wrapf(err, "Error waiting for %s daemon set roll out.", daemonSetName)
	}
	return nil
}
