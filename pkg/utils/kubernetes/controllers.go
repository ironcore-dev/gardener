// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetes

import (
	"fmt"

	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

type VersionRange struct {
	AddedInVersion   string
	RemovedInVersion string
}

var APIGroupControllerMap = map[string]map[string]VersionRange{
	"certificates/v1beta1": {
		"csrsigning": {},
	},
	"apps/v1beta1": {
		"disruption": {},
	},
	"authorization/v1": {
		"csrapproving": {},
	},
	"extensions/v1beta1": {
		"disruption": {},
	},
	"coordination/v1": {
		"nodelifecycle":      {},
		"storage-version-gc": {},
	},
	"core/v1": {
		"attachdetach":                         {},
		"bootstrapsigner":                      {},
		"cloud-node-lifecycle":                 {},
		"cronjob":                              {},
		"csrapproving":                         {},
		"csrsigning":                           {},
		"daemonset":                            {},
		"deployment":                           {},
		"disruption":                           {},
		"endpoint":                             {},
		"endpointslice":                        {},
		"endpointslicemirroring":               {},
		"ephemeral-volume":                     {},
		"garbagecollector":                     {},
		"horizontalpodautoscaling":             {},
		"job":                                  {},
		"legacy-service-account-token-cleaner": {AddedInVersion: "1.28"},
		"namespace":                            {},
		"nodelifecycle":                        {},
		"persistentvolume-binder":              {},
		"persistentvolume-expander":            {},
		"podgc":                                {},
		"pv-protection":                        {},
		"pvc-protection":                       {},
		"replicaset":                           {},
		"replicationcontroller":                {},
		"resource-claim-controller":            {AddedInVersion: "1.27"},
		"resourcequota":                        {},
		"root-ca-cert-publisher":               {},
		"route":                                {},
		"service":                              {},
		"serviceaccount":                       {},
		"serviceaccount-token":                 {},
		"statefulset":                          {},
		"tokencleaner":                         {},
		"ttl":                                  {},
		"ttl-after-finished},":                 {},
	},
	"batch/v1": {
		"cronjob":            {},
		"job":                {},
		"ttl-after-finished": {},
	},
	"authentication/v1": {
		"attachdetach":              {},
		"persistentvolume-expander": {},
	},
	"certificates/v1": {
		"csrapproving": {},
		"csrcleaner":   {},
		"csrsigning":   {},
	},
	"policy/v1": {
		"disruption": {},
	},
	"rbac/v1": {
		"clusterrole-aggregation": {},
	},
	"resource/v1alpha2": {
		"resource-claim-controller": {AddedInVersion: "1.27"},
	},
	"apps/v1": {
		"daemonset":   {},
		"deployment":  {},
		"replicaset":  {},
		"statefulset": {},
	},
	"apiserverinternal/v1alpha1": {
		"storage-version-gc": {},
	},
	"autoscaling/v1": {
		"horizontalpodautoscaling": {},
	},
	"autoscaling/v2": {
		"horizontalpodautoscaling": {},
	},
	"discovery/v1": {
		"endpointslice":          {},
		"endpointslicemirroring": {},
	},
}

// Contains returns true if the range contains the given version, false otherwise.
// The range contains the given version only if it's greater or equal than AddedInVersion (always true if AddedInVersion is empty),
// and less than RemovedInVersion (always true if RemovedInVersion is empty).
func (r *VersionRange) Contains(version string) (bool, error) {
	var constraint string
	switch {
	case r.AddedInVersion != "" && r.RemovedInVersion == "":
		constraint = fmt.Sprintf(">= %s", r.AddedInVersion)
	case r.AddedInVersion == "" && r.RemovedInVersion != "":
		constraint = fmt.Sprintf("< %s", r.RemovedInVersion)
	case r.AddedInVersion != "" && r.RemovedInVersion != "":
		constraint = fmt.Sprintf(">= %s, < %s", r.AddedInVersion, r.RemovedInVersion)
	default:
		constraint = "*"
	}
	return versionutils.CheckVersionMeetsConstraint(version, constraint)
}
