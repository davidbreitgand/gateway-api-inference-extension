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
)

// HandlePlugin defines a set of APIs to work with an instantiated plugin
type HandlePlugin interface {
	// AddPlugin adds a plugin
	AddBBRPlugin(name string, plugin BBRPlugin)

	// GetBBRPlugin returns a BBRPlugin instance by name
	GetBBRPlugin(name string) BBRPlugin
}

// Handle provides a standard set of tools to the plugin instance
type Handle interface {
	HandlePlugin
	Context() context.Context
}

// bbrHandle implements HandlePlugin interface
type bbrHandle struct {
	ctx context.Context
	HandlePlugin
}

// Context returns a context that might be required by a plugin
func (h *bbrHandle) Context() context.Context {
	return h.ctx
}

// bbrHandlePlugin implements HandlePlugin APIs to work with an instantiated plugin
type bbrHandlePlugin struct {
	plugins map[string]BBRPlugin
}

// AddBBRPlugin adds an instance of BBRPlugin. In this implementation, only one plugin instance can be added.
func (h *bbrHandlePlugin) AddBBRPlugin(name string, plugin BBRPlugin) {
	h.plugin = plugin
}

// GetBBRPlugin gets an instance of BBRPlugin
// The caller should always check for nil. There is no validly nil condition.
func (h *bbrHandlePlugin) GetBBRPlugin(name string) BBRPlugin {
	return h.plugins[name]
}

// Constructor for bbrHandle
func NewBbrHandle(ctx context.Context) Handle {
	return &bbrHandle{
		ctx:          ctx,
		HandlePlugin: &bbrHandlePlugin{},
	}
}
