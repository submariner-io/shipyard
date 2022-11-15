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

package framework

import (
	"k8s.io/apimachinery/pkg/api/errors"
)

// identify API errors which could be considered transient/recoverable
// due to server state.
func IsTransientError(err error, opMsg string) bool {
	if errors.IsInternalError(err) ||
		errors.IsServerTimeout(err) ||
		errors.IsTimeout(err) ||
		errors.IsServiceUnavailable(err) ||
		errors.IsUnexpectedServerError(err) ||
		errors.IsTooManyRequests(err) {
		Logf("Transient failure when attempting to %s: %v", opMsg, err)
		return true
	}

	return false
}
