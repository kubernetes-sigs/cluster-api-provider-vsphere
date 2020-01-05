# \DiscoveryApi

All URIs are relative to *http://localhost/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**GetAPIEndpoints**](DiscoveryApi.md#GetAPIEndpoints) | **Get** / | Return list of root endpoints
[**GetConfigurationEndpoints**](DiscoveryApi.md#GetConfigurationEndpoints) | **Get** /services/haproxy/configuration | Return list of HAProxy advanced configuration endpoints
[**GetHaproxyEndpoints**](DiscoveryApi.md#GetHaproxyEndpoints) | **Get** /services/haproxy | Return list of HAProxy related endpoints
[**GetServicesEndpoints**](DiscoveryApi.md#GetServicesEndpoints) | **Get** /services | Return list of service endpoints
[**GetStatsEndpoints**](DiscoveryApi.md#GetStatsEndpoints) | **Get** /services/haproxy/stats | Return list of HAProxy stats endpoints



## GetAPIEndpoints

> []Endpoint GetAPIEndpoints(ctx, )

Return list of root endpoints

Returns a list of root endpoints.

### Required Parameters

This endpoint does not need any parameter.

### Return type

[**[]Endpoint**](endpoint.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetConfigurationEndpoints

> []Endpoint GetConfigurationEndpoints(ctx, )

Return list of HAProxy advanced configuration endpoints

Returns a list of endpoints to be used for advanced configuration of HAProxy objects.

### Required Parameters

This endpoint does not need any parameter.

### Return type

[**[]Endpoint**](endpoint.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetHaproxyEndpoints

> []Endpoint GetHaproxyEndpoints(ctx, )

Return list of HAProxy related endpoints

Returns a list of HAProxy related endpoints.

### Required Parameters

This endpoint does not need any parameter.

### Return type

[**[]Endpoint**](endpoint.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetServicesEndpoints

> []Endpoint GetServicesEndpoints(ctx, )

Return list of service endpoints

Returns a list of API managed services endpoints.

### Required Parameters

This endpoint does not need any parameter.

### Return type

[**[]Endpoint**](endpoint.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetStatsEndpoints

> []Endpoint GetStatsEndpoints(ctx, )

Return list of HAProxy stats endpoints

Returns a list of HAProxy stats endpoints.

### Required Parameters

This endpoint does not need any parameter.

### Return type

[**[]Endpoint**](endpoint.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

