/*
Copyright 2026 The Kubernetes Authors.

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
	"context"
	"encoding/json"

	"sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/datastore"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/framework"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/plugin"
)

const (
	ModelExtractorType = "model-to-header"
	// TODO: remove these constants from /bbr/handlers/request.go when the plugin is connected to the handler
	modelHeader     = "X-Gateway-Model-Name"
	baseModelHeader = "X-Gateway-Base-Model-Name"
	modelField      = "model"
)

// compile-time type validation
var _ framework.PayloadProcessor = &ModelExtractorPlugin{}

type ModelExtractorPlugin struct {
	typedName plugin.TypedName
	datastore datastore.Datastore
}

// ModelExtractorPluginFactory defines the factory function for ModelExtractorPlugin.
// The name and rawParameters are ignored as the plugin uses a default configuration.
// The name is used to identify the plugin in the configuration.
// Users can configure the plugin via command-line flags: ./bbr --plugin=model-to-header
func ModelExtractorPluginFactory(name string, _ json.RawMessage) (framework.PayloadProcessor, error) {
	return NewModelExtractorPlugin().WithName(name), nil
}

// ModelExtractorPlugin returns a concrete *ModelExtractorPlugin.
func NewModelExtractorPlugin() *ModelExtractorPlugin {
	return &ModelExtractorPlugin{
		typedName: plugin.TypedName{Type: ModelExtractorType, Name: ModelExtractorType},
	}
}

// TypedName returns the type and name tuple of this plugin instance.
func (p *ModelExtractorPlugin) TypedName() plugin.TypedName {
	return p.typedName
}

// WithName sets the name of the default BBR plugin
func (p *ModelExtractorPlugin) WithName(name string) *ModelExtractorPlugin {
	p.typedName.Name = name
	return p
}

// WithDatastore sets the datastore for the ModelExtractorPlugin.
// Currently, only this plugin requires a datastore.
// The datastore is used to store the model metadata.
// The plugin will not function without a datastore.
func (p *ModelExtractorPlugin) WithDatastore(ds datastore.Datastore) *ModelExtractorPlugin {
	p.datastore = ds
	return p
}

// Execute is the entrypoint for the ModelExtractorPlugin.
func (p *ModelExtractorPlugin) Execute(ctx context.Context, headers map[string]string, body map[string]any) (map[string]string, map[string]any, error) {
	// Extract model name from request body
	targetModel, ok := body[modelField].(string)
	if !ok || targetModel == "" {
		// No model specified, return unchanged
		return headers, body, nil
	}
	// Set original target model for header routing
	headers[modelHeader] = targetModel

	// Look up base model using datastore
	if p.datastore != nil {
		baseModel := p.datastore.GetBaseModel(targetModel)
		if baseModel != "" {
			// Set model headers for routing
			headers[baseModelHeader] = baseModel
		}
	}
	// Return updated headers and original body
	return headers, body, nil
}
