package shared_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/component/shared"
)

var _ = Describe("APIServer", func() {
	Describe("#GetResourcesForEncryptionFromConfig", func() {
		It("should return nil when apiServerConfig is nil", func() {
			Expect(GetResourcesForEncryptionFromConfig(nil, nil)).To(BeNil())
		})

		It("should return the correct list of resources when apiServerConfig is not nil", func() {
			apiServerConfig := &gardencorev1beta1.KubeAPIServerConfig{
				EncryptionConfig: &gardencorev1beta1.EncryptionConfig{
					Resources: []string{"deployments.apps", "fancyresource.customoperator.io", "configmaps", "daemonsets.apps"},
				},
			}

			Expect(GetResourcesForEncryptionFromConfig(apiServerConfig, nil)).To(ConsistOf(
				"deployments.apps",
				"fancyresource.customoperator.io",
				"configmaps",
				"daemonsets.apps",
			))
		})
	})

	Describe("#GetResourcesForEncryption", func() {
		It("should return the correct list of resources when filter func is nil", func() {
			resources := []string{"deployments.apps", "fancyresource.customoperator.io", "configmaps", "daemonsets.apps"}

			Expect(GetResourcesForEncryption(resources, nil)).To(ConsistOf(
				"deployments.apps",
				"fancyresource.customoperator.io",
				"configmaps",
				"daemonsets.apps",
			))
		})

		It("should return the correct list of resources after filtering out the resources", func() {
			resources := []string{"deployments.apps", "fancyresource.customoperator.io", "configmaps", "daemonsets.apps"}

			filterAppsGroup := func(resource string) bool {
				return strings.HasSuffix(resource, ".apps")
			}

			Expect(GetResourcesForEncryption(resources, filterAppsGroup)).To(ConsistOf(
				"fancyresource.customoperator.io",
				"configmaps",
			))
		})
	})
})
