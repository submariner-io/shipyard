package wait

import (
	"context"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/submariner-io/armada/pkg/defaults"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// ForPodsRunning waits for pods to be running
func ForPodsRunning(clName string, c kubernetes.Interface, namespace, selector string, replicas int) error {
	ctx := context.Background()
	log.Infof("Waiting up to %v for pods running with label %q, namespace %q, replicas %v in cluster %q ...", defaults.WaitDurationResources, selector, namespace, replicas, clName)
	podsContext, cancel := context.WithTimeout(ctx, defaults.WaitDurationResources)
	wait.Until(func() {
		podList, err := c.CoreV1().Pods(namespace).List(metav1.ListOptions{
			LabelSelector: selector,
			FieldSelector: "status.phase=Running",
		})

		if err != nil {
			log.Errorf("Error listing pods for label %q, namespace %q in cluster %q: %v", selector, namespace, clName, err)
		} else if len(podList.Items) == replicas {
			log.Infof("✔ All pods with label %q in namespace %q are running in cluster %q.", selector, namespace, clName)
			cancel()
		} else {
			log.Infof("Still waiting for pods with label %q, namespace %q, replicas %v in cluster: %q.", selector, namespace, replicas, clName)
		}
	}, 2*time.Second, podsContext.Done())

	err := podsContext.Err()
	if err != nil && err != context.Canceled {
		return errors.Wrap(err, "Error waiting for pods to be running.")
	}
	return nil
}

// ForDeploymentReady waits for deployment roll out
func ForDeploymentReady(clName string, c kubernetes.Interface, namespace, deploymentName string) error {
	ctx := context.Background()
	log.Infof("Waiting up to %v for %q deployment roll out in cluster %q ...", defaults.WaitDurationResources, deploymentName, clName)
	deploymentContext, cancel := context.WithTimeout(ctx, defaults.WaitDurationResources)
	wait.Until(func() {
		deployment, err := c.AppsV1().Deployments(namespace).Get(deploymentName, metav1.GetOptions{})
		if err == nil {
			if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
				log.Infof("✔ %q successfully deployed in cluster %q with %v replicas ready", deploymentName, clName, deployment.Status.ReadyReplicas)
				cancel()
			} else {
				log.Infof("Still waiting for %q deployment in cluster %q, %v out of %v replicas ready", deploymentName, clName, deployment.Status.ReadyReplicas, *deployment.Spec.Replicas)
			}
		} else if apiErrors.IsNotFound(err) {
			log.Infof("Still waiting for %q deployment roll out in cluster %q", deploymentName, clName)
		} else {
			log.Errorf("Error getting deployment %q in cluster %q: %v", deploymentName, clName, err)
		}
	}, 2*time.Second, deploymentContext.Done())
	err := deploymentContext.Err()
	if err != nil && err != context.Canceled {
		return errors.Wrapf(err, "Error waiting for %q deployment roll out.", deploymentName)
	}
	return nil
}

// ForDaemonSetReady waits for daemon set roll out
func ForDaemonSetReady(clName string, c kubernetes.Interface, namespace, daemonSetName string) error {
	ctx := context.Background()
	log.Infof("Waiting up to %v for %q daemon set roll out in cluster %q ...", defaults.WaitDurationResources, daemonSetName, clName)
	deploymentContext, cancel := context.WithTimeout(ctx, defaults.WaitDurationResources)
	wait.Until(func() {
		daemonSet, err := c.AppsV1().DaemonSets(namespace).Get(daemonSetName, metav1.GetOptions{})
		if err == nil {
			if daemonSet.Status.NumberReady == daemonSet.Status.DesiredNumberScheduled {
				log.Infof("✔ Daemon set %q successfully rolled out %v replicas in cluster %q", daemonSetName, clName, daemonSet.Status.NumberReady)
				cancel()
			} else {
				log.Infof("Still waiting for daemon set %q roll out in cluster %q, %v out of %v replicas ready", daemonSetName, clName, daemonSet.Status.NumberReady, daemonSet.Status.DesiredNumberScheduled)
			}
		} else if apiErrors.IsNotFound(err) {
			log.Debugf("Still waiting for daemon set %q roll out in cluster %q", daemonSetName, clName)
		} else {
			log.Errorf("Error getting daemon set %q in cluster %q: %v", daemonSetName, clName, err)
		}
	}, 2*time.Second, deploymentContext.Done())
	err := deploymentContext.Err()
	if err != nil && err != context.Canceled {
		return errors.Wrapf(err, "Error waiting for %s daemon set roll out.", daemonSetName)
	}
	return nil
}
