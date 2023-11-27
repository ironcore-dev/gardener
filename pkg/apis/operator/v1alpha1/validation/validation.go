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

package validation

import (
	"fmt"
	"net"
	"reflect"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/component-base/featuregate"

	admissioncontrollerconfig "github.com/gardener/gardener/pkg/admissioncontroller/apis/config"
	admissioncontrollerv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	admissioncontrollervalidation "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/validation"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorv1alpha1conversion "github.com/gardener/gardener/pkg/apis/operator/v1alpha1/conversion"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	"github.com/gardener/gardener/pkg/utils/validation/kubernetesversion"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

var gardenCoreScheme *runtime.Scheme

func init() {
	gardenCoreScheme = runtime.NewScheme()
	utilruntime.Must(gardencoreinstall.AddToScheme(gardenCoreScheme))
	utilruntime.Must(admissioncontrollerv1alpha1.AddToScheme(gardenCoreScheme))
}

// ValidateGarden contains functionality for performing extended validation of a Garden object which is not possible
// with standard CRD validation, see https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules.
func ValidateGarden(garden *operatorv1alpha1.Garden) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateOperation(garden.Annotations[v1beta1constants.GardenerOperation], garden, field.NewPath("metadata", "annotations"))...)
	allErrs = append(allErrs, validateRuntimeCluster(garden.Spec.RuntimeCluster, field.NewPath("spec", "runtimeCluster"))...)
	allErrs = append(allErrs, validateVirtualCluster(garden.Spec.VirtualCluster, garden.Spec.RuntimeCluster, field.NewPath("spec", "virtualCluster"))...)

	if helper.TopologyAwareRoutingEnabled(garden.Spec.RuntimeCluster.Settings) {
		if len(garden.Spec.RuntimeCluster.Provider.Zones) <= 1 {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "runtimeCluster", "settings", "topologyAwareRouting", "enabled"), "topology-aware routing can only be enabled on multi-zone garden runtime cluster (with at least two zones in spec.provider.zones)"))
		}
		if !helper.HighAvailabilityEnabled(garden) {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "runtimeCluster", "settings", "topologyAwareRouting", "enabled"), "topology-aware routing can only be enabled when virtual cluster's high-availability is enabled"))
		}
	}

	return allErrs
}

// ValidateGardenUpdate contains functionality for performing extended validation of a Garden object under update which
// is not possible with standard CRD validation, see https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules.
func ValidateGardenUpdate(oldGarden, newGarden *operatorv1alpha1.Garden) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateVirtualClusterUpdate(oldGarden, newGarden)...)
	allErrs = append(allErrs, ValidateGarden(newGarden)...)

	return allErrs
}

