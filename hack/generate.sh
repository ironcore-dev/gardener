#!/usr/bin/env bash
#
# Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

WHAT="protobuf codegen manifests logcheck gomegacheck monitoring-docs"
WHICH=""
MODE="parallel"
AVAILABLE_CODEGEN_OPTIONS=(
  "authentication_groups"
  "core_groups"
  "extensions_groups"
  "resources_groups"
  "operator_groups"
  "seedmanagement_groups"
  "operations_groups"
  "settings_groups"
  "operatorconfig_groups"
  "controllermanager_groups"
  "admissioncontroller_groups"
  "scheduler_groups"
  "gardenlet_groups"
  "resourcemanager_groups"
  "shoottolerationrestriction_groups"
  "shootdnsrewriting_groups"
  "provider_local_groups"
  "extensions_config_groups"
)
AVAILABLE_MANIFESTS_OPTIONS=(
  "charts"
  "cmd"
  "example"
  "extensions"
  "pkg"
  "plugin"
  "test"
)

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
      --what)
        shift
        WHAT="${1:-$WHAT}"
        ;;
      --mode)
        shift
        if [[ -n "$1" ]]; then
        MODE="$1"
        fi
        ;;
      --which)
        shift
        WHICH="${1:-$WHICH}"
        ;;
      *)
        echo "Unknown argument: $1"
        exit 1
        ;;
    esac
    shift
  done
}

overwrite_paths() {
  local options=("$@")
  local updated_paths=()

  for option in "${options[@]}"; do
    updated_paths+=("./$option/...")
  done

  echo "${updated_paths[*]}"
}

validate_options() {
    local which=()
    IFS=' ' read -ra which <<< "$1"

    local available_options=("${!2}")
    local valid_options=()
    local invalid_options=()

    for option in "${which[@]}"; do
        valid=false

        for valid_option in "${available_options[@]}"; do
            if [[ "$option" == "$valid_option" ]]; then
                valid=true
                break
            fi
        done

        if $valid; then
            valid_options+=("$option")
        else
            invalid_options+=("$option")
        fi
    done

    echo "${valid_options[*]}:${invalid_options[*]}"
}

run_target() {
  local target=$1
  case "$target" in
    protobuf)
      $REPO_ROOT/hack/update-protobuf.sh
      ;;
    codegen)
      IFS=' ' read -ra available_options <<< "${AVAILABLE_CODEGEN_OPTIONS[@]}"
      if [[ -z "$WHICH" ]]; then
        WHICH=("${available_options[@]}")
        valid_options=("${available_options[@]}")
      else
        result=$(validate_options "$WHICH" available_options[@])
        IFS=':' read -ra results_array <<< "$result"
        
        valid_options=("${results_array[0]}")
        invalid_options=("${results_array[1]}")


        valid_options=("${valid_options[@]##*( )}")
        valid_options=("${valid_options[@]%%*( )}")

        invalid_options=("${invalid_options[@]##*( )}")
        invalid_options=("${invalid_options[@]%%*( )}")


        echo "Validation result: $result"
        echo "valid options: ${valid_options[*]}"
        echo "invalid options: ${invalid_options[*]}"
        
        trimmed_invalid_options="${invalid_options[*]// /}"

        if [[ ${#invalid_options[@]} -gt 0 ]]; then
            printf "\nSkipping invalid options: %s, Available options are: %s\n\n" "${invalid_options[*]}" "${available_options[*]}"
        fi
      fi

      
      if [[ ${#valid_options[@]} -gt 0 ]]; then  
        printf "> Running codegen for groups: ${valid_options[*]}\n\n"
        $REPO_ROOT/hack/update-codegen.sh --which "$valid_options" --mode "$MODE"
      fi
      ;;
    manifests)
      IFS=' ' read -ra available_options <<< "${AVAILABLE_MANIFESTS_OPTIONS[@]}"
      if [[ -z "$WHICH" ]]; then
        WHICH=("${available_options[@]}")
        valid_options=("${available_options[@]}")
      else
        result=$(validate_options "$WHICH" available_options[@])
        IFS=':' read -ra results_array <<< "$result"
        
        valid_options=("${results_array[0]}")
        invalid_options=("${results_array[1]}")
        
        if [[ ${#invalid_options[@]} -gt 0 ]]; then
            printf "\nSkipping invalid options: %s, Available options are: %s\n\n" "${invalid_options[*]}" "${available_options[*]}"
        fi
      fi

      printf "> Generating manifests for folders: ${valid_options[*]}\n\n"

      if [[ ${#valid_options[@]} -gt 0 ]]; then  
        if [[ "$MODE" == "sequential" ]]; then
          # In sequential mode, paths need to be converted to go package notation (e.g., ./charts/...)
          overwrite_paths "${valid_options[@]}"
          $REPO_ROOT/hack/generate-sequential.sh ${valid_options[@]}
        else
          $REPO_ROOT/hack/generate-parallel.sh ${valid_options[@]}
        fi
      fi
      ;;
    logcheck)
      cd "$REPO_ROOT/$LOGCHECK_DIR" && go generate ./...
      ;;
    gomegacheck)
      cd "$REPO_ROOT/$GOMEGACHECK_DIR" && go generate ./...
      ;;
    monitoring-docs)
      $REPO_ROOT/hack/generate-monitoring-docs.sh
      ;;
    *)
      printf "Unknown target: $target. Available targets are 'protobuf', 'codegen', 'manifests', 'logcheck', 'gomegacheck', 'monitoring-docs'.\n\n"
      ;;
  esac
}

parse_flags "$@"

IFS=' ' read -ra TARGETS <<< "$WHAT"
for target in "${TARGETS[@]}"; do
  run_target "$target"
done
