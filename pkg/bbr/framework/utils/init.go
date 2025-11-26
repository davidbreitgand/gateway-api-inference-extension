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

// Initializes PluginRegistry from environment variables

package utils

import (
	"fmt"
	"os"
	"strings"

	"sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/framework"
	bbrplugins "sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/framework/plugins"
)

func InitPlugins() (
	*framework.PluginRegistry,
	*framework.PluginsChain,
	*framework.PluginsChain,
	[]string,
	error) {

	//The environment variables defining plugins repertoire and plugin chains should be set in ConfigMap (used as part of the BBR Helm chart)

	registry := framework.NewPluginRegistry()
	requestChain := framework.NewPluginsChain()
	responseChain := framework.NewPluginsChain()
	var metaDataKeys = []string{}

	// Read metadata keys (the metadata keys should be defined if MetaDataExtractor is specified)
	// If no plugin is specified for PluginsRequestChain, then, by default, there always will be specified
	// metadata key "model" and an instance of SimpleModelExtractor of type MetaDataExtractor will be added to the RequestPluginsChain
	metaKeysEnv := os.Getenv("METADATA_KEYS")
	if metaKeysEnv != "" {
		metaDataKeys = strings.Split(metaKeysEnv, ",")
		for i := range metaDataKeys {
			metaDataKeys[i] = strings.TrimSpace(metaDataKeys[i])
		}
	} else {
		metaDataKeys = []string{"model"} // default
	}

	//helper to process plugins
	processPlugin := func(pluginType string, implementation string, chain framework.PluginsChain) error {
		//create the plugin instance
		plugin, err := registry.CreatePlugin(pluginType)
		if err != nil {
			return fmt.Errorf("failed to create an instance of %s/%s %v", pluginType, implementation, err)
		}
		//register the plugin instance
		err = registry.RegisterPlugin(plugin)
		if err != nil {
			return fmt.Errorf("failed to register an instance of %s/%s %v", pluginType, implementation, err)
		}
		chain.AddPlugin(pluginType, registry)
		return nil
	}

	// Helper to process plugin chains
	processChain := func(envVar string, chain framework.PluginsChain) error {
		configData := os.Getenv(envVar)
		if configData == "" {
			return nil // no plugins defined for this chain, but this is not an error
		}

		lines := strings.Split(configData, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			kvPair := strings.SplitN(line, ":", 2)
			if len(kvPair) != 2 {
				return fmt.Errorf("malformed plugin definition in %s: %s", envVar, line)
			}

			pluginType := strings.TrimSpace(kvPair[0])
			implementation := strings.TrimSpace(kvPair[1])

			switch {
			case pluginType == "MetadaExtractor":
				switch {
				case implementation == "simple-model-extractor":
					//register factory for this plugin
					if err := registry.RegisterFactory(pluginType,
						func() bbrplugins.BBRPlugin {
							return bbrplugins.NewSimpleModelExtractor()
						}); err != nil {
						return fmt.Errorf("failed to register factory for %s/%s %v", pluginType, implementation, err)
					}
					//process this plugin: create an instance, register it in a registry , and add to plugin chain by name
					if err := processPlugin(pluginType, implementation, chain); err != nil {
						return fmt.Errorf("failed to process %s/%s", pluginType, implementation)
					}
				default:
					continue
				}
			case pluginType == "ModelSelector": //TBA
				switch {
				case implementation == "semantic-model-selector":
					//register factory
					//process plugin
					continue
				default:
					continue
				}
			}
			continue
		}
		return nil
	}

	// Process both chains
	if err := processChain("REQUEST_PLUGINS_CHAIN", requestChain); err != nil {
		return nil, nil, nil, nil, err
	}
	if err := processChain("RESPONSE_PLUGINS_CHAIN", responseChain); err != nil {
		return nil, nil, nil, nil, err
	}

	// If request chain is empty, add default MetadataExtractor
	if requestChain.Length() == 0 {
		//use default plugin
		err := registry.RegisterFactory(bbrplugins.DefaultPluginType,
			func() bbrplugins.BBRPlugin {
				return bbrplugins.NewSimpleModelExtractor()
			})
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to register factory %v", err)
		}
		err = processPlugin(bbrplugins.DefaultPluginType, bbrplugins.DefaultPluginImplementation, requestChain)

		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to create default MetaDataExtractor: %v", err)
		}
	}

	return &registry, &requestChain, &responseChain, metaDataKeys, nil
}