func validateVirtualClusterUpdate(oldGarden, newGarden *operatorv1alpha1.Garden) field.ErrorList {
	var (
		allErrs           = field.ErrorList{}
		oldVirtualCluster = oldGarden.Spec.VirtualCluster
		newVirtualCluster = newGarden.Spec.VirtualCluster
		fldPath           = field.NewPath("spec", "virtualCluster")
	)

	// First domain is immutable. Changing this would incompatibly change the service account issuer in the cluster, ref https://github.com/gardener/gardener/blob/17ff592e734131ef746560641bdcdec3bcfce0f1/pkg/component/kubeapiserver/deployment.go#L585C8-L585C8
	// Note: We can consider supporting this scenario in the future but would need to re-issue all service account tokens during the reconcile run.
	if len(oldVirtualCluster.DNS.Domains) > 0 && len(newVirtualCluster.DNS.Domains) > 0 {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(oldVirtualCluster.DNS.Domains[0], newVirtualCluster.DNS.Domains[0], fldPath.Child("dns", "domains").Index(0))...)
	}

	if oldVirtualCluster.ControlPlane != nil && oldVirtualCluster.ControlPlane.HighAvailability != nil &&
		(newVirtualCluster.ControlPlane == nil || newVirtualCluster.ControlPlane.HighAvailability == nil) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(oldVirtualCluster.ControlPlane, newVirtualCluster.ControlPlane, fldPath.Child("controlPlane", "highAvailability"))...)
	}

	allErrs = append(allErrs, gardencorevalidation.ValidateKubernetesVersionUpdate(newVirtualCluster.Kubernetes.Version, oldVirtualCluster.Kubernetes.Version, fldPath.Child("kubernetes", "version"))...)

	var (
		oldKubeAPIServerConfig    = &gardencore.KubeAPIServerConfig{}
		newKubeAPIServerConfig    = &gardencore.KubeAPIServerConfig{}
		etcdEncryptionKeyRotation = &gardencore.ETCDEncryptionKeyRotation{}
		kubeAPIServerFldPath      = fldPath.Child("kubernetes", "kubeAPIServer")
	)

	if oldKubeAPIServer := oldVirtualCluster.Kubernetes.KubeAPIServer; oldKubeAPIServer != nil && oldKubeAPIServer.KubeAPIServerConfig != nil {
		if err := gardenCoreScheme.Convert(oldKubeAPIServer.KubeAPIServerConfig, oldKubeAPIServerConfig, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(kubeAPIServerFldPath, err))
		}
	}
	if newKubeAPIServer := newVirtualCluster.Kubernetes.KubeAPIServer; newKubeAPIServer != nil && newKubeAPIServer.KubeAPIServerConfig != nil {
		if err := gardenCoreScheme.Convert(newKubeAPIServer.KubeAPIServerConfig, newKubeAPIServerConfig, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(kubeAPIServerFldPath, err))
		}
	}
	if credentials := newGarden.Status.Credentials; credentials != nil && credentials.Rotation != nil && credentials.Rotation.ETCDEncryptionKey != nil {
		if err := gardenCoreScheme.Convert(credentials.Rotation.ETCDEncryptionKey, etcdEncryptionKeyRotation, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(field.NewPath("status", "credentials", "rotation", "etcdEncryptionKey"), err))
		}
	}

	allErrs = append(allErrs, gardencorevalidation.ValidateEncryptionConfigUpdate(newKubeAPIServerConfig, oldKubeAPIServerConfig, etcdEncryptionKeyRotation, kubeAPIServerFldPath.Child("encryptionConfig"))...)

	return allErrs
}

func validateRuntimeCluster(runtimeCluster operatorv1alpha1.RuntimeCluster, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if cidrvalidation.NetworksIntersect(runtimeCluster.Networking.Pods, runtimeCluster.Networking.Services) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "services"), runtimeCluster.Networking.Services, "pod network of runtime cluster intersects with service network of runtime cluster"))
	}
	if runtimeCluster.Networking.Nodes != nil {
		if cidrvalidation.NetworksIntersect(*runtimeCluster.Networking.Nodes, runtimeCluster.Networking.Pods) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "nodes"), *runtimeCluster.Networking.Nodes, "node network of runtime cluster intersects with pod network of runtime cluster"))
		}
		if cidrvalidation.NetworksIntersect(*runtimeCluster.Networking.Nodes, runtimeCluster.Networking.Services) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "nodes"), *runtimeCluster.Networking.Nodes, "node network of runtime cluster intersects with service network of runtime cluster"))
		}
	}

	return allErrs
}

