# \DefaultsApi

All URIs are relative to *http://localhost/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**GetDefaults**](DefaultsApi.md#GetDefaults) | **Get** /services/haproxy/configuration/defaults | Return defaults part of configuration
[**ReplaceDefaults**](DefaultsApi.md#ReplaceDefaults) | **Put** /services/haproxy/configuration/defaults | Replace defaults



## GetDefaults

> InlineResponse2003 GetDefaults(ctx, optional)

Return defaults part of configuration

Returns defaults part of configuration.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
 **optional** | ***GetDefaultsOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetDefaultsOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse2003**](inline_response_200_3.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ReplaceDefaults

> Defaults ReplaceDefaults(ctx, defaults, optional)

Replace defaults

Replace defaults part of config

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**defaults** | [**Defaults**](Defaults.md)|  | 
 **optional** | ***ReplaceDefaultsOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ReplaceDefaultsOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**Defaults**](defaults.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

