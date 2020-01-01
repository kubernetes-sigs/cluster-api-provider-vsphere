# \HTTPResponseRuleApi

All URIs are relative to *http://localhost/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CreateHTTPResponseRule**](HTTPResponseRuleApi.md#CreateHTTPResponseRule) | **Post** /services/haproxy/configuration/http_response_rules | Add a new HTTP Response Rule
[**DeleteHTTPResponseRule**](HTTPResponseRuleApi.md#DeleteHTTPResponseRule) | **Delete** /services/haproxy/configuration/http_response_rules/{id} | Delete a HTTP Response Rule
[**GetHTTPResponseRule**](HTTPResponseRuleApi.md#GetHTTPResponseRule) | **Get** /services/haproxy/configuration/http_response_rules/{id} | Return one HTTP Response Rule
[**GetHTTPResponseRules**](HTTPResponseRuleApi.md#GetHTTPResponseRules) | **Get** /services/haproxy/configuration/http_response_rules | Return an array of all HTTP Response Rules
[**ReplaceHTTPResponseRule**](HTTPResponseRuleApi.md#ReplaceHTTPResponseRule) | **Put** /services/haproxy/configuration/http_response_rules/{id} | Replace a HTTP Response Rule



## CreateHTTPResponseRule

> HttpResponseRule CreateHTTPResponseRule(ctx, parentName, parentType, httpResponseRule, optional)

Add a new HTTP Response Rule

Adds a new HTTP Response Rule of the specified type in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
**httpResponseRule** | [**HttpResponseRule**](HttpResponseRule.md)|  | 
 **optional** | ***CreateHTTPResponseRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CreateHTTPResponseRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**HttpResponseRule**](http_response_rule.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DeleteHTTPResponseRule

> DeleteHTTPResponseRule(ctx, id, parentName, parentType, optional)

Delete a HTTP Response Rule

Deletes a HTTP Response Rule configuration by it's ID from the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| HTTP Response Rule ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***DeleteHTTPResponseRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DeleteHTTPResponseRuleOpts struct


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


## GetHTTPResponseRule

> InlineResponse20015 GetHTTPResponseRule(ctx, id, parentName, parentType, optional)

Return one HTTP Response Rule

Returns one HTTP Response Rule configuration by it's ID in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| HTTP Response Rule ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***GetHTTPResponseRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetHTTPResponseRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20015**](inline_response_200_15.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetHTTPResponseRules

> InlineResponse20014 GetHTTPResponseRules(ctx, parentName, parentType, optional)

Return an array of all HTTP Response Rules

Returns all HTTP Response Rules that are configured in specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***GetHTTPResponseRulesOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetHTTPResponseRulesOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20014**](inline_response_200_14.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ReplaceHTTPResponseRule

> HttpResponseRule ReplaceHTTPResponseRule(ctx, id, parentName, parentType, httpResponseRule, optional)

Replace a HTTP Response Rule

Replaces a HTTP Response Rule configuration by it's ID in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| HTTP Response Rule ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
**httpResponseRule** | [**HttpResponseRule**](HttpResponseRule.md)|  | 
 **optional** | ***ReplaceHTTPResponseRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ReplaceHTTPResponseRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------




 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**HttpResponseRule**](http_response_rule.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

