//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by defaulter-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// RegisterDefaults adds defaulters functions to the given scheme.
// Public to allow building arbitrary schemes.
// All generated defaulters are covering - they call all nested defaulters.
func RegisterDefaults(scheme *runtime.Scheme) error {
	scheme.AddTypeDefaultingFunc(&ControllerManagerConfiguration{}, func(obj interface{}) {
		SetObjectDefaults_ControllerManagerConfiguration(obj.(*ControllerManagerConfiguration))
	})
	return nil
}

func SetObjectDefaults_ControllerManagerConfiguration(in *ControllerManagerConfiguration) {
	SetDefaults_ControllerManagerConfiguration(in)
	SetDefaults_ClientConnectionConfiguration(&in.GardenClientConnection)
	SetDefaults_ControllerManagerControllerConfiguration(&in.Controllers)
	if in.Controllers.Bastion != nil {
		SetDefaults_BastionControllerConfiguration(in.Controllers.Bastion)
	}
	if in.Controllers.CertificateSigningRequest != nil {
		SetDefaults_CertificateSigningRequestControllerConfiguration(in.Controllers.CertificateSigningRequest)
	}
	if in.Controllers.CloudProfile != nil {
		SetDefaults_CloudProfileControllerConfiguration(in.Controllers.CloudProfile)
	}
	if in.Controllers.ControllerDeployment != nil {
		SetDefaults_ControllerDeploymentControllerConfiguration(in.Controllers.ControllerDeployment)
	}
	if in.Controllers.ControllerRegistration != nil {
		SetDefaults_ControllerRegistrationControllerConfiguration(in.Controllers.ControllerRegistration)
	}
	if in.Controllers.Event != nil {
		SetDefaults_EventControllerConfiguration(in.Controllers.Event)
	}
	if in.Controllers.ExposureClass != nil {
		SetDefaults_ExposureClassControllerConfiguration(in.Controllers.ExposureClass)
	}
	if in.Controllers.Project != nil {
		SetDefaults_ProjectControllerConfiguration(in.Controllers.Project)
	}
	if in.Controllers.Quota != nil {
		SetDefaults_QuotaControllerConfiguration(in.Controllers.Quota)
	}
	if in.Controllers.SecretBinding != nil {
		SetDefaults_SecretBindingControllerConfiguration(in.Controllers.SecretBinding)
	}
	if in.Controllers.Seed != nil {
		SetDefaults_SeedControllerConfiguration(in.Controllers.Seed)
	}
	if in.Controllers.SeedExtensionsCheck != nil {
		SetDefaults_SeedExtensionsCheckControllerConfiguration(in.Controllers.SeedExtensionsCheck)
	}
	if in.Controllers.SeedBackupBucketsCheck != nil {
		SetDefaults_SeedBackupBucketsCheckControllerConfiguration(in.Controllers.SeedBackupBucketsCheck)
	}
	SetDefaults_ShootMaintenanceControllerConfiguration(&in.Controllers.ShootMaintenance)
	if in.Controllers.ShootQuota != nil {
		SetDefaults_ShootQuotaControllerConfiguration(in.Controllers.ShootQuota)
	}
	SetDefaults_ShootHibernationControllerConfiguration(&in.Controllers.ShootHibernation)
	if in.Controllers.ShootReference != nil {
		SetDefaults_ShootReferenceControllerConfiguration(in.Controllers.ShootReference)
	}
	if in.Controllers.ShootRetry != nil {
		SetDefaults_ShootRetryControllerConfiguration(in.Controllers.ShootRetry)
	}
	if in.Controllers.ShootConditions != nil {
		SetDefaults_ShootConditionsControllerConfiguration(in.Controllers.ShootConditions)
	}
	if in.Controllers.ShootStatusLabel != nil {
		SetDefaults_ShootStatusLabelControllerConfiguration(in.Controllers.ShootStatusLabel)
	}
	if in.Controllers.ManagedSeedSet != nil {
		SetDefaults_ManagedSeedSetControllerConfiguration(in.Controllers.ManagedSeedSet)
	}
	if in.LeaderElection != nil {
		SetDefaults_LeaderElectionConfiguration(in.LeaderElection)
	}
	SetDefaults_ServerConfiguration(&in.Server)
}
