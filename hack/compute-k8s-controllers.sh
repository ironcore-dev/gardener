#!/usr/bin/env bash
#
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e
set -x

usage() {
  echo "Usage:"
  echo "> compute-k8s-controllers.sh [ -h | <old version> <new version> ]"
  echo
  echo ">> For example: compute-k8s-controllers.sh 1.26 1.27"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 2 ]; then
  usage
fi

versions=("$1" "$2")

out_dir=$(mktemp -d)
function cleanup_output {
    rm -rf "$out_dir"
}
trap cleanup_output EXIT

# Define the package name map
declare -A package_map=(
  ["attachdetach"]="attachdetach"
  ["bootstrapsigner"]="bootstrap"
  ["cloud-node-lifecycle"]="cloud"
  ["clusterrole-aggregation"]="clusterroleaggregation"
  ["cronjob"]="cronjob"
  ["csrapproving"]="approver"
  ["csrcleaner"]="cleaner"
  ["csrsigning"]="signer"
  ["daemonset"]="daemon"
  ["deployment"]="deployment"
  ["disruption"]="disruption"
  ["endpoint"]="endpoint"
  ["endpointslice"]="endpointslice"
  ["endpointslicemirroring"]="endpointslicemirroring"
  ["ephemeral-volume"]="ephemeral"
  ["garbagecollector"]="garbagecollector"
  ["horizontalpodautoscaling"]="podautoscaler"
  ["job"]="job"
  ["legacy-service-account-token-cleaner"]="serviceaccount"
  ["namespace"]="namespace"
  ["nodeipam"]="nodeipam"
  ["nodelifecycle"]="nodelifecycle"
  ["persistentvolume-binder"]="persistentvolume"
  ["persistentvolume-expander"]="expand"
  ["podgc"]="podgc"
  ["pv-protection"]="pvprotection"
  ["pvc-protection"]="pvcprotection"
  ["replicaset"]="replicaset"
  ["replicationcontroller"]="replication"
  ["resource-claim-controller"]="resourceclaim"
  ["resourcequota"]="resourcequota"
  ["root-ca-cert-publisher"]="rootcacertpublisher"
  ["route"]="route"
  # ["route"]="staging/src/k8s.io/cloud-provider/controllers/route/route_controller.go"
  ["service"]="service"
  ["serviceaccount"]="serviceaccount"
  ["serviceaccount-token"]="serviceaccount"
  ["statefulset"]="statefulset"
  ["storage-version-gc"]="storageversiongc"
  ["tokencleaner"]="bootstrap"
  ["ttl"]="ttl"
  ["ttl-after-finished"]="ttlafterfinished"
)

kcm_dir="pkg/controller"
ccm_dir="staging/src/k8s.io/cloud-provider/controllers"

