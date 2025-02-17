/*
Policy Manager Service

No description provided (generated by Openapi Generator https://github.com/openapitools/openapi-generator)

API version: 1.0.0
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package openapiclient

import (
	"bytes"
	_context "context"
	_ioutil "io/ioutil"
	_nethttp "net/http"
	_neturl "net/url"
)

// Linger please
var (
	_ _context.Context
)

// DefaultApiService DefaultApi service
type DefaultApiService service

type ApiGetPoliciesDecisionsPostRequest struct {
	ctx                  _context.Context
	ApiService           *DefaultApiService
	xRequestCred         *string
	policyManagerRequest *PolicyManagerRequest
}

func (r ApiGetPoliciesDecisionsPostRequest) XRequestCred(xRequestCred string) ApiGetPoliciesDecisionsPostRequest {
	r.xRequestCred = &xRequestCred
	return r
}

// Policy Manager Request Object.
func (r ApiGetPoliciesDecisionsPostRequest) PolicyManagerRequest(policyManagerRequest PolicyManagerRequest) ApiGetPoliciesDecisionsPostRequest {
	r.policyManagerRequest = &policyManagerRequest
	return r
}

func (r ApiGetPoliciesDecisionsPostRequest) Execute() (PolicyManagerResponse, *_nethttp.Response, error) {
	return r.ApiService.GetPoliciesDecisionsPostExecute(r)
}

/*
GetPoliciesDecisionsPost getPoliciesDecisions.

 @param ctx _context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 @return ApiGetPoliciesDecisionsPostRequest
*/
func (a *DefaultApiService) GetPoliciesDecisionsPost(ctx _context.Context) ApiGetPoliciesDecisionsPostRequest {
	return ApiGetPoliciesDecisionsPostRequest{
		ApiService: a,
		ctx:        ctx,
	}
}

// Execute executes the request
//  @return PolicyManagerResponse
func (a *DefaultApiService) GetPoliciesDecisionsPostExecute(r ApiGetPoliciesDecisionsPostRequest) (PolicyManagerResponse, *_nethttp.Response, error) {
	var (
		localVarHTTPMethod   = _nethttp.MethodPost
		localVarPostBody     interface{}
		localVarFormFileName string
		localVarFileName     string
		localVarFileBytes    []byte
		localVarReturnValue  PolicyManagerResponse
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "DefaultApiService.GetPoliciesDecisionsPost")
	if err != nil {
		return localVarReturnValue, nil, GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/getPoliciesDecisions"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := _neturl.Values{}
	localVarFormParams := _neturl.Values{}
	if r.xRequestCred == nil {
		return localVarReturnValue, nil, reportError("xRequestCred is required and must be specified")
	}
	if r.policyManagerRequest == nil {
		return localVarReturnValue, nil, reportError("policyManagerRequest is required and must be specified")
	}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{"application/json"}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	localVarHeaderParams["X-Request-Cred"] = parameterToString(*r.xRequestCred, "")
	// body params
	localVarPostBody = r.policyManagerRequest
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFormFileName, localVarFileName, localVarFileBytes)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := _ioutil.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = _ioutil.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}
