# \ConfigurationApi

All URIs are relative to *http://localhost/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**GetHAProxyConfiguration**](ConfigurationApi.md#GetHAProxyConfiguration) | **Get** /services/haproxy/configuration/raw | Return HAProxy configuration
[**PostHAProxyConfiguration**](ConfigurationApi.md#PostHAProxyConfiguration) | **Post** /services/haproxy/configuration/raw | Push new haproxy configuration



## GetHAProxyConfiguration

> InlineResponse20032 GetHAProxyConfiguration(ctx, optional)

Return HAProxy configuration

Returns HAProxy configuration file in plain text

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
 **optional** | ***GetHAProxyConfigurationOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetHAProxyConfigurationOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 

### Return type

[**InlineResponse20032**](inline_response_200_32.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: text/plain, application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## PostHAProxyConfiguration

> string PostHAProxyConfiguration(ctx, body, optional)

Push new haproxy configuration

Push a new haproxy configuration file in plain text

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**body** | **string**|  | 
 **optional** | ***PostHAProxyConfigurationOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a PostHAProxyConfigurationOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

**string**

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: text/plain
- **Accept**: text/plain, application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

