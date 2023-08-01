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

for dir in $(yq '(.updates[] | select(.package-ecosystem == "gomod")).directory' "$conffile"); do

    (cd "${base}${dir}" || exit

     # Count the ignores
     ignores="$(yq '(.updates[] | select(.package-ecosystem == "gomod") | select(.directory == "'"$dir"'") | .ignore | length' "$conffile")"
     firstauto=0

     # Look for the start of automated ignores
     for (( i=0; i < ignores; i++ )); do
         if [ "$(yq '(.updates[] | select(.package-ecosystem == "gomod") | select(.directory == "'"$dir"'") | .ignore['"$i"'].dependency-name' "$conffile")" = "github.com/submariner-io/*" ]; then
             firstauto=$i
             break
         fi
     done

     # Remove the existing automated ignores
     while [ "$(yq '(.updates[] | select(.package-ecosystem == "gomod") | select(.directory == "'"$dir"'") | .ignore | length' "$conffile")" -gt "$firstauto" ]; do
         yq -i -P 'del(.updates[] | select(.package-ecosystem == "gomod") | select(.directory == "'"$dir"'") | .ignore['"$firstauto"'])' "$conffile"
     done

     # "See" remaining ignores
     read -ar seendeps < <(yq '(.updates[] | select(.package-ecosystem == "gomod") | select(.directory == "'"$dir"'") | .ignore[].dependency-name)' "$conffile")

     # Restore the submariner-io exclusion
     yq -i -P '(.updates[] | select(.package-ecosystem == "gomod") | select(.directory == "'"$dir"'")).ignore['"$firstauto"'].dependency-name = "github.com/submariner-io/*"' "$conffile"
     yq -i -P '(.updates[] | select(.package-ecosystem == "gomod") | select(.directory == "'"$dir"'")).ignore['"$firstauto"'] head_comment = "Our own dependencies are handled during releases"' "$conffile"

     # Ignore all parent dependencies
     for parent in $(GOWORK=off go list -m -mod=mod -json all | jq -r 'select(.Path | contains("/submariner-io/")) | select(.Main != true) .Path | gsub("github.com/submariner-io/"; "")'); do
         first=true
         for dep in $(GOWORK=off go list -m -mod=mod -json all | jq -r 'select(.Path | contains("/submariner-io") | not) | select(.Indirect != true) | select(.Main != true) .Path'); do
             if ! depseen "$dep"; then
                 if grep -q "$dep" "$base/../$parent/go.mod" && ! grep -q "$dep .*// indirect" "$base/../$parent/go.mod"; then
                     yq -i -P '(.updates[] | select(.package-ecosystem == "gomod") | select(.directory == "'"$dir"'")).ignore += { "dependency-name": "'"$dep"'" }' "$conffile"
                     if $first; then
                         yq -i -P 'with(.updates[] | select(.package-ecosystem == "gomod") | select(.directory == "'"$dir"'"); .ignore[.ignore | length - 1] head_comment = "Managed in '"$parent"'")' "$conffile"
                         first=false
                     fi
                     seendeps+=("$dep")
                 fi
             fi
         done
     done)

done
