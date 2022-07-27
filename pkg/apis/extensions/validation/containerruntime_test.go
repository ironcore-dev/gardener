// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation_test

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/validation"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("ContainerRuntime validation tests", func() {
	var cr *extensionsv1alpha1.ContainerRuntime

	BeforeEach(func() {
		cr = &extensionsv1alpha1.ContainerRuntime{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cr",
				Namespace: "shoot-namespace-seed",
			},
			Spec: extensionsv1alpha1.ContainerRuntimeSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: "provider",
				},
				BinaryPath: "/test/path",
				WorkerPool: extensionsv1alpha1.ContainerRuntimeWorkerPool{
					Name: "test-workerPool",
				},
			},
		}
	})

	Describe("#ValidContainerRuntime", func() {
		It("should forbid empty ContainerRuntime resources", func() {
			errorList := ValidateContainerRuntime(&extensionsv1alpha1.ContainerRuntime{})

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.namespace"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.binaryPath"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.workerPool.name"),
			}))))
		})

		It("should allow valid ContainerRuntime resources", func() {
			errorList := ValidateContainerRuntime(cr)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidContainerRuntimeUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()
			cr.DeletionTimestamp = &now
			newContainerRuntime := prepareContainerRuntimeForUpdate(cr)
			newContainerRuntime.Spec.BinaryPath = "changed-binaryPath"

			errorList := ValidateContainerRuntimeUpdate(newContainerRuntime, cr)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec"),
			}))))
		})

		It("should prevent updating the type and workerPool ", func() {
			newContainerRuntime := prepareContainerRuntimeForUpdate(cr)
			newContainerRuntime.Spec.Type = "changed-type"
			newContainerRuntime.Spec.WorkerPool.Name = "changed-workerpool-name"

			errorList := ValidateContainerRuntimeUpdate(newContainerRuntime, cr)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.workerPool.name"),
			}))))
		})

		It("should allow updating the binaryPath", func() {
			newContainerRuntime := prepareContainerRuntimeForUpdate(cr)
			newContainerRuntime.Spec.BinaryPath = "changed-binary-path"

			errorList := ValidateContainerRuntimeUpdate(newContainerRuntime, cr)
			Expect(errorList).To(BeEmpty())
		})
	})
})

func prepareContainerRuntimeForUpdate(obj *extensionsv1alpha1.ContainerRuntime) *extensionsv1alpha1.ContainerRuntime {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