func validateVirtualCluster(virtualCluster operatorv1alpha1.VirtualCluster, runtimeCluster operatorv1alpha1.RuntimeCluster, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	domains := sets.New[string]()
	for i, domain := range virtualCluster.DNS.Domains {
		allErrs = append(allErrs, gardencorevalidation.ValidateDNS1123Subdomain(domain, fldPath.Child("dns", "domains").Index(i))...)
		if domains.Has(domain) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Child("dns", "domains").Index(i), domain))
		}
		domains.Insert(domain)
	}

	if err := kubernetesversion.CheckIfSupported(virtualCluster.Kubernetes.Version); err != nil {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("kubernetes", "version"), virtualCluster.Kubernetes.Version, kubernetesversion.SupportedVersions))
	}

	if kubeAPIServer := virtualCluster.Kubernetes.KubeAPIServer; kubeAPIServer != nil && kubeAPIServer.KubeAPIServerConfig != nil {
		path := fldPath.Child("kubernetes", "kubeAPIServer")

		coreKubeAPIServerConfig := &gardencore.KubeAPIServerConfig{}
		if err := gardenCoreScheme.Convert(kubeAPIServer.KubeAPIServerConfig, coreKubeAPIServerConfig, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(path, err))
		}

		defaultEncryptedResources := gardenerutils.DefaultGardenerResourcesForEncryption().Union(gardenerutils.DefaultResourcesForEncryption())
		allErrs = append(allErrs, gardencorevalidation.ValidateKubeAPIServer(coreKubeAPIServerConfig, virtualCluster.Kubernetes.Version, true, defaultEncryptedResources, path)...)
	}

	if kubeControllerManager := virtualCluster.Kubernetes.KubeControllerManager; kubeControllerManager != nil && kubeControllerManager.KubeControllerManagerConfig != nil {
		path := fldPath.Child("kubernetes", "kubeControllerManager")

		coreKubeControllerManagerConfig := &gardencore.KubeControllerManagerConfig{}
		if err := gardenCoreScheme.Convert(kubeControllerManager.KubeControllerManagerConfig, coreKubeControllerManagerConfig, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(path, err))
		}

		allErrs = append(allErrs, gardencorevalidation.ValidateKubeControllerManager(coreKubeControllerManagerConfig, nil, virtualCluster.Kubernetes.Version, true, path)...)
	}

	allErrs = append(allErrs, validateGardener(virtualCluster.Gardener, fldPath.Child("gardener"))...)

	if _, _, err := net.ParseCIDR(virtualCluster.Networking.Services); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "services"), virtualCluster.Networking.Services, fmt.Sprintf("cannot parse service network cidr: %s", err.Error())))
	}
	if cidrvalidation.NetworksIntersect(runtimeCluster.Networking.Pods, virtualCluster.Networking.Services) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "services"), virtualCluster.Networking.Services, "pod network of runtime cluster intersects with service network of virtual cluster"))
	}
	if cidrvalidation.NetworksIntersect(runtimeCluster.Networking.Services, virtualCluster.Networking.Services) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "services"), virtualCluster.Networking.Services, "service network of runtime cluster intersects with service network of virtual cluster"))
	}
	if runtimeCluster.Networking.Nodes != nil && cidrvalidation.NetworksIntersect(*runtimeCluster.Networking.Nodes, virtualCluster.Networking.Services) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "services"), virtualCluster.Networking.Services, "node network of runtime cluster intersects with service network of virtual cluster"))
	}

	return allErrs
}

func validateGardener(config operatorv1alpha1.Gardener, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateGardenerAPIServerConfig(config.APIServer, fldPath.Child("gardenerAPIServer"))...)
	allErrs = append(allErrs, validateGardenerAdmissionController(config.AdmissionController, fldPath.Child("gardenerAdmissionController"))...)
	allErrs = append(allErrs, validateGardenerControllerManagerConfig(config.ControllerManager, fldPath.Child("gardenerControllerManager"))...)
	allErrs = append(allErrs, validateGardenerSchedulerConfig(config.Scheduler, fldPath.Child("gardenerScheduler"))...)

	return allErrs
}

