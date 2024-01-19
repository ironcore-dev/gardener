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

package shared

import (
	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/nginxingress"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// NewNginxIngress returns a deployer for nginx-ingress-controller.
func NewNginxIngress(
	c client.Client,
	namespaceName string,
	targetNamespace string,
	kubernetesVersion *semver.Version,
	config map[string]string,
	loadBalancerAnnotations map[string]string,
	loadBalancerSourceRanges []string,
	priorityClassName string,
	vpaEnabled bool,
	clusterType component.ClusterType,
	externalTrafficPolicy corev1.ServiceExternalTrafficPolicyType,
	ingressClass string,
) (
	component.DeployWaiter,
	error,
) {
	imageController, err := imagevector.ImageVector().FindImage(imagevector.ImageNameNginxIngressController, imagevectorutils.TargetVersion(kubernetesVersion.String()))
	if err != nil {
		return nil, err
	}
	imageDefaultBackend, err := imagevector.ImageVector().FindImage(imagevector.ImageNameIngressDefaultBackend, imagevectorutils.TargetVersion(kubernetesVersion.String()))
	if err != nil {
		return nil, err
	}

	values := nginxingress.Values{
		KubernetesVersion:        kubernetesVersion,
		ImageController:          imageController.String(),
		ImageDefaultBackend:      imageDefaultBackend.String(),
		IngressClass:             ingressClass,
		ConfigData:               config,
		LoadBalancerAnnotations:  loadBalancerAnnotations,
		LoadBalancerSourceRanges: loadBalancerSourceRanges,
		PriorityClassName:        priorityClassName,
		VPAEnabled:               vpaEnabled,
		TargetNamespace:          targetNamespace,
		ClusterType:              clusterType,
		ExternalTrafficPolicy:    externalTrafficPolicy,
	}

	return nginxingress.New(c, namespaceName, values), nil
}
