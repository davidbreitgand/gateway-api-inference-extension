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

package basemodelextractor

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"
)

const (
	baseModelKey = "baseModel"
	adaptersKey  = "adapters"
)

// The adaptersStore stores a mapping between a LoRA adapter and its corresponding base model.
// Each ConfigMap object stores a mapping between base model and a set of LoRA adapters and adaptersStore translates this into an in-memory cache.
// In case the number of LoRA adapters is extremely large, it's possible to use multiple ConfigMap objects, each storing a slice of the mapping.
// Base models are stored in a separate hash map, due to the fact that the same base model can have multiple configmap objects
// (each may store a subset of the LoRA adapters because of CR scale limitations).
type adaptersStore interface {
	configMapUpdateOrAddIfNotExist(configmap *corev1.ConfigMap) error
	configMapDelete(configmap *corev1.ConfigMap)
	getBaseModel(modelName string) string
}

// newAdaptersStore creates a new adapters store.
func newAdaptersStore() adaptersStore {
	return &adaptersStoreImpl{
		loraAdapterToBaseModel: map[string]string{},
		configmapAdapters:      map[types.NamespacedName]sets.Set[string]{},
		baseModels:             map[string]int{},
		lock:                   sync.RWMutex{},
	}
}

type adaptersStoreImpl struct {
	loraAdapterToBaseModel map[string]string                         // a mapping between a lora adapter and its corresponding base model
	configmapAdapters      map[types.NamespacedName]sets.Set[string] // map from configmap namespaced name to its adapters
	baseModels             map[string]int                            // base model with a counter of its configmaps
	lock                   sync.RWMutex
}

func (as *adaptersStoreImpl) configMapUpdateOrAddIfNotExist(configmap *corev1.ConfigMap) error {
	baseModel, newAdapters, err := as.parseConfigMap(configmap)
	if err != nil {
		return fmt.Errorf("failed to parse configmap - %w", err)
	}

	configmapNamespacedName := types.NamespacedName{Namespace: configmap.GetNamespace(), Name: configmap.GetName()}
	existingAdapters := sets.Set[string]{}
	as.lock.Lock()
	defer as.lock.Unlock()
	previousAdapters, configmapExist := as.configmapAdapters[configmapNamespacedName]
	if configmapExist {
		existingAdapters = previousAdapters
	}
	adaptersToRemove := existingAdapters.Difference(newAdapters) // adapters in existingAdapters but not in newAdapters.
	// update loraAdapterToBaseModel mapping
	for adapterToUpdate := range newAdapters {
		as.loraAdapterToBaseModel[adapterToUpdate] = baseModel
	}
	for adapterToRemove := range adaptersToRemove {
		delete(as.loraAdapterToBaseModel, adapterToRemove)
	}
	// update base model count, update count only if this configmap is added
	if !configmapExist {
		if count, baseExist := as.baseModels[baseModel]; baseExist {
			as.baseModels[baseModel] = count + 1
		} else {
			as.baseModels[baseModel] = 1
		}
	}
	// update configmap NamespacedName to current set of adapters mapping
	as.configmapAdapters[configmapNamespacedName] = newAdapters

	return nil
}

func (as *adaptersStoreImpl) configMapDelete(configmap *corev1.ConfigMap) {
	configmapNamespacedName := types.NamespacedName{Namespace: configmap.GetNamespace(), Name: configmap.GetName()}
	as.lock.Lock()
	defer as.lock.Unlock()
	adapters, configmapExist := as.configmapAdapters[configmapNamespacedName]
	if !configmapExist {
		return
	}
	// otherwise, configmap exist and we should delete its data

	// delete adapters from loraAdapterToBaseModel mapping
	for adapter := range adapters {
		delete(as.loraAdapterToBaseModel, adapter)
	}
	// delete configmap NamespacedName to current set of adapters mapping
	delete(as.configmapAdapters, configmapNamespacedName)

	// update base model count
	baseModel, err := as.parseBaseModelFromConfigMap(configmap) // parse base model
	if err != nil {
		return // if no base model field exist we cannot delete it
	}
	if count := as.baseModels[baseModel]; count == 1 {
		delete(as.baseModels, baseModel)
	} else {
		as.baseModels[baseModel] = count - 1
	}
}

func (as *adaptersStoreImpl) getBaseModel(modelName string) string {
	trimmedModelName := strings.TrimSpace(modelName)
	// if the given model name is a LoRA adapter, we should return its base model
	if baseModel, ok := as.loraAdapterToBaseModel[trimmedModelName]; ok {
		return baseModel
	}
	// otherwise, if the given model is a base model return it as is.
	if _, ok := as.baseModels[trimmedModelName]; ok {
		return trimmedModelName
	}
	// otherwise, this is not a LoRA adapter nor a base model, return an empty string
	return ""
}

// parseConfigMap returns a tuple consisting (base model, set of adapters, error)
// error is set in case the configmap data section is not in the expected format.
func (as *adaptersStoreImpl) parseConfigMap(configmap *corev1.ConfigMap) (string, sets.Set[string], error) {
	// parse base model
	baseModel, err := as.parseBaseModelFromConfigMap(configmap)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse configmap %s/%s", configmap.GetNamespace(), configmap.GetName())
	}

	adapters := sets.Set[string]{}
	// parse adapters
	if raw, ok := configmap.Data[adaptersKey]; ok && strings.TrimSpace(raw) != "" {
		var list []string
		if err := yaml.Unmarshal([]byte(raw), &list); err != nil {
			return "", nil, fmt.Errorf("failed to parse adapters: %w", err)
		}

		for _, adapter := range list {
			trimmedAdapter := strings.TrimSpace(adapter)
			if trimmedAdapter == "" {
				continue // skip empty entries
			}
			adapters.Insert(trimmedAdapter)
		}
	}

	return baseModel, adapters, nil
}

func (as *adaptersStoreImpl) parseBaseModelFromConfigMap(configmap *corev1.ConfigMap) (string, error) {
	// parse base model
	baseModel, ok := configmap.Data[baseModelKey]
	if !ok || strings.TrimSpace(baseModel) == "" {
		return "", errors.New("missing or empty baseModel in configmap")
	}

	return strings.TrimSpace(baseModel), nil
}
