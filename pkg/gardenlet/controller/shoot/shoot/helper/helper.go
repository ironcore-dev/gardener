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

package helper

import (
	"fmt"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// ShouldPrepareShootForMigration determines whether the controller should prepare the shoot control plane for migration
// to another seed.
func ShouldPrepareShootForMigration(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Status.SeedName != nil && shoot.Spec.SeedName != nil && *shoot.Spec.SeedName != *shoot.Status.SeedName
}

// ComputeOperationType determines which operation should be executed when acting on the given shoot.
func ComputeOperationType(shoot *gardencorev1beta1.Shoot) gardencorev1beta1.LastOperationType {
	if ShouldPrepareShootForMigration(shoot) {
		return gardencorev1beta1.LastOperationTypeMigrate
	}

	lastOperation := shoot.Status.LastOperation
	if lastOperation != nil && lastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate &&
		(lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded || lastOperation.State == gardencorev1beta1.LastOperationStateAborted) {
		return gardencorev1beta1.LastOperationTypeRestore
	}

	return v1beta1helper.ComputeOperationType(shoot.ObjectMeta, shoot.Status.LastOperation)
}

// GetEtcdDeployTimeout returns the timeout for the etcd deployment task of the reconcile flow.
func GetEtcdDeployTimeout(shoot *shoot.Shoot, defaultDuration time.Duration) time.Duration {
	timeout := defaultDuration
	if v1beta1helper.IsHAControlPlaneConfigured(shoot.GetInfo()) {
		timeout = etcd.DefaultTimeout
	}
	return timeout
}

// IsSeedReadyForMigration checks if the seed can be used as a target seed for migrating a shoot control plane.
// If the seed is ready, it returns nil. Otherwise, it returns an error with a description.
func IsSeedReadyForMigration(seed *gardencorev1beta1.Seed, identity *gardencorev1beta1.Gardener) error {
	if seed.DeletionTimestamp != nil {
		return fmt.Errorf("seed is marked to be deleted")
	}
	return health.CheckSeedForMigration(seed, identity)
}

// GetResourcesForEncryption returns a list of schema.GroupVersionKind for all the resources that needs to be encrypted. Secrets are
// returned by default and additional resources if specified in the encryptionConfig are returned.
func GetResourcesForEncryption(discoveryClient discovery.DiscoveryInterface, kubeAPIServer *gardencorev1beta1.KubeAPIServerConfig) ([]schema.GroupVersionKind, error) {
	var (
		encryptedGVKS           = sets.New(corev1.SchemeGroupVersion.WithKind("Secret"))
		coreResourcesToEncrypt  = sets.New[string]()
		groupResourcesToEncrypt = map[string]sets.Set[string]{}
	)

	if kubeAPIServer == nil || kubeAPIServer.EncryptionConfig == nil {
		return encryptedGVKS.UnsortedList(), nil
	}

	for _, resource := range kubeAPIServer.EncryptionConfig.Resources {
		var (
			split    = strings.Split(resource, ".")
			group    = strings.Join(split[1:], ".")
			resource = split[0]
		)

		if len(split) == 1 {
			coreResourcesToEncrypt.Insert(resource)
			continue
		}

		if _, ok := groupResourcesToEncrypt[group]; !ok {
			groupResourcesToEncrypt[group] = sets.New[string]()
		}

		groupResourcesToEncrypt[group].Insert(resource)
	}

	resourceLists, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return encryptedGVKS.UnsortedList(), fmt.Errorf("error discovering server preferred resources: %w", err)
	}

	for _, list := range resourceLists {
		if len(list.APIResources) == 0 {
			continue
		}

		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			return encryptedGVKS.UnsortedList(), fmt.Errorf("error parsing groupVersion: %w", err)
		}

		for _, apiResource := range list.APIResources {
			// If the resource doesn't support get, list and patch, we cannot list and rewrite it
			if !slices.Contains(apiResource.Verbs, "get") ||
				!slices.Contains(apiResource.Verbs, "list") ||
				!slices.Contains(apiResource.Verbs, "patch") {
				continue
			}

			var (
				group                   = gv.Group
				version                 = gv.Version
				resourceNeedsEncryption = false
			)

			if apiResource.Group != "" {
				group = apiResource.Group
			}
			if apiResource.Version != "" {
				version = apiResource.Version
			}

			if group == "" && coreResourcesToEncrypt.Has(apiResource.Name) {
				resourceNeedsEncryption = true
			}

			if resources, ok := groupResourcesToEncrypt[group]; ok && resources.Has(apiResource.Name) {
				resourceNeedsEncryption = true
			}

			if resourceNeedsEncryption {
				encryptedGVKS.Insert(schema.GroupVersionKind{Group: group, Version: version, Kind: apiResource.Kind})
			}
		}
	}

	return encryptedGVKS.UnsortedList(), nil
}
