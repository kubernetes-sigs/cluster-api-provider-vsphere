# \ServerApi

All URIs are relative to *http://localhost/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CreateServer**](ServerApi.md#CreateServer) | **Post** /services/haproxy/configuration/servers | Add a new server
[**DeleteServer**](ServerApi.md#DeleteServer) | **Delete** /services/haproxy/configuration/servers/{name} | Delete a server
[**GetServer**](ServerApi.md#GetServer) | **Get** /services/haproxy/configuration/servers/{name} | Return one server
[**GetServers**](ServerApi.md#GetServers) | **Get** /services/haproxy/configuration/servers | Return an array of servers
[**ReplaceServer**](ServerApi.md#ReplaceServer) | **Put** /services/haproxy/configuration/servers/{name} | Replace a server



## CreateServer

> Server CreateServer(ctx, backend, server, optional)

Add a new server

Adds a new server in the specified backend in the configuration file.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**backend** | **string**| Parent backend name | 
**server** | [**Server**](Server.md)|  | 
 **optional** | ***CreateServerOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CreateServerOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**Server**](server.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DeleteServer

> DeleteServer(ctx, name, backend, optional)

Delete a server

Deletes a server configuration by it's name in the specified backend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**name** | **string**| Server name | 
**backend** | **string**| Parent backend name | 
 **optional** | ***DeleteServerOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DeleteServerOpts struct


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


## GetServer

> InlineResponse20011 GetServer(ctx, name, backend, optional)

Return one server

Returns one server configuration by it's name in the specified backend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**name** | **string**| Server name | 
**backend** | **string**| Parent backend name | 
 **optional** | ***GetServerOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetServerOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20011**](inline_response_200_11.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetServers

> InlineResponse20010 GetServers(ctx, backend, optional)

Return an array of servers

Returns an array of all servers that are configured in specified backend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**backend** | **string**| Parent backend name | 
 **optional** | ***GetServersOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetServersOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20010**](inline_response_200_10.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ReplaceServer

> Server ReplaceServer(ctx, name, backend, server, optional)

Replace a server

Replaces a server configuration by it's name in the specified backend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**name** | **string**| Server name | 
**backend** | **string**| Parent backend name | 
**server** | [**Server**](Server.md)|  | 
 **optional** | ***ReplaceServerOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ReplaceServerOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**Server**](server.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

