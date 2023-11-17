// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helper_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/component/etcd"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot/helper"
	"github.com/gardener/gardener/pkg/operation/shoot"
)

var _ = Describe("ShouldPrepareShootForMigration", func() {
	var shoot *gardencorev1beta1.Shoot

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: pointer.String("seed"),
			},
			Status: gardencorev1beta1.ShootStatus{
				SeedName: pointer.String("seed"),
			},
		}
	})

	It("should return false if spec.seedName is not set", func() {
		shoot.Spec.SeedName = nil
		Expect(ShouldPrepareShootForMigration(shoot)).To(BeFalse())

		shoot.Status.SeedName = nil
		Expect(ShouldPrepareShootForMigration(shoot)).To(BeFalse())
	})

	It("should return false if status.seedName is not set", func() {
		shoot.Status.SeedName = nil
		Expect(ShouldPrepareShootForMigration(shoot)).To(BeFalse())
	})

	It("should return false if spec.seedName and status.seedName are equal", func() {
		Expect(ShouldPrepareShootForMigration(shoot)).To(BeFalse())
	})

	It("should return true if spec.seedName and status.seedName differ", func() {
		shoot.Spec.SeedName = pointer.String("other")
		Expect(ShouldPrepareShootForMigration(shoot)).To(BeTrue())
	})
})

var _ = Describe("ComputeOperationType", func() {
	var shoot *gardencorev1beta1.Shoot

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: pointer.String("seed"),
			},
			Status: gardencorev1beta1.ShootStatus{
				SeedName:      pointer.String("seed"),
				LastOperation: &gardencorev1beta1.LastOperation{},
			},
		}
	})

	It("should return Create if last operation is not set", func() {
		shoot.Status.LastOperation = nil
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeCreate))
	})

	It("should return Create if last operation is Create Error", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeCreate
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeCreate))
	})

	It("should return Reconcile if last operation is Create Succeeded", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeCreate
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeReconcile))
	})

	It("should return Reconcile if last operation is Restore Succeeded", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeRestore
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeReconcile))
	})

	It("should return Reconcile if last operation is Reconcile Succeeded", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeReconcile))
	})

	It("should return Reconcile if last operation is Reconcile Error", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeReconcile))
	})

	It("should return Reconcile if last operation is Reconcile Aborted", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateAborted
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeReconcile))
	})

	It("should return Migrate if spec.seedName and status.seedName differ", func() {
		shoot.Spec.SeedName = pointer.String("other")
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeMigrate))
	})

	It("should return Migrate if last operation is Migrate Error", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeMigrate
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeMigrate))
	})

	It("should return Restore if last operation is Migrate Succeeded", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeMigrate
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeRestore))
	})

	It("should return Restore if last operation is Migrate Aborted", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeMigrate
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateAborted
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeRestore))
	})

	It("should return Restore if last operation is Restore Error", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeRestore
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeRestore))
	})

	It("should return Delete if deletionTimestamp is set", func() {
		shoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeDelete))
	})

	It("should return Delete if last operation is Delete Error", func() {
		shoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeDelete
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeDelete))
	})
})

var _ = Describe("GetEtcdDeployTimeout", func() {
	var (
		s              *shoot.Shoot
		defaultTimeout time.Duration
	)

	BeforeEach(func() {
		s = &shoot.Shoot{}
		s.SetInfo(&gardencorev1beta1.Shoot{})
		defaultTimeout = 30 * time.Second
	})

	It("shoot is not marked to have HA control plane", func() {
		Expect(GetEtcdDeployTimeout(s, defaultTimeout)).To(Equal(defaultTimeout))
	})

	It("shoot spec has empty ControlPlane", func() {
		s.GetInfo().Spec.ControlPlane = &gardencorev1beta1.ControlPlane{}
		Expect(GetEtcdDeployTimeout(s, defaultTimeout)).To(Equal(defaultTimeout))
	})

	It("shoot is marked as multi-zonal", func() {
		s.GetInfo().Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
			HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeNode}},
		}
		Expect(GetEtcdDeployTimeout(s, defaultTimeout)).To(Equal(etcd.DefaultTimeout))
	})
})

