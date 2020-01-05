# \LogTargetApi

All URIs are relative to *http://localhost/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CreateLogTarget**](LogTargetApi.md#CreateLogTarget) | **Post** /services/haproxy/configuration/log_targets | Add a new Log Target
[**DeleteLogTarget**](LogTargetApi.md#DeleteLogTarget) | **Delete** /services/haproxy/configuration/log_targets/{id} | Delete a Log Target
[**GetLogTarget**](LogTargetApi.md#GetLogTarget) | **Get** /services/haproxy/configuration/log_targets/{id} | Return one Log Target
[**GetLogTargets**](LogTargetApi.md#GetLogTargets) | **Get** /services/haproxy/configuration/log_targets | Return an array of all Log Targets
[**ReplaceLogTarget**](LogTargetApi.md#ReplaceLogTarget) | **Put** /services/haproxy/configuration/log_targets/{id} | Replace a Log Target



## CreateLogTarget

> LogTarget CreateLogTarget(ctx, parentName, parentType, logTarget, optional)

Add a new Log Target

Adds a new Log Target of the specified type in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
**logTarget** | [**LogTarget**](LogTarget.md)|  | 
 **optional** | ***CreateLogTargetOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CreateLogTargetOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**LogTarget**](log_target.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DeleteLogTarget

> DeleteLogTarget(ctx, id, parentName, parentType, optional)

Delete a Log Target

Deletes a Log Target configuration by it's ID from the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| Log Target ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***DeleteLogTargetOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DeleteLogTargetOpts struct


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


## GetLogTarget

> InlineResponse20029 GetLogTarget(ctx, id, parentName, parentType, optional)

Return one Log Target

Returns one Log Target configuration by it's ID in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| Log Target ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***GetLogTargetOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetLogTargetOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20029**](inline_response_200_29.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetLogTargets

> InlineResponse20028 GetLogTargets(ctx, parentName, parentType, optional)

Return an array of all Log Targets

Returns all Log Targets that are configured in specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***GetLogTargetsOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetLogTargetsOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20028**](inline_response_200_28.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ReplaceLogTarget

> LogTarget ReplaceLogTarget(ctx, id, parentName, parentType, logTarget, optional)

Replace a Log Target

Replaces a Log Target configuration by it's ID in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| Log Target ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
**logTarget** | [**LogTarget**](LogTarget.md)|  | 
 **optional** | ***ReplaceLogTargetOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ReplaceLogTargetOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------




 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**LogTarget**](log_target.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

