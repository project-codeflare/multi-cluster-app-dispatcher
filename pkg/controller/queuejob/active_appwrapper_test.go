package queuejob_test

import (
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejob"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = ginkgo.Describe("Active App Wrapper Tests", func() {
	var activeAppWrapper *queuejob.ActiveAppWrapper
	ginkgo.When("Checking if is active app wrapper", func() {
		ginkgo.BeforeEach(func() {
			activeAppWrapper = queuejob.NewActiveAppWrapper()
		})
		ginkgo.It("should return 'true' for a nil active app wrapper", func() {
			gomega.Expect(activeAppWrapper.IsActiveAppWrapper("an-appwrapper-name", "unit-test-namespace"),
				gomega.BeTrue())
		})
		ginkgo.It("should return 'true' for a the same app wrapper name and namespace", func() {
			activeAppWrapper.AtomicSet(&arbv1.AppWrapper{
				ObjectMeta: v1.ObjectMeta{
					Name:      "an-appwrapper-name",
					Namespace: "unit-test-namespace",
				},
			})
			gomega.Expect(activeAppWrapper.IsActiveAppWrapper("an-appwrapper-name", "unit-test-namespace"),
				gomega.BeTrue())
		})
		ginkgo.It("should return 'false' for a the same app wrapper name and namespace", func() {
			activeAppWrapper.AtomicSet(&arbv1.AppWrapper{
				ObjectMeta: v1.ObjectMeta{
					Name:      "an-appwrapper-name",
					Namespace: "unit-test-namespace",
				},
			})
			gomega.Expect(activeAppWrapper.IsActiveAppWrapper("another-appwrapper-name", "other-unit-test-namespace"),
				gomega.BeTrue())
		})
	})

})
