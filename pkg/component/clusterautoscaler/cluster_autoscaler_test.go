// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package clusterautoscaler_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/clusterautoscaler"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ClusterAutoscaler", func() {
	var (
		ctrl              *gomock.Controller
		c                 *mockclient.MockClient
		fakeClient        client.Client
		sm                secretsmanager.Interface
		clusterAutoscaler Interface

		ctx                = context.TODO()
		fakeErr            = fmt.Errorf("fake error")
		namespace          = "shoot--foo--bar"
		namespaceUID       = types.UID("1234567890")
		image              = "registry.k8s.io/cluster-autoscaler:v1.2.3"
		replicas     int32 = 1

		machineDeployment1Name       = "pool1"
		machineDeployment1Min  int32 = 2
		machineDeployment1Max  int32 = 4
		machineDeployment2Name       = "pool2"
		machineDeployment2Min  int32 = 3
		machineDeployment2Max  int32 = 5
		machineDeployments           = []extensionsv1alpha1.MachineDeployment{
			{Name: machineDeployment1Name, Minimum: machineDeployment1Min, Maximum: machineDeployment1Max},
			{Name: machineDeployment2Name, Minimum: machineDeployment2Min, Maximum: machineDeployment2Max},
		}

		configExpander                            = gardencorev1beta1.ClusterAutoscalerExpanderRandom
		configMaxGracefulTerminationSeconds int32 = 60 * 60 * 24
		configMaxNodeProvisionTime                = &metav1.Duration{Duration: time.Second}
		configScaleDownDelayAfterAdd              = &metav1.Duration{Duration: time.Second}
		configScaleDownDelayAfterDelete           = &metav1.Duration{Duration: time.Second}
		configScaleDownDelayAfterFailure          = &metav1.Duration{Duration: time.Second}
		configScaleDownUnneededTime               = &metav1.Duration{Duration: time.Second}
		configScaleDownUtilizationThreshold       = pointer.Float64(1.2345)
		configScanInterval                        = &metav1.Duration{Duration: time.Second}
		configIgnoreDaemonsetsUtilization   bool  = true
		configVerbosity                     int32 = 4
		configMaxEmptyBulkDelete                  = pointer.Int32(20)
		configNewPodScaleUpDelay                  = &metav1.Duration{Duration: time.Second}
		configIgnoreTaints                        = []string{"taint-1", "taint-2"}
		configFull                                = &gardencorev1beta1.ClusterAutoscaler{
			Expander:                      &configExpander,
			MaxGracefulTerminationSeconds: &configMaxGracefulTerminationSeconds,
			MaxNodeProvisionTime:          configMaxNodeProvisionTime,
			ScaleDownDelayAfterAdd:        configScaleDownDelayAfterAdd,
			ScaleDownDelayAfterDelete:     configScaleDownDelayAfterDelete,
			ScaleDownDelayAfterFailure:    configScaleDownDelayAfterFailure,
			ScaleDownUnneededTime:         configScaleDownUnneededTime,
			ScaleDownUtilizationThreshold: configScaleDownUtilizationThreshold,
			ScanInterval:                  configScanInterval,
			IgnoreTaints:                  configIgnoreTaints,
			IgnoreDaemonsetsUtilization:   &configIgnoreDaemonsetsUtilization,
			Verbosity:                     &configVerbosity,
			MaxEmptyBulkDelete:            configMaxEmptyBulkDelete,
			NewPodScaleUpDelay:            configNewPodScaleUpDelay,
		}

		genericTokenKubeconfigSecretName = "generic-token-kubeconfig"
		serviceAccountName               = "cluster-autoscaler"
		secretName                       = "shoot-access-cluster-autoscaler"
		clusterRoleBindingName           = "cluster-autoscaler-" + namespace
		vpaName                          = "cluster-autoscaler-vpa"
		pdbName                          = "cluster-autoscaler"
		serviceName                      = "cluster-autoscaler"
		deploymentName                   = "cluster-autoscaler"
		managedResourceName              = "shoot-core-cluster-autoscaler"
		managedResourceSecretName        = "managedresource-shoot-core-cluster-autoscaler"

		vpaUpdateMode    = vpaautoscalingv1.UpdateModeAuto
		controlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		vpa              = &vpaautoscalingv1.VerticalPodAutoscaler{
			TypeMeta: metav1.TypeMeta{
				APIVersion: vpaautoscalingv1.SchemeGroupVersion.String(),
				Kind:       "VerticalPodAutoscaler",
			},
			ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace, ResourceVersion: "1"},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       deploymentName,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("50Mi"),
							},
							ControlledValues: &controlledValues,
						},
					},
				},
			},
		}
		pdbMaxUnavailable = intstr.FromInt32(1)
		pdb               = &policyv1.PodDisruptionBudget{
			TypeMeta: metav1.TypeMeta{
				APIVersion: policyv1.SchemeGroupVersion.String(),
				Kind:       "PodDisruptionBudget",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      pdbName,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "cluster-autoscaler",
				},
				ResourceVersion: "1",
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: &pdbMaxUnavailable,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":  "kubernetes",
						"role": "cluster-autoscaler",
					},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				APIVersion: rbacv1.SchemeGroupVersion.String(),
				Kind:       "ClusterRoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleBindingName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "v1",
					Kind:               "Namespace",
					Name:               namespace,
					UID:                namespaceUID,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				}},
				ResourceVersion: "1",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "system:cluster-autoscaler-seed",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: namespace,
			}},
		}
		serviceAccount = &corev1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "ServiceAccount",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            serviceAccountName,
				Namespace:       namespace,
				ResourceVersion: "1",
			},
			AutomountServiceAccountToken: ptr.To(false),
		}
		service = &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "cluster-autoscaler",
				},
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":8085}]`,
				},
				ResourceVersion: "1",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app":  "kubernetes",
					"role": "cluster-autoscaler",
				},
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: corev1.ClusterIPNone,
				Ports: []corev1.ServicePort{
					{
						Name:     "metrics",
						Protocol: corev1.ProtocolTCP,
						Port:     8085,
					},
				},
			},
		}
		secret = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "cluster-autoscaler",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
				ResourceVersion: "1",
			},
			Type: corev1.SecretTypeOpaque,
		}
		deploymentFor = func(withConfig bool) *appsv1.Deployment {
			var commandConfigFlags []string
			if !withConfig {
				commandConfigFlags = append(commandConfigFlags,
					"--expander=least-waste",
					"--max-graceful-termination-sec=600",
					"--max-node-provision-time=20m0s",
					"--scale-down-utilization-threshold=0.500000",
					"--scale-down-unneeded-time=30m0s",
					"--scale-down-delay-after-add=1h0m0s",
					"--scale-down-delay-after-delete=0s",
					"--scale-down-delay-after-failure=3m0s",
					"--scan-interval=10s",
					"--ignore-daemonsets-utilization=false",
					"--v=2",
					"--max-empty-bulk-delete=10",
					"--new-pod-scale-up-delay=0s",
				)
			} else {
				commandConfigFlags = append(commandConfigFlags,
					fmt.Sprintf("--expander=%s", string(configExpander)),
					fmt.Sprintf("--max-graceful-termination-sec=%d", configMaxGracefulTerminationSeconds),
					fmt.Sprintf("--max-node-provision-time=%s", configMaxNodeProvisionTime.Duration),
					fmt.Sprintf("--scale-down-utilization-threshold=%f", *configScaleDownUtilizationThreshold),
					fmt.Sprintf("--scale-down-unneeded-time=%s", configScaleDownUnneededTime.Duration),
					fmt.Sprintf("--scale-down-delay-after-add=%s", configScaleDownDelayAfterAdd.Duration),
					fmt.Sprintf("--scale-down-delay-after-delete=%s", configScaleDownDelayAfterDelete.Duration),
					fmt.Sprintf("--scale-down-delay-after-failure=%s", configScaleDownDelayAfterFailure.Duration),
					fmt.Sprintf("--scan-interval=%s", configScanInterval.Duration),
					fmt.Sprintf("--ignore-daemonsets-utilization=%t", configIgnoreDaemonsetsUtilization),
					fmt.Sprintf("--v=%d", configVerbosity),
					fmt.Sprintf("--max-empty-bulk-delete=%d", *configMaxEmptyBulkDelete),
					fmt.Sprintf("--new-pod-scale-up-delay=%s", configNewPodScaleUpDelay.Duration),
					fmt.Sprintf("--ignore-taint=%s", configIgnoreTaints[0]),
					fmt.Sprintf("--ignore-taint=%s", configIgnoreTaints[1]),
				)
			}

			command := append([]string{
				"./cluster-autoscaler",
				"--address=:8085",
				"--kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
				"--cloud-provider=mcm",
				"--stderrthreshold=info",
				"--skip-nodes-with-system-pods=false",
				"--skip-nodes-with-local-storage=false",
				"--expendable-pods-priority-cutoff=-10",
				"--balance-similar-node-groups=true",
				"--ignore-taint=node.gardener.cloud/critical-components-not-ready",
			}, commandConfigFlags...)
			command = append(command,
				fmt.Sprintf("--nodes=%d:%d:%s.%s", machineDeployment1Min, machineDeployment1Max, namespace, machineDeployment1Name),
				fmt.Sprintf("--nodes=%d:%d:%s.%s", machineDeployment2Min, machineDeployment2Max, namespace, machineDeployment2Name),
			)

			deploy := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: namespace,
					Labels: map[string]string{
						"app":                 "kubernetes",
						"role":                "cluster-autoscaler",
						"gardener.cloud/role": "controlplane",
						"high-availability-config.resources.gardener.cloud/type": "controller",
					},
					ResourceVersion: "1",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas:             &replicas,
					RevisionHistoryLimit: pointer.Int32(1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":  "kubernetes",
							"role": "cluster-autoscaler",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app":                                "kubernetes",
								"role":                               "cluster-autoscaler",
								"gardener.cloud/role":                "controlplane",
								"maintenance.gardener.cloud/restart": "true",
								"networking.gardener.cloud/to-dns":   "allowed",
								"networking.gardener.cloud/to-runtime-apiserver":                "allowed",
								"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:            "cluster-autoscaler",
									Image:           image,
									ImagePullPolicy: corev1.PullIfNotPresent,
									Command:         command,
									Ports: []corev1.ContainerPort{
										{
											Name:          "metrics",
											ContainerPort: 8085,
											Protocol:      corev1.ProtocolTCP,
										},
									},
									Env: []corev1.EnvVar{
										{
											Name:  "CONTROL_NAMESPACE",
											Value: namespace,
										},
										{
											Name:  "TARGET_KUBECONFIG",
											Value: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
										},
									},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("100m"),
											corev1.ResourceMemory: resource.MustParse("300Mi"),
										},
									},
								},
							},
							PriorityClassName:             v1beta1constants.PriorityClassNameShootControlPlane300,
							ServiceAccountName:            serviceAccountName,
							TerminationGracePeriodSeconds: pointer.Int64(5),
						},
					},
				},
			}

			Expect(gardenerutils.InjectGenericKubeconfig(deploy, genericTokenKubeconfigSecretName, secret.Name)).To(Succeed())
			return deploy
		}

		clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: gardener.cloud:target:cluster-autoscaler
