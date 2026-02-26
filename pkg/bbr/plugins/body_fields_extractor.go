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

	"sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/framework"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/plugin"
)

const (
	BodyFieldToHeaderPluginType = "body-field-to-header"
)

// compile-time type validation
var _ framework.PayloadProcessor = &BodyFieldToHeaderPlugin{}

// BodyFieldToHeaderPlugin extracts values from request body fields and sets them as HTTP headers.
// All headers are treated as custom headers.
//
// Configuration of the plugin is done via CLI flag: --plugin <type>:<name>:<json>
// Example:
//
//	--plugin body-field-to-header:my-field-extractor:'{"headers":{"X-Model-Name":"model","X-Provider":"provider","Some-Header":"some_field"}}'
//
// Where:
//   - "body-field-to-header" is the plugin type (BodyFieldToHeaderPluginType constant)
//   - "my-field-extractor" is the plugin instance name that can be any string
//   - The JSON object defines the field-to-header mappings
//
// If the request body contains:
//
//	{"model": "gpt-4", "provider": "openai", "some_field": "some-value"}
//
// The plugin will set headers exactly as configured:
//   - X-Model-Name: gpt-4
//   - X-Provider: openai
//   - Some-Header: some-value
type BodyFieldToHeaderPlugin struct {
	typedName plugin.TypedName
	// headerToFieldMap maps header names to body field names
	headerToFieldMap map[string]string
}

// BodyFieldToHeaderConfig defines the JSON configuration structure for the plugin.
type BodyFieldToHeaderConfig struct {
	// Headers maps header names to body field names
	Headers map[string]string `json:"headers"`
}

// BodyFieldToHeaderPluginFactory creates a new BodyFieldToHeaderPlugin instance from JSON configuration.
func BodyFieldToHeaderPluginFactory(name string, rawParameters json.RawMessage) (framework.PayloadProcessor, error) {
	var config BodyFieldToHeaderConfig

	if len(rawParameters) > 0 {
		if err := json.Unmarshal(rawParameters, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal BodyFieldToHeader plugin configuration: %w", err)
		}
	}

	if len(config.Headers) == 0 {
		return nil, errors.New("BodyFieldToHeader plugin requires at least one header mapping in configuration")
	}

	return NewBodyFieldToHeaderPlugin(name, config.Headers), nil
}

// NewBodyFieldToHeaderPlugin creates a new BodyFieldToHeaderPlugin with the given configuration.
func NewBodyFieldToHeaderPlugin(name string, headerToFieldMap map[string]string) *BodyFieldToHeaderPlugin {
	if name == "" {
		name = BodyFieldToHeaderPluginType
	}

	return &BodyFieldToHeaderPlugin{
		typedName: plugin.TypedName{
			Type: BodyFieldToHeaderPluginType,
			Name: name,
		},
		headerToFieldMap: headerToFieldMap,
	}
}

// TypedName returns the type and name tuple of this plugin instance.
func (p *BodyFieldToHeaderPlugin) TypedName() plugin.TypedName {
	return p.typedName
}

// WithName sets the name of the plugin instance.
func (p *BodyFieldToHeaderPlugin) WithName(name string) *BodyFieldToHeaderPlugin {
	p.typedName.Name = name
	return p
}

// Execute extracts value from a given body field and sets it as HTTP header.
func (p *BodyFieldToHeaderPlugin) Execute(ctx context.Context, headers map[string]string, body map[string]any) (map[string]string, map[string]any, error) {
	// Create a new headers map with only the extracted values from the body
	updatedHeaders := make(map[string]string, len(p.headerToFieldMap))

	// Extract field values from body and set them as headers
	for headerName, fieldName := range p.headerToFieldMap {
		fieldValue, exists := body[fieldName]
		// Missing body fields are skipped and header is not set
		if !exists {
			continue
		}

		// Extract string value - body fields can be strings, numbers, booleans, etc.
		// Use type assertion for strings, fmt.Sprintf for other types
		var headerValue string
		if str, ok := fieldValue.(string); ok {
			headerValue = str
		} else {
			// Convert non-string types (numbers, booleans, etc.) to string
			headerValue = fmt.Sprintf("%v", fieldValue)
		}

		// Set the header value exactly as configured
		updatedHeaders[headerName] = headerValue
	}

	// Return updated headers and unchanged body
	return updatedHeaders, body, nil
}
