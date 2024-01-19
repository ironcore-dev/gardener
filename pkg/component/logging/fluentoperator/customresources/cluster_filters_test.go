// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package customresources_test

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/filter"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/component/logging/fluentoperator/customresources"
)

var _ = Describe("Logging", func() {
	Describe("#GetClusterFilters", func() {
		var (
			configName = "some-name"
			labels     = map[string]string{"some-key": "some-value"}
		)

		It("should return the expected ClusterFilter custom resources", func() {
			fluentBitClusterFilters := GetClusterFilters(configName, labels)

			Expect(fluentBitClusterFilters).To(Equal(
				[]*fluentbitv1alpha2.ClusterFilter{
					{
						ObjectMeta: metav1.ObjectMeta{
							// This filter will be the second one of fluent-bit because the operator orders them by name
							Name:   "02-containerd",
							Labels: labels,
						},
						Spec: fluentbitv1alpha2.FilterSpec{
							Match: "kubernetes.*",
							FilterItems: []fluentbitv1alpha2.FilterItem{
								{
									Parser: &fluentbitv1alpha2filter.Parser{
										KeyName:     "log",
										Parser:      "containerd-parser",
										ReserveData: ptr.To(true),
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							// This filter will be the third one of fluent-bit because the operator orders them by name
							Name:   "03-add-tag-to-record",
							Labels: labels,
						},
						Spec: fluentbitv1alpha2.FilterSpec{
							Match: "kubernetes.*",
							FilterItems: []fluentbitv1alpha2.FilterItem{
								{
									Lua: &fluentbitv1alpha2filter.Lua{
										Script: corev1.ConfigMapKeySelector{
											Key: "add_tag_to_record.lua",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: configName,
											},
										},
										Call: "add_tag_to_record",
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							// This filter will be the last one of fluent-bit because the operator orders them by name
							Name:   "zz-modify-severity",
							Labels: labels,
						},
						Spec: fluentbitv1alpha2.FilterSpec{
							Match: "kubernetes.*",
							FilterItems: []fluentbitv1alpha2.FilterItem{
								{
									Lua: &fluentbitv1alpha2filter.Lua{
										Script: corev1.ConfigMapKeySelector{
											Key: "modify_severity.lua",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: configName,
											},
										},
										Call: "cb_modify",
									},
								},
							},
						},
					},
				}))
		})
	})
})
