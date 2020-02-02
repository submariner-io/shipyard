package wait_test

import (
	"errors"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/armada/pkg/wait"

	"sync/atomic"
)

func TestWait(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wait test suite")
}

var _ = Describe("Wait tests", func() {
	Context("ForTasksComplete", testForTasksComplete)
})

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