func validateGardenerAPIServerConfig(config *operatorv1alpha1.GardenerAPIServerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config == nil {
		return allErrs
	}

	allErrs = append(allErrs, validateGardenerFeatureGates(config.FeatureGates, fldPath.Child("featureGates"))...)

	for i, admissionPlugin := range config.AdmissionPlugins {
		idxPath := fldPath.Child("admissionPlugins").Index(i)

		if len(admissionPlugin.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "must provide a name"))
			return allErrs
		}

		if !utils.ValueExists(admissionPlugin.Name, plugin.AllPluginNames()) {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("name"), admissionPlugin.Name, plugin.AllPluginNames()))
		}
	}

	if auditConfig := config.AuditConfig; auditConfig != nil {
		auditPath := fldPath.Child("auditConfig")
		if auditPolicy := auditConfig.AuditPolicy; auditPolicy != nil && auditConfig.AuditPolicy.ConfigMapRef != nil {
			allErrs = append(allErrs, gardencorevalidation.ValidateAuditPolicyConfigMapReference(auditPolicy.ConfigMapRef, auditPath.Child("auditPolicy", "configMapRef"))...)
		}
	}

	if config.WatchCacheSizes != nil {
		watchCacheSizes := &gardencore.WatchCacheSizes{}
		if err := gardenCoreScheme.Convert(config.WatchCacheSizes, watchCacheSizes, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(fldPath.Child("watchCacheSizes"), err))
		}
		allErrs = append(allErrs, gardencorevalidation.ValidateWatchCacheSizes(watchCacheSizes, fldPath.Child("watchCacheSizes"))...)
	}

	if config.Logging != nil {
		logging := &gardencore.APIServerLogging{}
		if err := gardenCoreScheme.Convert(config.Logging, logging, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(fldPath.Child("logging"), err))
		}
		allErrs = append(allErrs, gardencorevalidation.ValidateAPIServerLogging(logging, fldPath.Child("logging"))...)
	}

	if config.Requests != nil {
		requests := &gardencore.APIServerRequests{}
		if err := gardenCoreScheme.Convert(config.Requests, requests, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(fldPath.Child("requests"), err))
		}
		allErrs = append(allErrs, gardencorevalidation.ValidateAPIServerRequests(requests, fldPath.Child("requests"))...)
	}

	return allErrs
}

func validateGardenerAdmissionController(config *operatorv1alpha1.GardenerAdmissionControllerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config == nil {
		return allErrs
	}

	if config.ResourceAdmissionConfiguration != nil {
		externalAdmissionConfiguration := operatorv1alpha1conversion.ConvertToAdmissionControllerResourceAdmissionConfiguration(config.ResourceAdmissionConfiguration)
		internalAdmissionConfiguration := &admissioncontrollerconfig.ResourceAdmissionConfiguration{}
		if err := gardenCoreScheme.Convert(externalAdmissionConfiguration, internalAdmissionConfiguration, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(fldPath.Child("resourceAdmissionConfiguration"), err))
		}
		allErrs = append(allErrs, admissioncontrollervalidation.ValidateResourceAdmissionConfiguration(internalAdmissionConfiguration, fldPath.Child("resourceAdmissionConfiguration"))...)
	}

	return allErrs
}

func validateGardenerControllerManagerConfig(config *operatorv1alpha1.GardenerControllerManagerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config == nil {
		return allErrs
	}

	allErrs = append(allErrs, validateGardenerFeatureGates(config.FeatureGates, fldPath.Child("featureGates"))...)

	for i, quota := range config.DefaultProjectQuotas {
		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(quota.ProjectSelector, metav1validation.LabelSelectorValidationOptions{AllowInvalidLabelValueInSelector: true}, fldPath.Child("defaultProjectQuotas").Index(i).Child("projectSelector"))...)
	}

	return allErrs
}

func validateGardenerSchedulerConfig(config *operatorv1alpha1.GardenerSchedulerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config == nil {
		return allErrs
	}

	allErrs = append(allErrs, validateGardenerFeatureGates(config.FeatureGates, fldPath.Child("featureGates"))...)

	return allErrs
}

func validateGardenerFeatureGates(featureGates map[string]bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for featureGate := range featureGates {
		spec, supported := features.AllFeatureGates[featuregate.Feature(featureGate)]
		if !supported {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child(featureGate), "not supported by Gardener"))
		} else {
			if spec.LockToDefault && featureGates[featureGate] != spec.Default {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child(featureGate), fmt.Sprintf("cannot set feature gate to %v, feature is locked to %v", featureGates[featureGate], spec.Default)))
			}
		}
	}

	return allErrs
}

