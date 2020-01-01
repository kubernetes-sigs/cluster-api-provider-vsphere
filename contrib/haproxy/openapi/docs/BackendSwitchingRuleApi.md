# \BackendSwitchingRuleApi

All URIs are relative to *http://localhost/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CreateBackendSwitchingRule**](BackendSwitchingRuleApi.md#CreateBackendSwitchingRule) | **Post** /services/haproxy/configuration/backend_switching_rules | Add a new Backend Switching Rule
[**DeleteBackendSwitchingRule**](BackendSwitchingRuleApi.md#DeleteBackendSwitchingRule) | **Delete** /services/haproxy/configuration/backend_switching_rules/{id} | Delete a Backend Switching Rule
[**GetBackendSwitchingRule**](BackendSwitchingRuleApi.md#GetBackendSwitchingRule) | **Get** /services/haproxy/configuration/backend_switching_rules/{id} | Return one Backend Switching Rule
[**GetBackendSwitchingRules**](BackendSwitchingRuleApi.md#GetBackendSwitchingRules) | **Get** /services/haproxy/configuration/backend_switching_rules | Return an array of all Backend Switching Rules
[**ReplaceBackendSwitchingRule**](BackendSwitchingRuleApi.md#ReplaceBackendSwitchingRule) | **Put** /services/haproxy/configuration/backend_switching_rules/{id} | Replace a Backend Switching Rule



## CreateBackendSwitchingRule

> BackendSwitchingRule CreateBackendSwitchingRule(ctx, frontend, backendSwitchingRule, optional)

Add a new Backend Switching Rule

Adds a new Backend Switching Rule of the specified type in the specified frontend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**frontend** | **string**| Frontend name | 
**backendSwitchingRule** | [**BackendSwitchingRule**](BackendSwitchingRule.md)|  | 
 **optional** | ***CreateBackendSwitchingRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CreateBackendSwitchingRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**BackendSwitchingRule**](backend_switching_rule.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DeleteBackendSwitchingRule

> DeleteBackendSwitchingRule(ctx, id, frontend, optional)

Delete a Backend Switching Rule

Deletes a Backend Switching Rule configuration by it's ID from the specified frontend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| Switching Rule ID | 
**frontend** | **string**| Frontend name | 
 **optional** | ***DeleteBackendSwitchingRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DeleteBackendSwitchingRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

 (empty response body)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetBackendSwitchingRule

> InlineResponse20021 GetBackendSwitchingRule(ctx, id, frontend, optional)

Return one Backend Switching Rule

Returns one Backend Switching Rule configuration by it's ID in the specified frontend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| Switching Rule ID | 
**frontend** | **string**| Frontend name | 
 **optional** | ***GetBackendSwitchingRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetBackendSwitchingRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20021**](inline_response_200_21.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetBackendSwitchingRules

> InlineResponse20020 GetBackendSwitchingRules(ctx, frontend, optional)

Return an array of all Backend Switching Rules

Returns all Backend Switching Rules that are configured in specified frontend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**frontend** | **string**| Frontend name | 
 **optional** | ***GetBackendSwitchingRulesOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetBackendSwitchingRulesOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20020**](inline_response_200_20.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ReplaceBackendSwitchingRule

> BackendSwitchingRule ReplaceBackendSwitchingRule(ctx, id, frontend, backendSwitchingRule, optional)

Replace a Backend Switching Rule

Replaces a Backend Switching Rule configuration by it's ID in the specified frontend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| Switching Rule ID | 
**frontend** | **string**| Frontend name | 
**backendSwitchingRule** | [**BackendSwitchingRule**](BackendSwitchingRule.md)|  | 
 **optional** | ***ReplaceBackendSwitchingRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ReplaceBackendSwitchingRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**BackendSwitchingRule**](backend_switching_rule.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

