#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
#
# Copyright Contributors to the Submariner project.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Adds ignore statements to .github/dependabot.yml for direct dependencies
# already present in parent Submariner projects (i.e. projects which the
# processed project depends on).
#
# Existing entries are removed, starting with the first entry marked
# "Our own dependencies are handled during releases" (if none, all
# entries are removed). This tool assumes that all Submariner projects
# are checked out alongside each other (i.e. ../shipyard contains
# Shipyard, ../admiral contains Admiral etc.).
#
# By default, the default branch is processed; specify another branch
# as argument to handle that instead.

declare -a seendeps

function depseen() {
    local dep
    for dep in "${seendeps[@]}"; do
        if [ "$dep" = "$1" ]; then
            return 0
        fi
    done
    return 1
}

base="$(pwd)"
conffile="${base}/.github/dependabot.yml"

branch="${1:-null}"

gomodfilter=".updates[] | select(.package-ecosystem == \"gomod\")"

for dir in $(yq "(${gomodfilter}).directory" "$conffile"); do

    dirfilter="${gomodfilter} | select(.directory == \"${dir}\")"

    # Looping over branch is pointless since dependencies need to track
    # (i.e. other repos need to be in the same branch)

    if [ "$branch" = "null" ]; then
        branchfilter="${dirfilter} | select(has(\"target-branch\") | not)"
    else
        branchfilter="${dirfilter} | select(.target-branch == \"$branch\")"
    fi

    (cd "${base}${dir}" || exit

     # Count the ignores
     ignores="$(yq "(${branchfilter}).ignore | length" "$conffile")"
     firstauto=0

     # Look for the start of automated ignores
     for (( i=0; i < ignores; i++ )); do
         if [ "$(yq "(${branchfilter}).ignore[$i].dependency-name" "$conffile")" = "github.com/submariner-io/*" ]; then
             firstauto=$i
             break
         fi
     done

     # Remove the existing automated ignores
     while [ "$(yq "(${branchfilter}).ignore | length" "$conffile")" -gt "$firstauto" ]; do
         yq -i -P "del(${branchfilter}.ignore[$firstauto])" "$conffile"
     done

     # "See" remaining ignores
     read -ar seendeps < <(yq "(${branchfilter}).ignore[].dependency-name" "$conffile")

     # Restore the submariner-io exclusion
     yq -i -P "(${branchfilter}).ignore[$firstauto].dependency-name = \"github.com/submariner-io/*\"" "$conffile"
     yq -i -P "(${branchfilter}).ignore[$firstauto] head_comment = \"Our own dependencies are handled during releases\"" "$conffile"

     # Ignore all parent dependencies
     for parent in $(GOWORK=off go list -m -mod=mod -json all | jq -r 'select(.Path | contains("/submariner-io/")) | select(.Main != true) .Path | gsub("github.com/submariner-io/"; "")'); do
         first=true
         for dep in $(GOWORK=off go list -m -mod=mod -json all | jq -r 'select(.Path | contains("/submariner-io") | not) | select(.Indirect != true) | select(.Main != true) .Path'); do
             if ! depseen "$dep"; then
                 if grep -q "$dep" "$base/../$parent/go.mod" && ! grep -q "$dep .*// indirect" "$base/../$parent/go.mod"; then
                     yq -i -P "(${branchfilter}).ignore += { \"dependency-name\": \"$dep\" }" "$conffile"
                     if $first; then
                         yq -i -P "with(${branchfilter}; .ignore[.ignore | length - 1] head_comment = \"Managed in $parent\")" "$conffile"
                         first=false
                     fi
                     seendeps+=("$dep")
                 fi
             fi
         done
     done)

done
