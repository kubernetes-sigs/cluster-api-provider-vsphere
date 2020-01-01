# \StatsApi

All URIs are relative to *http://localhost/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**GetStats**](StatsApi.md#GetStats) | **Get** /services/haproxy/stats/native | Gets stats



## GetStats

> []map[string]interface{} GetStats(ctx, optional)

Gets stats

Getting stats from the HAProxy.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
 **optional** | ***GetStatsOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetStatsOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **type_** | **optional.String**| Object type to get stats for (one of frontend, backend, server) | 
 **name** | **optional.String**| Object name to get stats for | 
 **parent** | **optional.String**| Object parent name to get stats for, in case the object is a server | 

### Return type

[**[]map[string]interface{}**](map[string]interface{}.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