var _ = Describe("GetResourcesForEncryption", func() {
	var fakeDiscoveryClient *fakeDiscoveryWithServerPreferredResources

	BeforeEach(func() {
		fakeDiscoveryClient = &fakeDiscoveryWithServerPreferredResources{}
	})

	It("should return the correct GVK list", func() {
		config := &gardencorev1beta1.KubeAPIServerConfig{
			EncryptionConfig: &gardencorev1beta1.EncryptionConfig{
				Resources: []string{
					"crontabs.stable.example.com",
					"*.resources.gardener.cloud",
					"configmaps",
				},
			},
		}

		list, err := GetResourcesForEncryption(fakeDiscoveryClient, config)
		Expect(err).NotTo(HaveOccurred())
		Expect(list).To(ConsistOf(
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			schema.GroupVersionKind{Group: "stable.example.com", Version: "v1", Kind: "CronTab"},
			schema.GroupVersionKind{Group: "resources.gardener.cloud", Version: "v1alpha1", Kind: "ManagedResource"},
		))
	})

	It("should return the correct GVK list for wildcard matching all resources", func() {
		config := &gardencorev1beta1.KubeAPIServerConfig{
			EncryptionConfig: &gardencorev1beta1.EncryptionConfig{
				Resources: []string{"*.*"},
				ExcludedResources: []string{
					"statefulsets.apps",
					"crontabs.stable.example.com",
				},
			},
		}

		list, err := GetResourcesForEncryption(fakeDiscoveryClient, config)
		Expect(err).NotTo(HaveOccurred())
		Expect(list).To(ConsistOf(
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"},
			schema.GroupVersionKind{Group: "stable.example.com", Version: "v1", Kind: "CronBar"},
			schema.GroupVersionKind{Group: "resources.gardener.cloud", Version: "v1alpha1", Kind: "ManagedResource"},
			schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"},
			schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		))
	})

	It("should return the correct GVK list for wildcard matching core group", func() {
		config := &gardencorev1beta1.KubeAPIServerConfig{
			EncryptionConfig: &gardencorev1beta1.EncryptionConfig{
				Resources:         []string{"*."},
				ExcludedResources: []string{"services"},
			},
		}

		list, err := GetResourcesForEncryption(fakeDiscoveryClient, config)
		Expect(err).NotTo(HaveOccurred())
		Expect(list).To(ConsistOf(
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		))
	})

	It("should return the correct GVK list for wildcard matching apps group", func() {
		config := &gardencorev1beta1.KubeAPIServerConfig{
			EncryptionConfig: &gardencorev1beta1.EncryptionConfig{
				Resources:         []string{"*.apps"},
				ExcludedResources: []string{"deployments.apps"},
			},
		}

		list, err := GetResourcesForEncryption(fakeDiscoveryClient, config)
		Expect(err).NotTo(HaveOccurred())
		Expect(list).To(ConsistOf(
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
			schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"},
			schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
		))
	})

	It("should return the correct GVK list for mix of wildcards and resources", func() {
		config := &gardencorev1beta1.KubeAPIServerConfig{
			EncryptionConfig: &gardencorev1beta1.EncryptionConfig{
				Resources: []string{
					"*.stable.example.com",
					"*.resources.gardener.cloud",
					"configmaps",
				},
				ExcludedResources: []string{
					"managedresources.resources.gardener.cloud",
				},
			},
		}

		list, err := GetResourcesForEncryption(fakeDiscoveryClient, config)
		Expect(err).NotTo(HaveOccurred())
		Expect(list).To(ConsistOf(
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			schema.GroupVersionKind{Group: "stable.example.com", Version: "v1", Kind: "CronTab"},
			schema.GroupVersionKind{Group: "stable.example.com", Version: "v1", Kind: "CronBar"},
		))
	})

	It("should return the correct GVK list for wildcard in excluded resources", func() {
		config := &gardencorev1beta1.KubeAPIServerConfig{
			EncryptionConfig: &gardencorev1beta1.EncryptionConfig{
				Resources: []string{
					"*.*",
				},
				ExcludedResources: []string{
					"*.apps",
					"*.stable.example.com",
				},
			},
		}

		list, err := GetResourcesForEncryption(fakeDiscoveryClient, config)
		Expect(err).NotTo(HaveOccurred())
		Expect(list).To(ConsistOf(
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"},
			schema.GroupVersionKind{Group: "resources.gardener.cloud", Version: "v1alpha1", Kind: "ManagedResource"},
		))
	})
})

type fakeDiscoveryWithServerPreferredResources struct {
	*fakediscovery.FakeDiscovery
}

func (c *fakeDiscoveryWithServerPreferredResources) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "configmaps",
					Namespaced: true,
					Group:      corev1.SchemeGroupVersion.Group,
					Version:    corev1.SchemeGroupVersion.Version,
					Kind:       "ConfigMap",
					Verbs:      metav1.Verbs{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
				},
				{
					Name:       "services",
					Namespaced: true,
					Group:      corev1.SchemeGroupVersion.Group,
					Version:    corev1.SchemeGroupVersion.Version,
					Kind:       "Service",
					Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					ShortNames: []string{"svc"},
				},
			},
		},

		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{
					Name:         "daemonsets",
					SingularName: "daemonset",
					Namespaced:   true,
					Group:        "",
					Version:      "",
					Kind:         "DaemonSet",
					Verbs:        metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					ShortNames:   []string{"ds"},
				},
				{
					Name:         "deployments",
					SingularName: "deployment",
					Namespaced:   true,
					Group:        "",
					Version:      "",
					Kind:         "Deployment",
					Verbs:        metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					ShortNames:   []string{"deploy"},
				},
				{
					Name:         "statefulsets",
					SingularName: "statefulset",
					Namespaced:   true,
					Group:        "",
					Version:      "",
					Kind:         "StatefulSet",
					Verbs:        metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
					ShortNames:   []string{"sts"},
				},
			},
		},
		{
			GroupVersion: "resources.gardener.cloud/v1alpha1",
			APIResources: []metav1.APIResource{
				{
					Name:       "managedresources",
					Namespaced: true,
					Group:      "resources.gardener.cloud",
					Version:    "v1alpha1",
					Kind:       "ManagedResource",
					Verbs:      metav1.Verbs{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
				},
			},
		},
		{
			GroupVersion: "stable.example.com/v1",
			APIResources: []metav1.APIResource{
				{
					Name:         "crontabs",
					SingularName: "crontab",
					Namespaced:   true,
					Group:        "stable.example.com",
					Version:      "v1",
					Kind:         "CronTab",
					Verbs:        metav1.Verbs{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
				},
				{
					Name:         "cronbars",
					SingularName: "cronbar",
					Namespaced:   true,
					Group:        "stable.example.com",
					Version:      "v1",
					Kind:         "CronBar",
					Verbs:        metav1.Verbs{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"},
				},
			},
		},
	}, nil
}
