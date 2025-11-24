/*
Copyright 2025 The Kubernetes Authors.

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

package utils

// MergeMaps copies all key/value pairs from src into dst and returns dst.
// If dst is nil a new map is allocated.
func MergeMaps(dst map[string]string, src map[string]string) map[string]string {
	if src == nil {
		if dst == nil {
			return map[string]string{}
		}
		return dst
	}
	if dst == nil {
		dst = make(map[string]string, len(src))
	}

	for k, v := range src {
		if _, exists := dst[k]; !exists {
			dst[k] = v
		}
	}

	return dst
}