func validateOperation(operation string, garden *operatorv1alpha1.Garden, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if operation == "" {
		return allErrs
	}

	fldPathOp := fldPath.Key(v1beta1constants.GardenerOperation)

	if operation != "" && !operatorv1alpha1.AvailableOperationAnnotations.Has(operation) {
		allErrs = append(allErrs, field.NotSupported(fldPathOp, operation, sets.List(operatorv1alpha1.AvailableOperationAnnotations)))
	}
	allErrs = append(allErrs, validateOperationContext(operation, garden, fldPathOp)...)

	return allErrs
}

func validateOperationContext(operation string, garden *operatorv1alpha1.Garden, fldPath *field.Path) field.ErrorList {
	var (
		allErrs         = field.ErrorList{}
		apiServerConfig *gardencorev1beta1.KubeAPIServerConfig
	)

	if garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer != nil {
		apiServerConfig = garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.KubeAPIServerConfig
	}

	switch operation {
	case v1beta1constants.OperationRotateCredentialsStart:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if garden has deletion timestamp"))
		}
		if phase := helper.GetCARotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if .status.credentials.rotation.certificateAuthorities.phase is not 'Completed'"))
		}
		if phase := helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if .status.credentials.rotation.serviceAccountKey.phase is not 'Completed'"))
		}
		if phase := helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Completed'"))
		}
		if !reflect.DeepEqual(sharedcomponent.GetResourcesForEncryptionFromConfig(apiServerConfig, nil), garden.Status.EncryptedResources) {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials when spec.virtualCluster.kubernetes.kubeAPIServer.encryptionConfig.resources and status.encryptedResources are not equal"))
		}
	case v1beta1constants.OperationRotateCredentialsComplete:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if garden has deletion timestamp"))
		}
		if helper.GetCARotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if .status.credentials.rotation.certificateAuthorities.phase is not 'Prepared'"))
		}
		if helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if .status.credentials.rotation.serviceAccountKey.phase is not 'Prepared'"))
		}
		if helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Prepared'"))
		}

	case v1beta1constants.OperationRotateCAStart:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start CA rotation if garden has deletion timestamp"))
		}
		if phase := helper.GetCARotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start CA rotation if .status.credentials.rotation.certificateAuthorities.phase is not 'Completed'"))
		}
	case v1beta1constants.OperationRotateCAComplete:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete CA rotation if garden has deletion timestamp"))
		}
		if helper.GetCARotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete CA rotation if .status.credentials.rotation.certificateAuthorities.phase is not 'Prepared'"))
		}

	case v1beta1constants.OperationRotateServiceAccountKeyStart:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start service account key rotation if garden has deletion timestamp"))
		}
		if phase := helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start service account key rotation if .status.credentials.rotation.serviceAccountKey.phase is not 'Completed'"))
		}
	case v1beta1constants.OperationRotateServiceAccountKeyComplete:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete service account key rotation if garden has deletion timestamp"))
		}
		if helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete service account key rotation if .status.credentials.rotation.serviceAccountKey.phase is not 'Prepared'"))
		}

	case v1beta1constants.OperationRotateETCDEncryptionKeyStart:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start ETCD encryption key rotation if garden has deletion timestamp"))
		}
		if phase := helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start ETCD encryption key rotation if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Completed'"))
		}
		if !reflect.DeepEqual(sharedcomponent.GetResourcesForEncryptionFromConfig(apiServerConfig, nil), garden.Status.EncryptedResources) {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start ETCD encryption key rotation when spec.virtualCluster.kubernetes.kubeAPIServer.encryptionConfig.resources and status.encryptedResources are not equal"))
		}
	case v1beta1constants.OperationRotateETCDEncryptionKeyComplete:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete ETCD encryption key rotation if garden has deletion timestamp"))
		}
		if helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete ETCD encryption key rotation if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Prepared'"))
		}

	case v1beta1constants.OperationRotateObservabilityCredentials:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start Observability credentials rotation if garden has deletion timestamp"))
		}
	}

	return allErrs
}
