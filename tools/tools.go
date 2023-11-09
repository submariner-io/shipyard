//go:build tools

/*
SPDX-License-Identifier: Apache-2.0

Copyright Contributors to the Submariner project.

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

// Place any runtime dependencies as imports in this file.
// Go modules will be forced to download and install them.
package tools

import (
	_ "github.com/docker/buildx/cmd/buildx"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/mikefarah/yq/v4"
	_ "github.com/psampaz/go-mod-outdated"
	_ "helm.sh/helm/v3/cmd/helm"
	_ "sigs.k8s.io/kind/cmd/kind"
)
