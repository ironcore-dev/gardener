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

package gardenerscheduler_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	coordinationv1beta1 "k8s.io/api/coordination/v1beta1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/gardenerscheduler"
	componenttest "github.com/gardener/gardener/pkg/component/test"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	schedulerv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("GardenerScheduler", func() {
	var (
		ctx context.Context

		managedResourceNameRuntime = "gardener-scheduler-runtime"
		managedResourceNameVirtual = "gardener-scheduler-virtual"
		namespace                  = "some-namespace"

		fakeClient        client.Client
		fakeSecretManager secretsmanager.Interface
		deployer          component.DeployWaiter
		values            Values

		fakeOps *retryfake.Ops

		managedResourceRuntime       *resourcesv1alpha1.ManagedResource
		managedResourceVirtual       *resourcesv1alpha1.ManagedResource
		managedResourceSecretRuntime *corev1.Secret
		managedResourceSecretVirtual *corev1.Secret

		podDisruptionBudget *policyv1.PodDisruptionBudget
		serviceRuntime      *corev1.Service
		vpa                 *vpaautoscalingv1.VerticalPodAutoscaler

		clusterRole        *rbacv1.ClusterRole
		clusterRoleBinding *rbacv1.ClusterRoleBinding
	)

	BeforeEach(func() {
		ctx = context.TODO()

		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(fakeClient, namespace)
		values = Values{}

		fakeOps = &retryfake.Ops{MaxAttempts: 2}
		DeferCleanup(test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		))

		managedResourceRuntime = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceNameRuntime,
				Namespace: namespace,
			},
		}
		managedResourceVirtual = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceNameVirtual,
				Namespace: namespace,
			},
		}
		managedResourceSecretRuntime = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceRuntime.Name,
				Namespace: namespace,
			},
		}
		managedResourceSecretVirtual = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceVirtual.Name,
				Namespace: namespace,
			},
		}
		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-scheduler",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "scheduler",
				},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: gardenerutils.IntStrPtrFromInt32(1),
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "gardener",
					"role": "scheduler",
				}},
			},
		}
		serviceRuntime = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-scheduler",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "scheduler",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Selector: map[string]string{
					"app":  "gardener",
					"role": "scheduler",
				},
				Ports: []corev1.ServicePort{{
					Name:       "metrics",
					Port:       19251,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt32(19251),
				}},
			},
		}
		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-scheduler-vpa",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "scheduler",
				},
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "gardener-scheduler",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: "*",
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("25Mi"),
							},
						},
					},
				},
			},
		}
		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:scheduler",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "scheduler",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "patch", "update"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
					Verbs:     []string{"create", "delete", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{gardencorev1beta1.GroupName},
					Resources: []string{
						"cloudprofiles",
						"seeds",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{gardencorev1beta1.GroupName},
					Resources: []string{
						"shoots",
						"shoots/status",
					},
					Verbs: []string{"get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{gardencorev1beta1.GroupName},
					Resources: []string{
						"shoots/binding",
					},
					Verbs: []string{"update"},
				},
				{
					APIGroups: []string{coordinationv1beta1.GroupName},
					Resources: []string{
						"leases",
					},
					Verbs: []string{"create"},
				},
				{
					APIGroups: []string{coordinationv1beta1.GroupName},
					Resources: []string{
						"leases",
					},
					ResourceNames: []string{
						"gardener-scheduler-leader-election",
					},
					Verbs: []string{"get", "watch", "update"},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:scheduler",
				Labels: map[string]string{
					"app":  "gardener",
					"role": "scheduler",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:scheduler",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "gardener-scheduler",
				Namespace: "kube-system",
			}},
		}

		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
	})

	JustBeforeEach(func() {
		deployer = New(fakeClient, namespace, fakeSecretManager, values)
	})

	Describe("#Deploy", func() {
		Context("resources generation", func() {
			BeforeEach(func() {
				// test with typical values
				values = Values{
					LogLevel: "info",
				}

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(BeNotFoundError())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())
			})

			It("should successfully deploy all resources", func() {
				Expect(deployer.Deploy(ctx)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(Succeed())
				expectedRuntimeMr := &resourcesv1alpha1.ManagedResource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ManagedResource",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResourceRuntime.Name,
						Namespace:       managedResourceRuntime.Namespace,
						ResourceVersion: "2",
						Generation:      1,
						Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class:       pointer.String("seed"),
						SecretRefs:  []corev1.LocalObjectReference{{Name: managedResourceRuntime.Spec.SecretRefs[0].Name}},
						KeepObjects: pointer.Bool(false),
					},
					Status: healthyManagedResourceStatus,
				}
				utilruntime.Must(references.InjectAnnotations(expectedRuntimeMr))
				Expect(managedResourceRuntime).To(Equal(expectedRuntimeMr))

				managedResourceSecretRuntime.Name = managedResourceRuntime.Spec.SecretRefs[0].Name
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(Succeed())
				expectedVirtualMr := &resourcesv1alpha1.ManagedResource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ManagedResource",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResourceVirtual.Name,
						Namespace:       managedResourceVirtual.Namespace,
						ResourceVersion: "2",
						Generation:      1,
						Labels:          map[string]string{"origin": "gardener"},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						SecretRefs:   []corev1.LocalObjectReference{{Name: managedResourceVirtual.Spec.SecretRefs[0].Name}},
						KeepObjects:  pointer.Bool(false),
					},
					Status: healthyManagedResourceStatus,
				}
				utilruntime.Must(references.InjectAnnotations(expectedVirtualMr))
				Expect(managedResourceVirtual).To(Equal(expectedVirtualMr))

				managedResourceSecretVirtual.Name = expectedVirtualMr.Spec.SecretRefs[0].Name
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(Succeed())

				Expect(managedResourceSecretRuntime.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecretRuntime.Data).To(HaveLen(5))
				Expect(string(managedResourceSecretRuntime.Data["configmap__some-namespace__gardener-scheduler-config-3cf6616e.yaml"])).To(Equal(configMap(namespace, values)))
				Expect(string(managedResourceSecretRuntime.Data["poddisruptionbudget__some-namespace__gardener-scheduler.yaml"])).To(Equal(componenttest.Serialize(podDisruptionBudget)))
				Expect(string(managedResourceSecretRuntime.Data["service__some-namespace__gardener-scheduler.yaml"])).To(Equal(componenttest.Serialize(serviceRuntime)))
				Expect(string(managedResourceSecretRuntime.Data["verticalpodautoscaler__some-namespace__gardener-scheduler-vpa.yaml"])).To(Equal(componenttest.Serialize(vpa)))
				Expect(string(managedResourceSecretRuntime.Data["deployment__some-namespace__gardener-scheduler.yaml"])).To(Equal(deployment(namespace, "gardener-scheduler-config-3cf6616e", values)))
				Expect(managedResourceSecretRuntime.Immutable).To(Equal(pointer.Bool(true)))
				Expect(managedResourceSecretRuntime.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

				Expect(managedResourceSecretVirtual.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecretVirtual.Data).To(HaveLen(2))
				Expect(string(managedResourceSecretVirtual.Data["clusterrole____gardener.cloud_system_scheduler.yaml"])).To(Equal(componenttest.Serialize(clusterRole)))
				Expect(string(managedResourceSecretVirtual.Data["clusterrolebinding____gardener.cloud_system_scheduler.yaml"])).To(Equal(componenttest.Serialize(clusterRoleBinding)))
				Expect(managedResourceSecretVirtual.Immutable).To(Equal(pointer.Bool(true)))
				Expect(managedResourceSecretVirtual.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
			})
		})

		Context("secrets", func() {
			It("should successfully deploy the access secret for the virtual garden", func() {
				accessSecret := &corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Secret",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shoot-access-gardener-scheduler",
						Namespace: namespace,
						Labels: map[string]string{
							"resources.gardener.cloud/purpose": "token-requestor",
							"resources.gardener.cloud/class":   "shoot",
						},
						Annotations: map[string]string{
							"serviceaccount.resources.gardener.cloud/name":      "gardener-scheduler",
							"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
						},
					},
					Type: corev1.SecretTypeOpaque,
				}

				Expect(deployer.Deploy(ctx)).To(Succeed())

				actualShootAccessSecret := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(accessSecret), actualShootAccessSecret)).To(Succeed())
				accessSecret.ResourceVersion = "1"
				Expect(actualShootAccessSecret).To(Equal(accessSecret))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(fakeClient.Create(ctx, managedResourceRuntime)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceVirtual)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecretRuntime)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecretVirtual)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(Succeed())

			Expect(deployer.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		Describe("#Wait", func() {
			It("should fail because reading the runtime ManagedResource fails", func() {
				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the runtime and virtual ManagedResources are unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should fail because the runtime ManagedResource is unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should fail because the virtual ManagedResource is unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should succeed because the runtime and virtual ManagedResource are healthy and progressing", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(Succeed())
			})

			It("should succeed because the both ManagedResource are healthy and progressed", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the runtime managed resource deletion times out", func() {
				Expect(fakeClient.Create(ctx, managedResourceRuntime)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should fail when the wait for the virtual managed resource deletion times out", func() {
				Expect(fakeClient.Create(ctx, managedResourceVirtual)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when they are already removed", func() {
				Expect(deployer.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

var (
	healthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
		ObservedGeneration: 1,
		Conditions: []gardencorev1beta1.Condition{
			{
				Type:   resourcesv1alpha1.ResourcesApplied,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   resourcesv1alpha1.ResourcesHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
		},
	}
	unhealthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
		ObservedGeneration: 1,
		Conditions: []gardencorev1beta1.Condition{
			{
				Type:   resourcesv1alpha1.ResourcesApplied,
				Status: gardencorev1beta1.ConditionFalse,
			},
			{
				Type:   resourcesv1alpha1.ResourcesHealthy,
				Status: gardencorev1beta1.ConditionFalse,
			},
		},
	}
)

func configMap(namespace string, testValues Values) string {
	schedulerConfig := &schedulerv1alpha1.SchedulerConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "scheduler.config.gardener.cloud/v1alpha1",
			Kind:       "SchedulerConfiguration",
		},
		ClientConnection: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
			QPS:        100,
			Burst:      130,
			Kubeconfig: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
		},
		LeaderElection: &componentbaseconfigv1alpha1.LeaderElectionConfiguration{
			LeaderElect:       pointer.Bool(true),
			ResourceName:      "gardener-scheduler-leader-election",
			ResourceNamespace: "kube-system",
		},
		LogLevel:  testValues.LogLevel,
		LogFormat: "json",
		Server: schedulerv1alpha1.ServerConfiguration{
			HealthProbes: &schedulerv1alpha1.Server{
				Port: 10251,
			},
			Metrics: &schedulerv1alpha1.Server{
				Port: 19251,
			},
		},
		Schedulers: schedulerv1alpha1.SchedulerControllerConfiguration{
			Shoot: &schedulerv1alpha1.ShootSchedulerConfiguration{
				Strategy: "MinimalDistance",
			},
		},
		FeatureGates: testValues.FeatureGates,
	}

	data, err := json.Marshal(schedulerConfig)
	utilruntime.Must(err)
	data, err = yaml.JSONToYAML(data)
	utilruntime.Must(err)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{},
			Labels: map[string]string{
				"app":  "gardener",
				"role": "scheduler",
			},
			Name:      "gardener-scheduler-config",
			Namespace: namespace,
		},
		Data: map[string]string{
			"schedulerconfiguration.yaml": string(data),
		},
	}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	return componenttest.Serialize(configMap)
}

func deployment(namespace, configSecretName string, testValues Values) string {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardener-scheduler",
			Namespace: namespace,
			Labels: map[string]string{
				"app":  "gardener",
				"role": "scheduler",
				"high-availability-config.resources.gardener.cloud/type": "controller",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             pointer.Int32(1),
			RevisionHistoryLimit: pointer.Int32(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":  "gardener",
					"role": "scheduler",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: gardenerutils.MergeStringMaps(GetLabels(), map[string]string{
						"app":                              "gardener",
						"role":                             "scheduler",
						"networking.gardener.cloud/to-dns": "allowed",
						"networking.resources.gardener.cloud/to-virtual-garden-kube-apiserver-tcp-443": "allowed",
					}),
				},
				Spec: corev1.PodSpec{
					PriorityClassName:            "gardener-garden-system-200",
					AutomountServiceAccountToken: pointer.Bool(false),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: pointer.Bool(true),
						RunAsUser:    pointer.Int64(65532),
						RunAsGroup:   pointer.Int64(65532),
						FSGroup:      pointer.Int64(65532),
					},
					Containers: []corev1.Container{
						{
							Name:            "gardener-scheduler",
							Image:           testValues.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--config=/etc/gardener-scheduler/config/schedulerconfiguration.yaml",
							},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt32(10251),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 15,
								TimeoutSeconds:      5,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/readyz",
										Port:   intstr.FromInt32(10251),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 10,
								TimeoutSeconds:      5,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "gardener-scheduler-config",
									MountPath: "/etc/gardener-scheduler/config",
								},
								{
									Name:      "kubeconfig",
									MountPath: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "gardener-scheduler-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: configSecretName},
								},
							},
						},
						{
							Name: "kubeconfig",
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									DefaultMode: pointer.Int32(420),
									Sources: []corev1.VolumeProjection{
										{
											Secret: &corev1.SecretProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "generic-token-kubeconfig",
												},
												Items: []corev1.KeyToPath{{
													Key:  "kubeconfig",
													Path: "kubeconfig",
												}},
												Optional: pointer.Bool(false),
											},
										},
										{
											Secret: &corev1.SecretProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "shoot-access-gardener-scheduler",
												},
												Items: []corev1.KeyToPath{{
													Key:  "token",
													Path: "token",
												}},
												Optional: pointer.Bool(false),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	utilruntime.Must(references.InjectAnnotations(deployment))

	return componenttest.Serialize(deployment)
}
