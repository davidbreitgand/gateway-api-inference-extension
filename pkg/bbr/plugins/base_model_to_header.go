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
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/datastore"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/framework"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/metrics"
	logutil "sigs.k8s.io/gateway-api-inference-extension/pkg/common/observability/logging"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/plugin"
)

const (
	BaseModelToHeaderPluginType = "base-model-to-header"
	// TODO: remove these constants from /bbr/handlers/request.go when the plugin is connected to the handler
	baseModelHeader = "X-Gateway-Base-Model-Name"
	modelField      = "model"
)

// compile-time type validation
var _ framework.RequestProcessor = &BaseModelToHeaderPlugin{}

type BaseModelToHeaderPlugin struct {
	typedName plugin.TypedName
	datastore datastore.Datastore
}

// BaseModelToHeaderPluginFactory defines the factory function for BaseModelToHeaderPlugin
// The name and rawParameters are ignored as the plugin uses a default configuration.
// The name is used to identify the plugin instance name in the configuration.
// Users can configure the plugin via command-line flags: ./bbr --plugin=base-model-to-header:my-base-model-to-header
func BaseModelToHeaderPluginFactory(name string, _ json.RawMessage) (framework.BBRPlugin, error) {
	return NewBaseModelToHeaderPlugin().WithName(name), nil
}

// Returns a concrete *BaseModelToHeaderPlugin.
func NewBaseModelToHeaderPlugin() *BaseModelToHeaderPlugin {
	return &BaseModelToHeaderPlugin{
		typedName: plugin.TypedName{Type: BaseModelToHeaderPluginType, Name: BaseModelToHeaderPluginType},
	}
}

// TypedName returns the type and name tuple of this plugin instance.
func (p *BaseModelToHeaderPlugin) TypedName() plugin.TypedName {
	return p.typedName
}

// WithName sets the name of the default BBR plugin
func (p *BaseModelToHeaderPlugin) WithName(name string) *BaseModelToHeaderPlugin {
	p.typedName.Name = name
	return p
}

// WithDatastore sets the datastore for the BaseModelToHeaderPlugin.
// Currently, only this plugin requires a datastore.
// The datastore is used to store the model metadata.
// The plugin will not function without a datastore.
func (p *BaseModelToHeaderPlugin) WithDatastore(ds datastore.Datastore) *BaseModelToHeaderPlugin {
	p.datastore = ds
	return p
}

// Execute is the entrypoint for the BaseModelToHeaderPlugin.
func (p *BaseModelToHeaderPlugin) Execute(ctx context.Context, headers map[string]string, body map[string]any) (map[string]string, map[string]any, error) {
	// Check arguments
	if headers == nil {
		return nil, nil, errors.New("headers map is nil")
	}
	// Check that body map is not nil
	if body == nil {
		return nil, nil, errors.New("body map is nil")
	}

	// Extract model name from request body
	targetModel, ok := body[modelField].(string)
	if !ok || targetModel == "" {
		// No model specified, return unchanged, raise error
		return headers, body, errors.New("no model in the body")
	}

	// Look up base model using datastore
	if p.datastore != nil {
		baseModel := p.datastore.GetBaseModel(targetModel)
		if baseModel != "" {
			// Set model headers for routing
			headers[baseModelHeader] = baseModel
		}
		// Return updated headers and original body
		return headers, body, nil
	}
	// No datastore: return headers and body unchanged, and raise an error
	return headers, body, errors.New("no datastore configured")
}

// ProcessRequest sets base model name on the header
func (p *BaseModelToHeaderPlugin) ProcessRequest(ctx context.Context, request *framework.InferenceRequest) error {
	if request == nil || request.Headers == nil || request.Body == nil {
		return nil // this shouldn't happen
	}

	// extract raw field value from body
	rawFieldValue, exists := request.Body[modelField]
	if !exists {
		metrics.RecordBodyFieldNotFound(modelField)
		return fmt.Errorf("field '%s' not found in request body", modelField)
	}

	targetModel := fmt.Sprintf("%v", rawFieldValue) // convert any type to string
	if targetModel == "" {
		metrics.RecordBodyFieldEmpty(modelField)
		return fmt.Errorf("field '%s' is empty and couldn't be processed", modelField)
	}

	// Look up base model using configured datastore
	if p.datastore != nil {
		baseModel := p.datastore.GetBaseModel(targetModel)
		if baseModel != "" {
			// Set model headers for routing
			request.SetHeader(baseModelHeader, baseModel)
			log.FromContext(ctx).V(logutil.VERBOSE).Info("parsed field from body", "field", modelField, "value", targetModel)
			return nil
		}
		return fmt.Errorf("could not find base model for %s", targetModel)
	}
	// No datastore configured - this indicates a configuration error during plugin initialization
	return errors.New("datastore not configured")
}