rules:
- apiGroups:
  - ""
  resources:
  - events
  - endpoints
  verbs:
  - create
  - patch
- apiGroups:
  - ""
  resources:
  - pods/eviction
  verbs:
  - create
- apiGroups:
  - ""
  resources:
  - pods/status
  verbs:
  - update
- apiGroups:
  - ""
  resourceNames:
  - cluster-autoscaler
  resources:
  - endpoints
  verbs:
  - get
  - update
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - watch
  - list
  - get
  - update
- apiGroups:
  - ""
  resources:
  - namespaces
  - pods
  - services
  - replicationcontrollers
  - persistentvolumeclaims
  - persistentvolumes
  verbs:
  - watch
  - list
  - get
- apiGroups:
  - apps
  - extensions
  resources:
  - daemonsets
  - replicasets
  - statefulsets
  verbs:
  - watch
  - list
  - get
- apiGroups:
  - policy
  resources:
  - poddisruptionbudgets
  verbs:
  - watch
  - list
- apiGroups:
  - storage.k8s.io
  resources:
  - storageclasses
  - csinodes
  - csidrivers
  - csistoragecapacities
  verbs:
  - watch
  - list
  - get
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - create
- apiGroups:
  - coordination.k8s.io
  resourceNames:
  - cluster-autoscaler
  resources:
  - leases
  verbs:
  - get
  - update
