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

package plugins

import (
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/plugins"
)

const (
	ModelHeader     = "X-Gateway-Model-Name"
	BaseModelHeader = "X-Gateway-Base-Model-Name"
)

type BBRPlugin interface {
	plugins.Plugin

	// Execute runs the plugin logic on the request body and a map of headers.
	// A plugin's imnplementation logic CAN mutate the body of the message.
	// A plugin's implementation MUST return a map of headers.
	// If no headers are set by the implementation, the return headers map is nil.
	// A value of a header in an extended implementation NEED NOT to be identical to the value of that same header as would be set
	// in a default implementation.
	// Example: in the body of a request model is set to "semantic-model-selector",
	// which, say, stands for "select a best model for this request at minimal cost"
	// A plugin implementation of "semantic-model-selector" sets X-Gateway-Model-Name to any valid
	// model name from the inventory of the backend models and also mutates the body accordingly
	// If the body is not mutated, the original request body MUST be returned.
	MutateHeadersAndBody(requestBodyBytes []byte, requestHeaders map[string][]string) (headers map[string][]string, mutatedBodyBytes []byte, err error)
}
