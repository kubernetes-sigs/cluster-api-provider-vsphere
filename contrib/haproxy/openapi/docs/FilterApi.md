# \FilterApi

All URIs are relative to *http://localhost/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CreateFilter**](FilterApi.md#CreateFilter) | **Post** /services/haproxy/configuration/filters | Add a new Filter
[**DeleteFilter**](FilterApi.md#DeleteFilter) | **Delete** /services/haproxy/configuration/filters/{id} | Delete a Filter
[**GetFilter**](FilterApi.md#GetFilter) | **Get** /services/haproxy/configuration/filters/{id} | Return one Filter
[**GetFilters**](FilterApi.md#GetFilters) | **Get** /services/haproxy/configuration/filters | Return an array of all Filters
[**ReplaceFilter**](FilterApi.md#ReplaceFilter) | **Put** /services/haproxy/configuration/filters/{id} | Replace a Filter



## CreateFilter

> Filter CreateFilter(ctx, parentName, parentType, filter, optional)

Add a new Filter

Adds a new Filter of the specified type in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
**filter** | [**Filter**](Filter.md)|  | 
 **optional** | ***CreateFilterOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CreateFilterOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**Filter**](filter.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DeleteFilter

> DeleteFilter(ctx, id, parentName, parentType, optional)

Delete a Filter

Deletes a Filter configuration by it's ID from the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| Filter ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***DeleteFilterOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DeleteFilterOpts struct


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


## GetFilter

> InlineResponse20025 GetFilter(ctx, id, parentName, parentType, optional)

Return one Filter

Returns one Filter configuration by it's ID in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| Filter ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***GetFilterOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetFilterOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20025**](inline_response_200_25.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetFilters

> InlineResponse20024 GetFilters(ctx, parentName, parentType, optional)

Return an array of all Filters

Returns all Filters that are configured in specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***GetFiltersOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetFiltersOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20024**](inline_response_200_24.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ReplaceFilter

> Filter ReplaceFilter(ctx, id, parentName, parentType, filter, optional)

Replace a Filter

Replaces a Filter configuration by it's ID in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| Filter ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
**filter** | [**Filter**](Filter.md)|  | 
 **optional** | ***ReplaceFilterOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ReplaceFilterOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------




 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**Filter**](filter.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

