# \TransactionsApi

All URIs are relative to *http://localhost/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CommitTransaction**](TransactionsApi.md#CommitTransaction) | **Put** /services/haproxy/transactions/{id} | Commit transaction
[**DeleteTransaction**](TransactionsApi.md#DeleteTransaction) | **Delete** /services/haproxy/transactions/{id} | Delete a transaction
[**GetTransaction**](TransactionsApi.md#GetTransaction) | **Get** /services/haproxy/transactions/{id} | Return one HAProxy configuration transactions
[**GetTransactions**](TransactionsApi.md#GetTransactions) | **Get** /services/haproxy/transactions | Return list of HAProxy configuration transactions.
[**StartTransaction**](TransactionsApi.md#StartTransaction) | **Post** /services/haproxy/transactions | Start a new transaction



## CommitTransaction

> Transaction CommitTransaction(ctx, id, optional)

Commit transaction

Commit transaction, execute all operations in transaction and return msg

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **string**| Transaction id | 
 **optional** | ***CommitTransactionOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CommitTransactionOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**Transaction**](transaction.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DeleteTransaction

> DeleteTransaction(ctx, id)

Delete a transaction

Deletes a transaction.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **string**| Transaction id | 

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


## GetTransaction

> Transaction GetTransaction(ctx, id)

Return one HAProxy configuration transactions

Returns one HAProxy configuration transactions.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **string**| Transaction id | 

### Return type

[**Transaction**](transaction.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetTransactions

> []Transaction GetTransactions(ctx, optional)

Return list of HAProxy configuration transactions.

Returns a list of HAProxy configuration transactions. Transactions can be filtered by their status.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
 **optional** | ***GetTransactionsOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetTransactionsOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **status** | **optional.String**| Filter by transaction status | 

### Return type

[**[]Transaction**](transaction.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## StartTransaction

> Transaction StartTransaction(ctx, version)

Start a new transaction

Starts a new transaction and returns it's id

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**version** | **int32**| Configuration version on which to work on | 

### Return type

[**Transaction**](transaction.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

