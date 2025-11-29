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

package framework

import (
	"github.com/openai/openai-go/v3" //imported as openai
	bbrplugins "sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/framework/plugins"
)

// placeholder for Plugin constructors
type PluginFactoryFunc func() bbrplugins.BBRPlugin //any no-argument function that returns bbrplugins.BBRPlugin can be assigned to this type (including a constructor function)

// ----------------------- Registry Interface --------------------------------------------------
// PluginRegistry defines operations for managing plugin factories and plugin instances
type PluginRegistry interface {
	RegisterFactory(typeKey string, factory PluginFactoryFunc) error //constructors
	RegisterPlugin(plugin bbrplugins.BBRPlugin) error                //registers a plugin instance (the instance is supposed to be created via the factory first)
	GetFactory(typeKey string) (PluginFactoryFunc, error)
	GetPlugin(typeKey string) (bbrplugins.BBRPlugin, error)
	GetFactories() map[string]PluginFactoryFunc
	GetPlugins() map[string]bbrplugins.BBRPlugin
	UnregisterPlugin(typeKey string) error
	UnregisterFactory(typeKey string) error
	ListPlugins() []string
	ListFactories() []string
	CreatePlugin(typeKey string) (bbrplugins.BBRPlugin, error)
	ContainsFactory(typeKey string) bool
	ContainsPlugin(typeKey string) bool
	String() string
	Clear()
}

// ------------------------ Ordered Plugins Interface  ------------------------------------------
// PluginsChain is used to define a specific order of execution of the plugin instances stored in the registry
type PluginsChain interface {
	AddPlugin(typeKey string, registry PluginRegistry) error                    //to be added to the chain the plugin should be registered in the registry first
	AddPluginAtInd(typeKey string, i int, r PluginRegistry) error               //only affects the instance of the plugin chain
	DeletePlugin(name string) error                                             //delete only deletes the plugin from a chain instance, but not from the registry
	GetPlugin(index int, registry PluginRegistry) (bbrplugins.BBRPlugin, error) //retrieves i-th plugin as defined in the chain from the registry
	Length() int
	ParseChatCompletion(data []byte) (openai.ChatCompletionNewParams, error)
	ParseCompletion(data []byte) (openai.CompletionNewParams, error)
	GetSharedChatCompletion() openai.ChatCompletionNewParams
	GetSharedCompletion() openai.CompletionNewParams
	GetSharedMemory(which string) interface{}
	String() string
}

// ------------------------- Supported plugin interfaces -----------------------------------------
// Extend this map when new interfaces and/or implementations are added
var SupportedInterfaces = map[string][]string{
	"ModelSelector":     {"semantic-model-selector"},
	"GuardRail":         {"bad-words-blocker", "pid-disclosure-blocker"},
	"MetadataExtractor": {"simple-model-extractor"},
}