- apiGroups:
  - batch
  - extensions
  resources:
  - jobs
  verbs:
  - get
  - list
  - patch
  - watch
- apiGroups:
  - batch
  resources:
  - jobs
  - cronjobs
  verbs:
  - get
  - list
  - watch
`
		clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: gardener.cloud:target:cluster-autoscaler
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:target:cluster-autoscaler
subjects:
- kind: ServiceAccount
  name: cluster-autoscaler
  namespace: kube-system
`

		roleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  creationTimestamp: null
  name: gardener.cloud:target:cluster-autoscaler
  namespace: kube-system
rules:
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - watch
  - list
  - get
  - create
- apiGroups:
  - ""
  resourceNames:
  - cluster-autoscaler-status
  resources:
  - configmaps
  verbs:
  - delete
  - update
`

		roleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  creationTimestamp: null
  name: gardener.cloud:target:cluster-autoscaler
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: gardener.cloud:target:cluster-autoscaler
subjects:
- kind: ServiceAccount
  name: cluster-autoscaler
`
		managedResourceSecret = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            managedResourceSecretName,
				Namespace:       namespace,
				ResourceVersion: "1",
				Labels: map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"clusterrole____gardener.cloud_target_cluster-autoscaler.yaml":            []byte(clusterRoleYAML),
				"clusterrolebinding____gardener.cloud_target_cluster-autoscaler.yaml":     []byte(clusterRoleBindingYAML),
				"role__kube-system__gardener.cloud_target_cluster-autoscaler.yaml":        []byte(roleYAML),
				"rolebinding__kube-system__gardener.cloud_target_cluster-autoscaler.yaml": []byte(roleBindingYAML),
			},
			Immutable: ptr.To(true),
		}
		managedResource = &resourcesv1alpha1.ManagedResource{
			TypeMeta: metav1.TypeMeta{
				APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
				Kind:       "ManagedResource",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            managedResourceName,
				Namespace:       namespace,
				Labels:          map[string]string{"origin": "gardener"},
				ResourceVersion: "1",
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs:   []corev1.LocalObjectReference{},
				InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
				KeepObjects:  ptr.To(false),
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())

		clusterAutoscaler = New(c, namespace, sm, image, replicas, nil, nil)
		clusterAutoscaler.SetNamespaceUID(namespaceUID)
		clusterAutoscaler.SetMachineDeployments(machineDeployments)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		Context("should successfully deploy all the resources", func() {
			test := func(withConfig bool, runtimeVersionGreaterEquals126 bool) {
				var config *gardencorev1beta1.ClusterAutoscaler
				if withConfig {
					config = configFull
				}

				if runtimeVersionGreaterEquals126 {
					clusterAutoscaler = New(fakeClient, namespace, sm, image, replicas, config, semver.MustParse("1.26.1"))
				} else {
					clusterAutoscaler = New(fakeClient, namespace, sm, image, replicas, config, semver.MustParse("1.25.0"))
				}
				clusterAutoscaler.SetNamespaceUID(namespaceUID)
				clusterAutoscaler.SetMachineDeployments(machineDeployments)

				Expect(clusterAutoscaler.Deploy(ctx)).To(Succeed())

				actualMr := &resourcesv1alpha1.ManagedResource{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), actualMr)).To(Succeed())
				managedResource.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: actualMr.Spec.SecretRefs[0].Name}}

				utilruntime.Must(references.InjectAnnotations(managedResource))
				Expect(actualMr).To(DeepEqual(managedResource))

				actualMRSecret := &corev1.Secret{}
				managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), actualMRSecret)).To(Succeed())
				Expect(actualMRSecret).To(DeepEqual(managedResourceSecret))

				actualServiceAccount := &corev1.ServiceAccount{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), actualServiceAccount)).To(Succeed())
				Expect(actualServiceAccount).To(DeepEqual(serviceAccount))

				actualClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), actualClusterRoleBinding)).To(Succeed())
				Expect(actualClusterRoleBinding).To(DeepEqual(clusterRoleBinding))

				actualService := &corev1.Service{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(service), actualService)).To(Succeed())
				Expect(actualService).To(DeepEqual(service))

				actualSecret := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), actualSecret)).To(Succeed())
				Expect(actualSecret).To(DeepEqual(secret))

				actualDeployment := &appsv1.Deployment{}
				deploy := deploymentFor(withConfig)
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), actualDeployment)).To(Succeed())
				Expect(actualDeployment).To(DeepEqual(deploy))

				actualPDB := &policyv1.PodDisruptionBudget{}
				unhealthyPodEvictionPolicyAlwatysAllow := policyv1.AlwaysAllow
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pdb), actualPDB)).To(Succeed())
				if runtimeVersionGreaterEquals126 {
					pdb.Spec.UnhealthyPodEvictionPolicy = &unhealthyPodEvictionPolicyAlwatysAllow
				}
				Expect(actualPDB).To(DeepEqual(pdb))

				actualVPA := &vpaautoscalingv1.VerticalPodAutoscaler{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(vpa), actualVPA)).To(Succeed())
				Expect(actualVPA).To(DeepEqual(vpa))
			}

			It("w/o config", func() { test(false, false) })
			It("w/ config, kubernetes version < 1.26", func() { test(true, false) })
			It("w/ config, kubernetes version >= 1.26", func() { test(true, true) })
		})
	})

	Describe("#Destroy", func() {
		It("should fail because the managed resource cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the managed resource secret cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the vpa cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the pod disruption budget cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the deployment cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the cluster role binding cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the secret cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: secretName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the service cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: secretName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the service account cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: secretName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceName}}),
				c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceAccountName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully delete all the resources", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: secretName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceName}}),
				c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceAccountName}}),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(clusterAutoscaler.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(clusterAutoscaler.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
