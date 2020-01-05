# \ServerSwitchingRuleApi

All URIs are relative to *http://localhost/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CreateServerSwitchingRule**](ServerSwitchingRuleApi.md#CreateServerSwitchingRule) | **Post** /services/haproxy/configuration/server_switching_rules | Add a new Server Switching Rule
[**DeleteServerSwitchingRule**](ServerSwitchingRuleApi.md#DeleteServerSwitchingRule) | **Delete** /services/haproxy/configuration/server_switching_rules/{id} | Delete a Server Switching Rule
[**GetServerSwitchingRule**](ServerSwitchingRuleApi.md#GetServerSwitchingRule) | **Get** /services/haproxy/configuration/server_switching_rules/{id} | Return one Server Switching Rule
[**GetServerSwitchingRules**](ServerSwitchingRuleApi.md#GetServerSwitchingRules) | **Get** /services/haproxy/configuration/server_switching_rules | Return an array of all Server Switching Rules
[**ReplaceServerSwitchingRule**](ServerSwitchingRuleApi.md#ReplaceServerSwitchingRule) | **Put** /services/haproxy/configuration/server_switching_rules/{id} | Replace a Server Switching Rule



## CreateServerSwitchingRule

> ServerSwitchingRule CreateServerSwitchingRule(ctx, backend, serverSwitchingRule, optional)

Add a new Server Switching Rule

Adds a new Server Switching Rule of the specified type in the specified backend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**backend** | **string**| Backend name | 
**serverSwitchingRule** | [**ServerSwitchingRule**](ServerSwitchingRule.md)|  | 
 **optional** | ***CreateServerSwitchingRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CreateServerSwitchingRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**ServerSwitchingRule**](server_switching_rule.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DeleteServerSwitchingRule

> DeleteServerSwitchingRule(ctx, id, backend, optional)

Delete a Server Switching Rule

Deletes a Server Switching Rule configuration by it's ID from the specified backend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| Switching Rule ID | 
**backend** | **string**| Backend name | 
 **optional** | ***DeleteServerSwitchingRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DeleteServerSwitchingRuleOpts struct


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


## GetServerSwitchingRule

> InlineResponse20023 GetServerSwitchingRule(ctx, id, backend, optional)

Return one Server Switching Rule

Returns one Server Switching Rule configuration by it's ID in the specified backend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| Switching Rule ID | 
**backend** | **string**| Backend name | 
 **optional** | ***GetServerSwitchingRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetServerSwitchingRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20023**](inline_response_200_23.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetServerSwitchingRules

> InlineResponse20022 GetServerSwitchingRules(ctx, backend, optional)

Return an array of all Server Switching Rules

Returns all Backend Switching Rules that are configured in specified backend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**backend** | **string**| Backend name | 
 **optional** | ***GetServerSwitchingRulesOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetServerSwitchingRulesOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20022**](inline_response_200_22.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ReplaceServerSwitchingRule

> ServerSwitchingRule ReplaceServerSwitchingRule(ctx, id, backend, serverSwitchingRule, optional)

Replace a Server Switching Rule

Replaces a Server Switching Rule configuration by it's ID in the specified backend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| Switching Rule ID | 
**backend** | **string**| Backend name | 
**serverSwitchingRule** | [**ServerSwitchingRule**](ServerSwitchingRule.md)|  | 
 **optional** | ***ReplaceServerSwitchingRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ReplaceServerSwitchingRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**ServerSwitchingRule**](server_switching_rule.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