for version in "${versions[@]}"; do
  rm -rf "${out_dir}/kubernetes-${version}"
  rm -f "${out_dir}/k8s-controllers-${version}.txt"

  git clone --depth 1 --filter=blob:none --sparse https://github.com/kubernetes/kubernetes -b "release-${version}" "${out_dir}/kubernetes-${version}"
  pushd "${out_dir}/kubernetes-${version}" > /dev/null
  git sparse-checkout set "cmd/kube-controller-manager" "pkg/controller" "staging/src/k8s.io/cloud-provider/controllers"
  popd > /dev/null

  if [ "$version" \< "1.26" ]; then
    names=$(grep -o 'controllers\["[^"]*' "${out_dir}/kubernetes-${version}/cmd/kube-controller-manager/app/controllermanager.go" | awk -F '"' '{print $2}')
    # This is a special controller which is not initialized normally, see https://github.com/kubernetes/kubernetes/blob/99151c39b7d4595632f7745ba7fb4dea4356f7fd/cmd/kube-controller-manager/app/controllermanager.go#L405-L411
    names+=" serviceaccount-token"
  elif [ "$version" \< "1.28" ]; then
    names=$(grep -o 'register("[^"]*' "${out_dir}/kubernetes-${version}/cmd/kube-controller-manager/app/controllermanager.go" | awk -F '"' '{print $2}')
    # This is a special controller which is not initialized normally, see https://github.com/kubernetes/kubernetes/blob/99151c39b7d4595632f7745ba7fb4dea4356f7fd/cmd/kube-controller-manager/app/controllermanager.go#L405-L411
    names+=" serviceaccount-token"
  else
    names=$(grep -E 'func KCMControllerAliases\(\) map\[string\]string \{' "${out_dir}/kubernetes-${version}/cmd/kube-controller-manager/names/controller_names.go" -A 200 | awk -F '[" :]+' '/^		\"[a-zA-Z0-9-]+\"/ {print $2}')
  fi

  for name in $names; do
    if [ ! "${package_map[$name]}" ]; then
      echo "No package mapping found for $name", The controller could have been removed or the package name might have changed.
      echo "Please enhance the map in the script with the correct package name for this controller."
      exit 1
    fi
  done

  unset api_group_controllers
  declare -A api_group_controllers

  for controller in $names; do
    package_name="${package_map[$controller]}"
    
    groups=()
    for dir in "$kcm_dir" "$ccm_dir"; do
      file_path="${out_dir}/kubernetes-${version}/${dir}"
      files+=$(grep -rl "^package $package_name" "$dir")
    
      for file in "$files"; do
        # Find lines containing 'k8s.io/api/' in the file, and extract content after 'k8s.io/api/' up to
        # the next double quote. This will be the API groups used for this controller.
        groups+=$(grep -o 'k8s\.io/api/[^"]*' "$file" | awk -F 'k8s.io/api/' '{print $2}')
      done 
    done

    api_groups=$(echo "${groups[@]}" | tr ' ' '\n' | sort -u)
    
    ## if apigroups are empty, something is missing, error maybe?
    ##
    for api_group in $api_groups; do
      api_group=$(echo "$api_group" | tr -d '[:space:]')
      # Add controller to the corresponding API group key in the map
      if [ -n "$api_group" ]; then
          api_group_controllers["$api_group"]+="$controller "
      fi
    done
  done

  for api_group in "${!api_group_controllers[@]}"; do
    echo "$api_group:$(echo "${api_group_controllers[$api_group]}" | tr ' ' '\n' | sort | tr '\n' ' ')" >> "${out_dir}/k8s-controllers-${version}.txt"
  done
done

cat "${out_dir}/k8s-controllers-${1}.txt"

echo
echo "kube-controller-manager controllers added in $2 compared to $1:"
IFS=$'\n' read -r -d '' -a added_lines < <(diff "${out_dir}/k8s-controllers-$1.txt" "${out_dir}/k8s-controllers-$2.txt" | grep '^>' | sed 's/^> //' && printf '\0')
for added_line in "${added_lines[@]}"; do
  api_group=$(echo "$added_line" | awk -F ': ' '{print $1}')
  controllers=$(echo "$added_line" | awk -F ': ' '{print $2}' | tr ' ' '\n')

  # Find the corresponding line in the other file
  old_line=$(grep "^$api_group: " "${out_dir}/k8s-controllers-$1.txt" | awk -F ': ' '{print $2}' | tr ' ' '\n')

  added_controllers=$(comm -23 <(echo "$controllers" | sort) <(echo "$old_line" | sort) | tr '\n' ' ')

  if [ -n "$added_controllers" ]; then
    echo "Added Controllers for API Group [$api_group]: $added_controllers"
  fi
done

echo
echo "kube-controller-manager controllers removed in $2 compared to $1:"
IFS=$'\n' read -r -d '' -a removed_lines < <(diff "${out_dir}/k8s-controllers-$1.txt" "${out_dir}/k8s-controllers-$2.txt" | grep '^<' | sed 's/^< //' && printf '\0')
for removed_line in "${removed_lines[@]}"; do
  api_group=$(echo "$removed_line" | awk -F ': ' '{print $1}')
  controllers=$(echo "$removed_line" | awk -F ': ' '{print $2}' | tr ' ' '\n')

  # Find the corresponding line in the other file
  new_line=$(grep "^$api_group: " "${out_dir}/k8s-controllers-$2.txt" | awk -F ': ' '{print $2}' | tr ' ' '\n')

  removed_controllers=$(comm -23 <(echo "$controllers" | sort) <(echo "$new_line" | sort) | tr '\n' ' ')

  if [ -n "$removed_controllers" ]; then
    echo "Removed Controllers for API Group [$api_group]: $removed_controllers"
  fi
done
