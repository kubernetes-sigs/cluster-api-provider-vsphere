# \ReloadsApi

All URIs are relative to *http://localhost/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**GetReload**](ReloadsApi.md#GetReload) | **Get** /services/haproxy/reloads/{id} | Return one HAProxy reload status
[**GetReloads**](ReloadsApi.md#GetReloads) | **Get** /services/haproxy/reloads | Return list of HAProxy Reloads.



## GetReload

> Reload GetReload(ctx, id)

Return one HAProxy reload status

Returns one HAProxy reload status.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **string**| Reload id | 

### Return type

[**Reload**](reload.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetReloads

> []Reload GetReloads(ctx, )

Return list of HAProxy Reloads.

Returns a list of HAProxy reloads.

### Required Parameters

This endpoint does not need any parameter.

### Return type

[**[]Reload**](reload.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

