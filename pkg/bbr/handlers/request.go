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

package handlers

import (
	"context"
	"fmt"
	"strings"

	basepb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	eppb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"sigs.k8s.io/controller-runtime/pkg/log"
	bbrplugins "sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/framework/plugins"
	helpers "sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/framework/utils"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/metrics"
	logutil "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/util/logging"
)

// HandleRequestBody handles request bodies.
func (s *Server) HandleRequestBody(ctx context.Context, requestBodyBytes []byte) ([]*eppb.ProcessingResponse, error) {
	logger := log.FromContext(ctx)

	mutatedBodyBytes := requestBodyBytes
	which := ""

	//does request plugins chain contain at least one plugin that requires parsing of the full body?
	for i := range s.requestChain.Length() {
		plugin, err := s.requestChain.GetPlugin(i, s.registry)
		if err != nil {
			logger.V(logutil.DEFAULT).Info("%s", err)
			continue
		}
		if plugin.RequiresFullParsing() {
			logger.V(logutil.DEBUG).Info("BBRPlugin %s/%s requires full body parsing", plugin.TypedName().Type, plugin.TypedName().Name)
			//parse into shared in-memory struct inside PluginsChain according to the endpoint type and break
			switch {
			case strings.Contains(s.endpoint, "/v1/completions"):
				if _, err := s.requestChain.ParseCompletion(requestBodyBytes); err != nil {
					return nil, err
				}
				which = "/v1/completions"
			case strings.Contains(s.endpoint, "/v1/chat/completions"):
				if _, err := s.requestChain.ParseChatCompletion(requestBodyBytes); err != nil {
					return nil, err
				}
				which = "/v1/chat/completions"
			}
			break //we found at least one plugin requiring full body parsing, no need to check for others
		}
	}
	//run plugins in requestPluginChain sequentially (correct order is user's responsibility)
	//TODO: replace/add pluginsDAG, to allow for more sophisticated execution flow

	var allHeaders = map[string]string{}

	for i := range s.requestChain.Length() { //no need to check for errors; validations were performed earlier at various steps
		plugin, _ := s.requestChain.GetPlugin(i, s.registry)
		pluginType := plugin.TypedName().Type
		switch pluginType {
		case "MetadataExtractor": //does not mutate body
			if metExtPlugin, ok := plugin.(bbrplugins.MetadataExtractor); ok {
				headers, err := metExtPlugin.Extract(ctx,
					requestBodyBytes,
					s.metaDataKeys,
					s.requestChain.GetSharedMemory(which))
				if err != nil {
					return nil, err
				}
				helpers.MergeMaps(allHeaders, headers) //note that the existing keys are over-written; it's the
			}
		case "ModelSelector": //potentially mutates body; even if the body is not mutated a non-empty body that equals the original body should be returned
			if modSelect, ok := plugin.(bbrplugins.ModelSelector); ok {
				var headers map[string]string
				var err error
				headers, mutatedBodyBytes, err = modSelect.Select(ctx,
					requestBodyBytes,
					s.requestChain.GetSharedMemory(which))
				if err != nil {
					return nil, err
				}
				if len(mutatedBodyBytes) == 0 {
					return nil, fmt.Errorf("empty mutated body bytes slice returned from plugin %s/%s", modSelect.TypedName().Type, modSelect.TypedName().Name)
				}
				helpers.MergeMaps(allHeaders, headers) //note that the existing keys are NOT over-written
			}
		default:
			logger.V(logutil.DEFAULT).Info("Unknown plugin type %s", pluginType)
		}
	}
	//At this point, we have all the headers and a mutated body (note that actually, the body might not be mutated, but we do not care)

	var ret []*eppb.ProcessingResponse

	// process headers
	Model := allHeaders[bbrplugins.ModelHeader] //it is required that the ModelHeader is always set (i.e., that there always exist requestPluginsChain with at least one plugin that sets the model header)
	if Model == "" {
		metrics.RecordModelNotInBodyCounter()
		logger.V(logutil.DEFAULT).Info("Request body does not contain model parameter")
		if s.streaming {
			ret = append(ret, &eppb.ProcessingResponse{
				Response: &eppb.ProcessingResponse_RequestHeaders{
					RequestHeaders: &eppb.HeadersResponse{},
				},
			})
			ret = addStreamedBodyResponse(ret, requestBodyBytes)
			return ret, nil
		} else {
			ret = append(ret, &eppb.ProcessingResponse{
				Response: &eppb.ProcessingResponse_RequestBody{
					RequestBody: &eppb.BodyResponse{},
				},
			})
		}
		return ret, nil
	}

	metrics.RecordSuccessCounter()

	if s.streaming {
		ret = append(ret, &eppb.ProcessingResponse{
			Response: &eppb.ProcessingResponse_RequestHeaders{
				RequestHeaders: &eppb.HeadersResponse{
					Response: &eppb.CommonResponse{
						ClearRouteCache: true,
						HeaderMutation: &eppb.HeaderMutation{
							SetHeaders: []*basepb.HeaderValueOption{
								{
									Header: &basepb.HeaderValue{
										Key:      bbrplugins.ModelHeader,
										RawValue: []byte(Model),
									},
								},
							},
						},
					},
				},
			},
		})
		ret = addStreamedBodyResponse(ret, mutatedBodyBytes)
		return ret, nil
	}

	return []*eppb.ProcessingResponse{
		{
			Response: &eppb.ProcessingResponse_RequestBody{
				RequestBody: &eppb.BodyResponse{
					Response: &eppb.CommonResponse{
						// Necessary so that the new headers are used in the routing decision.
						ClearRouteCache: true,
						HeaderMutation: &eppb.HeaderMutation{
							SetHeaders: []*basepb.HeaderValueOption{
								{
									Header: &basepb.HeaderValue{
										Key:      bbrplugins.ModelHeader,
										RawValue: []byte(Model),
									},
								},
							},
						},
						BodyMutation: &eppb.BodyMutation{
							Mutation: &eppb.BodyMutation_Body{
								Body: mutatedBodyBytes,
							},
						},
					},
				},
			},
		},
	}, nil
}

func addStreamedBodyResponse(responses []*eppb.ProcessingResponse, requestBodyBytes []byte) []*eppb.ProcessingResponse {
	return append(responses, &eppb.ProcessingResponse{
		Response: &eppb.ProcessingResponse_RequestBody{
			RequestBody: &eppb.BodyResponse{
				Response: &eppb.CommonResponse{
					BodyMutation: &eppb.BodyMutation{
						Mutation: &eppb.BodyMutation_StreamedResponse{
							StreamedResponse: &eppb.StreamedBodyResponse{
								Body:        requestBodyBytes,
								EndOfStream: true,
							},
						},
					},
				},
			},
		},
	})
}

// HandleRequestHeaders handles request headers.
func (s *Server) HandleRequestHeaders(headers *eppb.HttpHeaders) ([]*eppb.ProcessingResponse, error) {
	return []*eppb.ProcessingResponse{
		{
			Response: &eppb.ProcessingResponse_RequestHeaders{
				RequestHeaders: &eppb.HeadersResponse{},
			},
		},
	}, nil
}

// HandleRequestTrailers handles request trailers.
func (s *Server) HandleRequestTrailers(trailers *eppb.HttpTrailers) ([]*eppb.ProcessingResponse, error) {
	return []*eppb.ProcessingResponse{
		{
			Response: &eppb.ProcessingResponse_RequestTrailers{
				RequestTrailers: &eppb.TrailersResponse{},
			},
		},
	}, nil
}
