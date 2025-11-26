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

package bbrplugins

import (
	"context"

	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/plugins"
)

// ---------------------------- Well-known custom headers ---------------------------------
const ModelHeader = "X-Gateway-Model-Name"

// ----------------------------------------------------------------------------------------
// ------------------------------------ Defaults ------------------------------------------
const DefaultPluginType = "MetadataExtractor"
const DefaultPluginImplementation = "simple-model-selector"

// BBR plugins should never mutate the body directly.
// If the body is mutated to have an alignment with the headers wherever
// required (e.g., model and ModelHeader), the mutated body should be returned separateley
type BBRPlugin interface {
	plugins.Plugin
	RequiresFullParsing() bool //specifies whether a full parsing of the body is required (to facilitate efficient memory sharing across plugins in a plugins chain)
}

// plugins implementing the MetadataExtractor interface should be read-only.
type MetadataExtractor interface {
	BBRPlugin
	Extract(ctx context.Context,
		requestBodyBytes []byte,
		metaDataKeys []string,
		sharedMemory interface{}) (headers map[string]string, err error) //shared memory can be either openai.ChatCompletionNewParams or openai.CompletionNewParams struct
}

// plugins implementing the ModelSelector interface should always return mutatedBodyBytes even if the body
// is not mutatted: in this case, the original bodyBytes are returned
type ModelSelector interface {
	BBRPlugin
	Select(ctx context.Context, requestBodyBytes []byte, sharedMemory interface{}) (
		headers map[string]string,
		mutatedBodyBytes []byte, err error)
}
