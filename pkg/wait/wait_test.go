package wait_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/armada/pkg/defaults"
	"github.com/submariner-io/armada/pkg/wait"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestWait(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wait test suite")
}

const namespace = "test-namespace"

var origWaitDurationResources time.Duration
var origWaitRetryPeriod time.Duration

var _ = BeforeSuite(func() {
	origWaitDurationResources = defaults.WaitDurationResources
	origWaitRetryPeriod = defaults.WaitRetryPeriod
})

var _ = AfterSuite(func() {
	defaults.WaitDurationResources = origWaitDurationResources
	defaults.WaitRetryPeriod = origWaitRetryPeriod
})

type testClient struct {
	client.Client
	initialError error
}

func (c *testClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	err := c.initialError
	if err != nil {
		c.initialError = nil
		return err
	}

	return c.Client.Get(ctx, key, obj)
}

func (c *testClient) List(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
	err := c.initialError
	if err != nil {
		c.initialError = nil
		return err
	}

	return c.Client.List(ctx, list, opts...)
}

var _ = Describe("Wait tests", func() {
	BeforeEach(func() {
		defaults.WaitDurationResources = 10 * time.Second
		defaults.WaitRetryPeriod = 200 * time.Millisecond
	})

	Context("ForTasksComplete", testForTasksComplete)
	Context("ForDeploymentReady", testForDeploymentReady)
	Context("ForDaemonSetReady", testForDaemonSetReady)
	Context("ForPodsRunning", testForPodsRunning)
})

func testForPodsRunning() {
	var (
		initialPods   []*corev1.Pod
		client        *testClient
		waitCh        chan error
		listError     error
		numReplicas   int
		labelSelector string
	)

	BeforeEach(func() {
		listError = nil
		initialPods = nil
		numReplicas = 1
		labelSelector = "app=test"
	})

	JustBeforeEach(func() {
		var initObjs []runtime.Object
		for _, p := range initialPods {
			initObjs = append(initObjs, p)
		}

		client = &testClient{Client: fake.NewFakeClientWithScheme(scheme.Scheme, initObjs...), initialError: listError}

		waitCh = runAsync(func() error {
			return wait.ForPodsRunning("east", client, namespace, labelSelector, numReplicas)
		})
	})

	When("the Pods initially exist", func() {
		BeforeEach(func() {
			numReplicas = 2
			initialPods = append(initialPods, newPod("test-Pod1", corev1.PodPending), newPod("test-Pod2", corev1.PodPending))
		})

		Context("and are initially running", func() {
			BeforeEach(func() {
				for _, p := range initialPods {
					p.Status.Phase = corev1.PodRunning
				}
			})

			It("should return success", func() {
				Eventually(waitCh, defaults.WaitDurationResources).Should(BeClosed())
			})
		})

		Context("and are eventually running", func() {
			It("should return success", func() {
				go func() {
					time.Sleep(defaults.WaitRetryPeriod * 2)
					for _, p := range initialPods {
						p.Status.Phase = corev1.PodRunning
						_ = client.Update(context.TODO(), p)
					}
				}()

				Eventually(waitCh, defaults.WaitDurationResources).Should(BeClosed())
			})
		})
	})

	When("the Pod does not initially exist but is eventually created and running", func() {
		It("should return success", func() {
			go func() {
				time.Sleep(defaults.WaitRetryPeriod * 2)
				_ = client.Create(context.TODO(), newPod("test-Pod1", corev1.PodRunning))
			}()

			Eventually(waitCh, defaults.WaitDurationResources).Should(BeClosed())
		})
	})

	When("retrieval of the running Pod list initially fails but eventually succeeds", func() {
		BeforeEach(func() {
			initialPods = append(initialPods, newPod("test-Pod", corev1.PodRunning))
			listError = errors.New("mock error")
		})

		It("should return success", func() {
			Eventually(waitCh, defaults.WaitDurationResources).Should(BeClosed())
		})
	})

	When("the Pod never becomes running", func() {
		BeforeEach(func() {
			defaults.WaitDurationResources = 300 * time.Millisecond
			defaults.WaitRetryPeriod = 50 * time.Millisecond
			initialPods = append(initialPods, newPod("test-Pod", corev1.PodPending))
		})

		It("should timeout and return an error", func() {
			Eventually(waitCh, defaults.WaitDurationResources*2).Should(Receive())
		})
	})

	When("an invalid label selector is provided", func() {
		BeforeEach(func() {
			labelSelector = "a bogus selector"
			initialPods = append(initialPods, newPod("test-Pod", corev1.PodRunning))
		})

		It("should return an error", func() {
			Eventually(waitCh, defaults.WaitDurationResources*2).Should(Receive())
		})
	})
}

