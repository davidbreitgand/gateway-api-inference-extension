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
	"bytes"
	"context"
	"encoding/json"
	"os"

	basepb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	eppb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/metrics"
	logutil "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/util/logging"
)

const modelHeader = "X-Gateway-Model-Name"

type RequestBody struct {
	Model string `json:"model"`
}

// HandleRequestBody handles request bodies.
func (s *Server) HandleRequestBody(ctx context.Context, requestBodyBytes []byte) ([]*eppb.ProcessingResponse, error) {
	logger := log.FromContext(ctx)
	var ret []*eppb.ProcessingResponse

	var requestBody RequestBody
	if err := json.Unmarshal(requestBodyBytes, &requestBody); err != nil {
		metrics.RecordModelNotParsedCounter()
		return nil, err
	}

	//The reason for this additional unmarshal is that I change the model name and then re-marshal RequestBody struct. But it has only one field, and I need to preserve original message at re-marshalling.
	//This can be done more efficiently if a full "official" struct by OpenAI is used. In OpenAI v2 it should be ChatCompletionNewParams
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(requestBodyBytes, &raw); err != nil {
		metrics.RecordModelNotParsedCounter()
		return nil, err
	}

	if requestBody.Model == "" {
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

	//Mutate model name if it contains reserved keyword "lora" indicating that the requested model is lora (served from the same vLLM as the base model and from the same inferencepool)
	//Convention: [model-family]/<model-name>/lora/<lora-name>
	//Model name definition (the vLLM side) does not change: <my-arbitrary-lora-name>
	//Model name in request (the client side): lora-name (no change from before)
	loraTag := os.Getenv("LORA_TAG") //set via environment
	if loraTag == "" {
		loraTag = "lora"
	}

	orig := []byte(requestBody.Model)
	prefix := []byte(requestBody.Model)
	var suffix []byte

	logger.V(logutil.DEFAULT).Info("Orig: " + string(orig))

	if idx := bytes.Index(orig, []byte(loraTag)); idx != -1 {
		lastSlash := bytes.LastIndex(orig[:idx], []byte("/"))
		if lastSlash != -1 {
			afterTag := orig[idx+len(loraTag):]             // slice after "lora"
			nextSlash := bytes.Index(afterTag, []byte("/")) // index relative to afterTag
			if nextSlash != -1 {
				prefix = orig[:lastSlash] // safe: based on orig
				// skip the slash itself by adding +1 so suffix doesn't start with '/'
				suffix = afterTag[nextSlash+1:]
				logger.V(logutil.DEFAULT).Info("Model name after mutation:" + string(suffix))
				requestBody.Model = string(suffix)
				// update only the "model" field in the original raw map so other fields (e.g. prompt) are preserved
				modelBytes, merr := json.Marshal(requestBody.Model)
				if merr != nil {
					logger.V(logutil.DEFAULT).Info("failed to marshal new model value: " + merr.Error())
				} else {
					raw["model"] = json.RawMessage(modelBytes)
					if updatedBodyBytes, merr2 := json.Marshal(raw); merr2 != nil {
						logger.V(logutil.DEFAULT).Info("failed to marshal updated request body: " + merr2.Error())
					} else {
						requestBodyBytes = updatedBodyBytes
					}
				}
			}
		}
	}

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
										Key: modelHeader,
										//RawValue: []byte(requestBody.Model),
										RawValue: prefix,
									},
								},
							},
						},
					},
				},
			},
		})
		ret = addStreamedBodyResponse(ret, requestBodyBytes)
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
										Key: modelHeader,
										//RawValue: []byte(requestBody.Model),
										RawValue: prefix,
									},
								},
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
