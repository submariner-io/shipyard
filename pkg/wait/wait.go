package wait

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/submariner-io/armada/pkg/defaults"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ForPodsRunning waits for pods to be running
func ForPodsRunning(clName string, c client.Client, namespace, selector string, replicas int) error {
	labelSelector, err := labels.Parse(selector)
	if err != nil {
		return err
	}

	fieldSelector, err := fields.ParseSelector("status.phase=Running")
	if err != nil {
		return err
	}

	ctx := context.Background()
	log.Infof("Waiting up to %v for pods running with label %q, namespace %q, replicas %v in cluster %q ...", defaults.WaitDurationResources, selector, namespace, replicas, clName)
	podsContext, cancel := context.WithTimeout(ctx, defaults.WaitDurationResources)
	wait.Until(func() {
		podList := &corev1.PodList{}
		err := c.List(context.TODO(), podList, &client.ListOptions{
			Namespace:     namespace,
			LabelSelector: labelSelector,
			FieldSelector: fieldSelector,
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

	err = podsContext.Err()
	if err != nil && err != context.Canceled {
		return errors.Wrap(err, "Error waiting for pods to be running.")
	}
	return nil
}

// ForDeploymentReady waits for deployment roll out
func ForDeploymentReady(clName string, c client.Client, namespace, deploymentName string) error {
	ctx := context.Background()
	log.Infof("Waiting up to %v for %q deployment roll out in cluster %q ...", defaults.WaitDurationResources, deploymentName, clName)
	deploymentContext, cancel := context.WithTimeout(ctx, defaults.WaitDurationResources)
	wait.Until(func() {
		deployment := &appsv1.Deployment{}
		err := c.Get(context.TODO(), types.NamespacedName{Name: deploymentName, Namespace: namespace}, deployment)
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
func ForDaemonSetReady(clName string, c client.Client, namespace, daemonSetName string) error {
	ctx := context.Background()
	log.Infof("Waiting up to %v for %q daemon set roll out in cluster %q ...", defaults.WaitDurationResources, daemonSetName, clName)
	deploymentContext, cancel := context.WithTimeout(ctx, defaults.WaitDurationResources)
	wait.Until(func() {
		daemonSet := &appsv1.DaemonSet{}
		err := c.Get(context.TODO(), types.NamespacedName{Name: daemonSetName, Namespace: namespace}, daemonSet)
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

func ForTasksComplete(timeout time.Duration, tasks ...func() error) error {
	var wg sync.WaitGroup
	wg.Add(len(tasks))

	failed := make(chan error, len(tasks))
	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()

	for _, t := range tasks {
		go func(task func() error) {
			defer wg.Done()
			err := task()
			if err != nil {
				failed <- err
			}
		}(t)
	}

	select {
	case err := <-failed:
		return err
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timed out after %v", timeout)
	}
}