func testForDeploymentReady() {
	testForResourceReady("Deployment", func() runtime.Object {
		replicas := int32(1)
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "test-Deployment",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
			},
		}
	}, func(obj runtime.Object) {
		obj.(*appsv1.Deployment).Status.ReadyReplicas = 1
	}, wait.ForDeploymentReady)
}

func testForDaemonSetReady() {
	testForResourceReady("DaemonSet", func() runtime.Object {
		return &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "test-DaemonSet",
			},
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 1,
			},
		}
	}, func(obj runtime.Object) {
		obj.(*appsv1.DaemonSet).Status.NumberReady = 1
	}, wait.ForDaemonSetReady)
}

func testForResourceReady(resourceType string, newResource func() runtime.Object, updateToReady func(runtime.Object),
	waitForReady func(string, client.Client, string, string) error) {

	var (
		resource        runtime.Object
		initialResource runtime.Object
		client          *testClient
		waitCh          chan error
		initialError    error
	)

	BeforeEach(func() {
		resource = newResource()

		initialError = nil
		initialResource = nil
	})

	JustBeforeEach(func() {
		var initObjs []runtime.Object
		if initialResource != nil {
			initObjs = append(initObjs, initialResource)
		}

		client = &testClient{Client: fake.NewFakeClientWithScheme(scheme.Scheme, initObjs...), initialError: initialError}

		metadata, err := meta.Accessor(resource)
		Expect(err).To(Succeed())

		waitCh = runAsync(func() error {
			return waitForReady("east", client, metadata.GetNamespace(), metadata.GetName())
		})
	})

	When(fmt.Sprintf("the %s initially exists", resourceType), func() {
		BeforeEach(func() {
			initialResource = resource
		})

		Context("and is initially ready", func() {
			BeforeEach(func() {
				updateToReady(resource)
			})

			It("should return success", func() {
				Eventually(waitCh, defaults.WaitDurationResources).Should(BeClosed())
			})
		})

		Context("and is eventually ready", func() {
			It("should return success", func() {
				go func() {
					time.Sleep(defaults.WaitRetryPeriod * 2)
					updateToReady(resource)
					_ = client.Update(context.TODO(), resource)
				}()

				Eventually(waitCh, defaults.WaitDurationResources).Should(BeClosed())
			})
		})
	})

	When(fmt.Sprintf("the %s does not initially exist but is eventually created and ready", resourceType), func() {
		It("should return success", func() {
			go func() {
				time.Sleep(defaults.WaitRetryPeriod * 2)
				updateToReady(resource)
				_ = client.Create(context.TODO(), resource)
			}()

			Eventually(waitCh, defaults.WaitDurationResources).Should(BeClosed())
		})
	})

	When(fmt.Sprintf("retrieval of the %s initially fails but eventually succeeds and is ready", resourceType), func() {
		BeforeEach(func() {
			initialResource = resource
			initialError = errors.New("mock error")
			updateToReady(resource)
		})

		It("should return success", func() {
			Eventually(waitCh, defaults.WaitDurationResources).Should(BeClosed())
		})
	})

	When(fmt.Sprintf("the %s never becomes ready", resourceType), func() {
		BeforeEach(func() {
			defaults.WaitDurationResources = 300 * time.Millisecond
			defaults.WaitRetryPeriod = 50 * time.Millisecond
		})

		It("should timeout and return an error", func() {
			Eventually(waitCh, defaults.WaitDurationResources*2).Should(Receive())
		})
	})
}

func runAsync(f func() error) chan error {
	waitCh := make(chan error)
	go func() {
		err := f()
		if err == nil {
			close(waitCh)
		} else {
			waitCh <- err
		}
	}()

	return waitCh
}

func testForTasksComplete() {
	When("tasks successfully complete", func() {
		It("should return success", func() {
			tasks := []func() error{}
			numTasks := 5
			var count uint32
			for i := 1; i <= numTasks; i++ {
				tasks = append(tasks, func() error {
					atomic.AddUint32(&count, 1)
					return nil
				})
			}

			err := wait.ForTasksComplete(10*time.Second, tasks...)
			Expect(err).To(Succeed())
			Expect(int(count)).To(Equal(numTasks))
		})
	})

	When("a task fails", func() {
		errMsg := "task failed"
		It("should return the error", func() {
			tasks := []func() error{
				func() error {
					return nil
				},
				func() error {
					return errors.New(errMsg)
				},
			}

			err := wait.ForTasksComplete(10*time.Second, tasks...)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(errMsg))
		})
	})

	When("tasks don't complete in time", func() {
		It("should timeout and return an error", func() {
			tasks := []func() error{
				func() error {
					time.Sleep(5 * time.Second)
					return nil
				},
			}

			err := wait.ForTasksComplete(200*time.Millisecond, tasks...)
			Expect(err).To(HaveOccurred())
		})
	})
}

func newPod(name string, phase corev1.PodPhase) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    map[string]string{"app": "test"},
		},
		Status: corev1.PodStatus{
			Phase: phase,
		},
	}
}
