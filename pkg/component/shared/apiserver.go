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
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/component/apiserver"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/secretsrotation"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func computeAPIServerAuditConfig(
	ctx context.Context,
	cl client.Client,
	objectMeta metav1.ObjectMeta,
	config *gardencorev1beta1.AuditConfig,
	webhookConfig *apiserver.AuditWebhook,
) (
	*apiserver.AuditConfig,
	error,
) {
	if config == nil || config.AuditPolicy == nil || config.AuditPolicy.ConfigMapRef == nil {
		return nil, nil
	}

	var (
		out = &apiserver.AuditConfig{
			Webhook: webhookConfig,
		}
		key = kubernetesutils.Key(objectMeta.Namespace, config.AuditPolicy.ConfigMapRef.Name)
	)

	configMap := &corev1.ConfigMap{}
	if err := cl.Get(ctx, key, configMap); err != nil {
		// Ignore missing audit configuration on cluster deletion to prevent failing redeployments of the
		// API server in case the end-user deleted the configmap before/simultaneously to the deletion.
		if !apierrors.IsNotFound(err) || objectMeta.DeletionTimestamp == nil {
			return nil, fmt.Errorf("retrieving audit policy from the ConfigMap %s failed: %w", key, err)
		}
	} else {
		policy, ok := configMap.Data["policy"]
		if !ok {
			return nil, fmt.Errorf("missing '.data.policy' in audit policy ConfigMap %s", key)
		}
		out.Policy = &policy
	}

	return out, nil
}

func computeEnabledAPIServerAdmissionPlugins(defaultPlugins, configuredPlugins []gardencorev1beta1.AdmissionPlugin) []gardencorev1beta1.AdmissionPlugin {
	for _, plugin := range configuredPlugins {
		pluginOverwritesDefault := false

		for i, defaultPlugin := range defaultPlugins {
			if defaultPlugin.Name == plugin.Name {
				pluginOverwritesDefault = true
				defaultPlugins[i] = plugin
				break
			}
		}

		if !pluginOverwritesDefault {
			defaultPlugins = append(defaultPlugins, plugin)
		}
	}

	var admissionPlugins []gardencorev1beta1.AdmissionPlugin
	for _, defaultPlugin := range defaultPlugins {
		if !pointer.BoolDeref(defaultPlugin.Disabled, false) {
			admissionPlugins = append(admissionPlugins, defaultPlugin)
		}
	}
	return admissionPlugins
}

func computeDisabledAPIServerAdmissionPlugins(configuredPlugins []gardencorev1beta1.AdmissionPlugin) []gardencorev1beta1.AdmissionPlugin {
	var disabledAdmissionPlugins []gardencorev1beta1.AdmissionPlugin

	for _, plugin := range configuredPlugins {
		if pointer.BoolDeref(plugin.Disabled, false) {
			disabledAdmissionPlugins = append(disabledAdmissionPlugins, plugin)
		}
	}

	return disabledAdmissionPlugins
}

func convertToAdmissionPluginConfigs(ctx context.Context, gardenClient client.Client, namespace string, plugins []gardencorev1beta1.AdmissionPlugin) ([]apiserver.AdmissionPluginConfig, error) {
	var (
		err error
		out []apiserver.AdmissionPluginConfig
	)

	for _, plugin := range plugins {
		config := apiserver.AdmissionPluginConfig{AdmissionPlugin: plugin}
		if plugin.KubeconfigSecretName != nil {
			key := client.ObjectKey{Namespace: namespace, Name: *plugin.KubeconfigSecretName}
			config.Kubeconfig, err = gardenerutils.FetchKubeconfigFromSecret(ctx, gardenClient, key)
			if err != nil {
				return nil, fmt.Errorf("failed reading kubeconfig for admission plugin from referenced secret %s: %w", key, err)
			}
		}
		out = append(out, config)
	}

	return out, nil
}

func computeAPIServerETCDEncryptionConfig(
	ctx context.Context,
	runtimeClient client.Client,
	runtimeNamespace string,
	deploymentName string,
	etcdEncryptionKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase,
	resources []string,
) (
	apiserver.ETCDEncryptionConfig,
	error,
) {
	config := apiserver.ETCDEncryptionConfig{
		RotationPhase:         etcdEncryptionKeyRotationPhase,
		EncryptWithCurrentKey: true,
		Resources:             resources,
	}

	if etcdEncryptionKeyRotationPhase == gardencorev1beta1.RotationPreparing {
		deployment := &metav1.PartialObjectMetadata{}
		deployment.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
		if err := runtimeClient.Get(ctx, kubernetesutils.Key(runtimeNamespace, deploymentName), deployment); err != nil {
			if !apierrors.IsNotFound(err) {
				return apiserver.ETCDEncryptionConfig{}, err
			}
		}

		// If the new encryption key was not yet populated to all replicas then we should still use the old key for
		// encryption of data. Only if all replicas know the new key we can switch and start encrypting with the new/
		// current key, see https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/#rotating-a-decryption-key.
		if !metav1.HasAnnotation(deployment.ObjectMeta, secretsrotation.AnnotationKeyNewEncryptionKeyPopulated) {
			config.EncryptWithCurrentKey = false
		}
	}

	return config, nil
}

func handleETCDEncryptionKeyRotation(
	ctx context.Context,
	runtimeClient client.Client,
	runtimeNamespace string,
	deploymentName string,
	apiServer apiserver.Interface,
	etcdEncryptionConfig apiserver.ETCDEncryptionConfig,
	etcdEncryptionKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase,
) error {
	switch etcdEncryptionKeyRotationPhase {
	case gardencorev1beta1.RotationPreparing:
		if !etcdEncryptionConfig.EncryptWithCurrentKey {
			if err := apiServer.Wait(ctx); err != nil {
				return err
			}

			// If we have hit this point then we have deployed API server successfully with the configuration option to
			// still use the old key for the encryption of ETCD data. Now we can mark this step as "completed" (via an
			// annotation) and redeploy it with the option to use the current/new key for encryption, see
			// https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/#rotating-a-decryption-key for details.
			if err := secretsrotation.PatchAPIServerDeploymentMeta(ctx, runtimeClient, runtimeNamespace, deploymentName, func(meta *metav1.PartialObjectMetadata) {
				metav1.SetMetaDataAnnotation(&meta.ObjectMeta, secretsrotation.AnnotationKeyNewEncryptionKeyPopulated, "true")
			}); err != nil {
				return err
			}

			etcdEncryptionConfig.EncryptWithCurrentKey = true
			apiServer.SetETCDEncryptionConfig(etcdEncryptionConfig)

			if err := apiServer.Deploy(ctx); err != nil {
				return err
			}
		}

	case gardencorev1beta1.RotationCompleting:
		if err := secretsrotation.PatchAPIServerDeploymentMeta(ctx, runtimeClient, runtimeNamespace, deploymentName, func(meta *metav1.PartialObjectMetadata) {
			delete(meta.Annotations, secretsrotation.AnnotationKeyNewEncryptionKeyPopulated)
		}); err != nil {
			return err
		}
	}

	return nil
}
