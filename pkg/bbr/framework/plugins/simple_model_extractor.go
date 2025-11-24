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

	"encoding/json"

	"fmt"

	"github.com/openai/openai-go/v3"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/plugins"
)

// ------------------------------------  INTERFACES ---------------------------------------------------------------
// Interfaces are defined in "sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/framework/plugins/interfaces.go"
// ----------------------------------------------------------------------------------------------------------------

// ------------------------------------ SAMPLE PLUGIN IMPLEMENTATION ----------------------------------------------

const ModelHeaderKey = "model"

type simpleModelExtractor struct { //implements the MetadataExtractor interface
	typedName           plugins.TypedName
	requiresFullParsing bool
}

// Constructor
func NewSimpleModelExtractor() MetadataExtractor {
	return &simpleModelExtractor{
		typedName: plugins.TypedName{
			Type: "MetadataExtractor",
			Name: "simple-model-extractor",
		},
		requiresFullParsing: false,
	}
}

func (s *simpleModelExtractor) RequiresFullParsing() bool {
	return s.requiresFullParsing
}

func (s *simpleModelExtractor) TypedName() plugins.TypedName {
	return s.typedName
}

//A canonical BBRPlugin implementation that takes advantage of the shared memory,
// would have teh following structure:
// 0. Make sure that the plugin works on a copy if it writes
// 1. Do type assertion of the shared memory struct
// 2. Implement a check whether the received shared memory is non-empty
// 3. If it is non-empty, use this structure in the business logic of the plugin
//    Note, that the plugin can ensure that the shared memory is non-empty by setting
//    requiresFullParsing = true
//	  However, this pattern is not encouraged if the plugin does not require a full body
//	  Rather, in this case, it should attempt to use shared memory and only if empty, resort to
//	  its custom implementation
// 4. If the shared memory is empty, then it means that all plugins use custom structs for efficiency

// This implementation is constrained to extract the model metadata key only and set the X-Gateway-Model-Name header only
// Thus, this is simply refactoring of the default BBR implementation to work with the pluggable framework
func (s *simpleModelExtractor) Extract(ctx context.Context,
	requestBodyBytes []byte,
	metaDataKeys []string, //in this implementation, the metaDataKeys are ignored, because the plugin only extracts Model
	sharedMemory interface{}) (headers map[string]string, err error) {

	h := map[string]string{}

	defaultImpl := func() (map[string]string, error) { //the default implementation that does not use shared memory for efficiency if no other plugin uses shared memory
		type RequestBody struct {
			Model string `json:"model"`
		}
		var requestBody RequestBody
		if err := json.Unmarshal(requestBodyBytes, &requestBody); err != nil {
			return nil, err
		}
		h[ModelHeaderKey] = requestBody.Model
		return h, nil
	}

	switch sharedMem := sharedMemory.(type) {
	case openai.ChatCompletionNewParams:
		if len(sharedMem.Model) > 0 {
			h[ModelHeaderKey] = string(sharedMem.Model)
			return h, nil
		}
		return defaultImpl()
	case openai.CompletionNewParams:
		if len(sharedMem.Model) > 0 {
			h[ModelHeaderKey] = string(sharedMem.Model)
			return h, nil
		}
		return defaultImpl()
	default:
		return nil, fmt.Errorf("unsupported shared memory type: %T", sharedMem)
	}
}
