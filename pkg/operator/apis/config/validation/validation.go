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
	"time"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operator/apis/config"
)

// ValidateOperatorConfiguration validates the given `OperatorConfiguration`.
func ValidateOperatorConfiguration(conf *config.OperatorConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.LogLevel != "" && !sets.New(logger.AllLogLevels...).Has(conf.LogLevel) {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("logLevel"), conf.LogLevel, logger.AllLogLevels))
	}

	if conf.LogFormat != "" && !sets.New(logger.AllLogFormats...).Has(conf.LogFormat) {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("logFormat"), conf.LogFormat, logger.AllLogFormats))
	}

	allErrs = append(allErrs, validateControllerConfiguration(conf.Controllers, field.NewPath("controllers"))...)
	allErrs = append(allErrs, validateNodeTolerationConfiguration(conf.NodeToleration, field.NewPath("nodeToleration"))...)

	return allErrs
}

func validateControllerConfiguration(conf config.ControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateGardenControllerConfiguration(conf.Garden, fldPath.Child("garden"))...)
	allErrs = append(allErrs, validateGardenCareControllerConfiguration(conf.GardenCare, fldPath.Child("gardenCare"))...)
	allErrs = append(allErrs, validateNetworkPolicyControllerConfiguration(conf.NetworkPolicy, fldPath.Child("networkPolicy"))...)

	return allErrs
}

func validateGardenControllerConfiguration(conf config.GardenControllerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateConcurrentSyncs(conf.ConcurrentSyncs, fldPath)...)
	allErrs = append(allErrs, validateSyncPeriod(conf.SyncPeriod, fldPath)...)

	return allErrs
}

func validateGardenCareControllerConfiguration(conf config.GardenCareControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateSyncPeriod(conf.SyncPeriod, fldPath)...)

	return allErrs
}

func validateNetworkPolicyControllerConfiguration(conf config.NetworkPolicyControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateConcurrentSyncs(conf.ConcurrentSyncs, fldPath)...)

	for i, l := range conf.AdditionalNamespaceSelectors {
		labelSelector := l
		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(&labelSelector, metav1validation.LabelSelectorValidationOptions{}, fldPath.Child("additionalNamespaceSelectors").Index(i))...)
	}

	return allErrs
}

func validateConcurrentSyncs(val *int, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if ptr.Deref(val, 0) <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("concurrentSyncs"), val, "must be at least 1"))
	}

	return allErrs
}

func validateSyncPeriod(val *metav1.Duration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if val == nil || val.Duration < 15*time.Second {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("syncPeriod"), val, "must be at least 15s"))
	}

	return allErrs
}

func validateNodeTolerationConfiguration(conf *config.NodeTolerationConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf == nil {
		return allErrs
	}

	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(pointer.Int64Deref(conf.DefaultNotReadyTolerationSeconds, 0), fldPath.Child("defaultNotReadyTolerationSeconds"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(pointer.Int64Deref(conf.DefaultUnreachableTolerationSeconds, 0), fldPath.Child("defaultUnreachableTolerationSeconds"))...)

	return allErrs
}
