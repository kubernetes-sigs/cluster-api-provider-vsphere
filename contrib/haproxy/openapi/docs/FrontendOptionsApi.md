# \FrontendOptionsApi

All URIs are relative to *http://localhost/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CreateAcl**](FrontendOptionsApi.md#CreateAcl) | **Post** /services/haproxy/configuration/acls | Add a new ACL line
[**CreateBackendSwitchingRule**](FrontendOptionsApi.md#CreateBackendSwitchingRule) | **Post** /services/haproxy/configuration/backend_switching_rules | Add a new Backend Switching Rule
[**CreateBind**](FrontendOptionsApi.md#CreateBind) | **Post** /services/haproxy/configuration/binds | Add a new bind
[**CreateFilter**](FrontendOptionsApi.md#CreateFilter) | **Post** /services/haproxy/configuration/filters | Add a new Filter
[**CreateHTTPRequestRule**](FrontendOptionsApi.md#CreateHTTPRequestRule) | **Post** /services/haproxy/configuration/http_request_rules | Add a new HTTP Request Rule
[**CreateHTTPResponseRule**](FrontendOptionsApi.md#CreateHTTPResponseRule) | **Post** /services/haproxy/configuration/http_response_rules | Add a new HTTP Response Rule
[**CreateLogTarget**](FrontendOptionsApi.md#CreateLogTarget) | **Post** /services/haproxy/configuration/log_targets | Add a new Log Target
[**CreateTCPRequestRule**](FrontendOptionsApi.md#CreateTCPRequestRule) | **Post** /services/haproxy/configuration/tcp_request_rules | Add a new TCP Request Rule
[**CreateTCPResponseRule**](FrontendOptionsApi.md#CreateTCPResponseRule) | **Post** /services/haproxy/configuration/tcp_response_rules | Add a new TCP Response Rule
[**DeleteAcl**](FrontendOptionsApi.md#DeleteAcl) | **Delete** /services/haproxy/configuration/acls/{id} | Delete a ACL line
[**DeleteBackendSwitchingRule**](FrontendOptionsApi.md#DeleteBackendSwitchingRule) | **Delete** /services/haproxy/configuration/backend_switching_rules/{id} | Delete a Backend Switching Rule
[**DeleteBind**](FrontendOptionsApi.md#DeleteBind) | **Delete** /services/haproxy/configuration/binds/{name} | Delete a bind
[**DeleteFilter**](FrontendOptionsApi.md#DeleteFilter) | **Delete** /services/haproxy/configuration/filters/{id} | Delete a Filter
[**DeleteHTTPRequestRule**](FrontendOptionsApi.md#DeleteHTTPRequestRule) | **Delete** /services/haproxy/configuration/http_request_rules/{id} | Delete a HTTP Request Rule
[**DeleteHTTPResponseRule**](FrontendOptionsApi.md#DeleteHTTPResponseRule) | **Delete** /services/haproxy/configuration/http_response_rules/{id} | Delete a HTTP Response Rule
[**DeleteLogTarget**](FrontendOptionsApi.md#DeleteLogTarget) | **Delete** /services/haproxy/configuration/log_targets/{id} | Delete a Log Target
[**DeleteTCPRequestRule**](FrontendOptionsApi.md#DeleteTCPRequestRule) | **Delete** /services/haproxy/configuration/tcp_request_rules/{id} | Delete a TCP Request Rule
[**DeleteTCPResponseRule**](FrontendOptionsApi.md#DeleteTCPResponseRule) | **Delete** /services/haproxy/configuration/tcp_response_rules/{id} | Delete a TCP Response Rule
[**GetAcl**](FrontendOptionsApi.md#GetAcl) | **Get** /services/haproxy/configuration/acls/{id} | Return one ACL line
[**GetAcls**](FrontendOptionsApi.md#GetAcls) | **Get** /services/haproxy/configuration/acls | Return an array of all ACL lines
[**GetBackendSwitchingRule**](FrontendOptionsApi.md#GetBackendSwitchingRule) | **Get** /services/haproxy/configuration/backend_switching_rules/{id} | Return one Backend Switching Rule
[**GetBackendSwitchingRules**](FrontendOptionsApi.md#GetBackendSwitchingRules) | **Get** /services/haproxy/configuration/backend_switching_rules | Return an array of all Backend Switching Rules
[**GetBind**](FrontendOptionsApi.md#GetBind) | **Get** /services/haproxy/configuration/binds/{name} | Return one bind
[**GetBinds**](FrontendOptionsApi.md#GetBinds) | **Get** /services/haproxy/configuration/binds | Return an array of binds
[**GetFilter**](FrontendOptionsApi.md#GetFilter) | **Get** /services/haproxy/configuration/filters/{id} | Return one Filter
[**GetFilters**](FrontendOptionsApi.md#GetFilters) | **Get** /services/haproxy/configuration/filters | Return an array of all Filters
[**GetHTTPRequestRule**](FrontendOptionsApi.md#GetHTTPRequestRule) | **Get** /services/haproxy/configuration/http_request_rules/{id} | Return one HTTP Request Rule
[**GetHTTPRequestRules**](FrontendOptionsApi.md#GetHTTPRequestRules) | **Get** /services/haproxy/configuration/http_request_rules | Return an array of all HTTP Request Rules
[**GetHTTPResponseRule**](FrontendOptionsApi.md#GetHTTPResponseRule) | **Get** /services/haproxy/configuration/http_response_rules/{id} | Return one HTTP Response Rule
[**GetHTTPResponseRules**](FrontendOptionsApi.md#GetHTTPResponseRules) | **Get** /services/haproxy/configuration/http_response_rules | Return an array of all HTTP Response Rules
[**GetLogTarget**](FrontendOptionsApi.md#GetLogTarget) | **Get** /services/haproxy/configuration/log_targets/{id} | Return one Log Target
[**GetLogTargets**](FrontendOptionsApi.md#GetLogTargets) | **Get** /services/haproxy/configuration/log_targets | Return an array of all Log Targets
[**GetTCPRequestRule**](FrontendOptionsApi.md#GetTCPRequestRule) | **Get** /services/haproxy/configuration/tcp_request_rules/{id} | Return one TCP Request Rule
[**GetTCPRequestRules**](FrontendOptionsApi.md#GetTCPRequestRules) | **Get** /services/haproxy/configuration/tcp_request_rules | Return an array of all TCP Request Rules
[**GetTCPResponseRule**](FrontendOptionsApi.md#GetTCPResponseRule) | **Get** /services/haproxy/configuration/tcp_response_rules/{id} | Return one TCP Response Rule
[**GetTCPResponseRules**](FrontendOptionsApi.md#GetTCPResponseRules) | **Get** /services/haproxy/configuration/tcp_response_rules | Return an array of all TCP Response Rules
[**ReplaceAcl**](FrontendOptionsApi.md#ReplaceAcl) | **Put** /services/haproxy/configuration/acls/{id} | Replace a ACL line
[**ReplaceBackendSwitchingRule**](FrontendOptionsApi.md#ReplaceBackendSwitchingRule) | **Put** /services/haproxy/configuration/backend_switching_rules/{id} | Replace a Backend Switching Rule
[**ReplaceBind**](FrontendOptionsApi.md#ReplaceBind) | **Put** /services/haproxy/configuration/binds/{name} | Replace a bind
[**ReplaceFilter**](FrontendOptionsApi.md#ReplaceFilter) | **Put** /services/haproxy/configuration/filters/{id} | Replace a Filter
[**ReplaceHTTPRequestRule**](FrontendOptionsApi.md#ReplaceHTTPRequestRule) | **Put** /services/haproxy/configuration/http_request_rules/{id} | Replace a HTTP Request Rule
[**ReplaceHTTPResponseRule**](FrontendOptionsApi.md#ReplaceHTTPResponseRule) | **Put** /services/haproxy/configuration/http_response_rules/{id} | Replace a HTTP Response Rule
[**ReplaceLogTarget**](FrontendOptionsApi.md#ReplaceLogTarget) | **Put** /services/haproxy/configuration/log_targets/{id} | Replace a Log Target
[**ReplaceTCPRequestRule**](FrontendOptionsApi.md#ReplaceTCPRequestRule) | **Put** /services/haproxy/configuration/tcp_request_rules/{id} | Replace a TCP Request Rule
[**ReplaceTCPResponseRule**](FrontendOptionsApi.md#ReplaceTCPResponseRule) | **Put** /services/haproxy/configuration/tcp_response_rules/{id} | Replace a TCP Response Rule



## CreateAcl

> Acl CreateAcl(ctx, parentName, parentType, acl, optional)

Add a new ACL line

Adds a new ACL line of the specified type in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
**acl** | [**Acl**](Acl.md)|  | 
 **optional** | ***CreateAclOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CreateAclOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**Acl**](acl.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## CreateBackendSwitchingRule

> BackendSwitchingRule CreateBackendSwitchingRule(ctx, frontend, backendSwitchingRule, optional)

Add a new Backend Switching Rule

Adds a new Backend Switching Rule of the specified type in the specified frontend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**frontend** | **string**| Frontend name | 
**backendSwitchingRule** | [**BackendSwitchingRule**](BackendSwitchingRule.md)|  | 
 **optional** | ***CreateBackendSwitchingRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CreateBackendSwitchingRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**BackendSwitchingRule**](backend_switching_rule.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## CreateBind

> Bind CreateBind(ctx, frontend, bind, optional)

Add a new bind

Adds a new bind in the specified frontend in the configuration file.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**frontend** | **string**| Parent frontend name | 
**bind** | [**Bind**](Bind.md)|  | 
 **optional** | ***CreateBindOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CreateBindOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**Bind**](bind.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


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


## CreateHTTPRequestRule

> HttpRequestRule CreateHTTPRequestRule(ctx, parentName, parentType, httpRequestRule, optional)

Add a new HTTP Request Rule

Adds a new HTTP Request Rule of the specified type in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
**httpRequestRule** | [**HttpRequestRule**](HttpRequestRule.md)|  | 
 **optional** | ***CreateHTTPRequestRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CreateHTTPRequestRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**HttpRequestRule**](http_request_rule.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


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


## CreateTCPRequestRule

> TcpRequestRule CreateTCPRequestRule(ctx, parentName, parentType, tcpRequestRule, optional)

Add a new TCP Request Rule

Adds a new TCP Request Rule of the specified type in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
**tcpRequestRule** | [**TcpRequestRule**](TcpRequestRule.md)|  | 
 **optional** | ***CreateTCPRequestRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CreateTCPRequestRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**TcpRequestRule**](tcp_request_rule.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## CreateTCPResponseRule

> TcpResponseRule CreateTCPResponseRule(ctx, backend, tcpResponseRule, optional)

Add a new TCP Response Rule

Adds a new TCP Response Rule of the specified type in the specified backend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**backend** | **string**| Parent backend name | 
**tcpResponseRule** | [**TcpResponseRule**](TcpResponseRule.md)|  | 
 **optional** | ***CreateTCPResponseRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CreateTCPResponseRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**TcpResponseRule**](tcp_response_rule.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DeleteAcl

> DeleteAcl(ctx, id, parentName, parentType, optional)

Delete a ACL line

Deletes a ACL line configuration by it's ID from the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| ACL line ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***DeleteAclOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DeleteAclOpts struct


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


## DeleteBackendSwitchingRule

> DeleteBackendSwitchingRule(ctx, id, frontend, optional)

Delete a Backend Switching Rule

Deletes a Backend Switching Rule configuration by it's ID from the specified frontend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| Switching Rule ID | 
**frontend** | **string**| Frontend name | 
 **optional** | ***DeleteBackendSwitchingRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DeleteBackendSwitchingRuleOpts struct


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


## DeleteBind

> DeleteBind(ctx, name, frontend, optional)

Delete a bind

Deletes a bind configuration by it's name in the specified frontend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**name** | **string**| Bind name | 
**frontend** | **string**| Parent frontend name | 
 **optional** | ***DeleteBindOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DeleteBindOpts struct


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


## DeleteHTTPRequestRule

> DeleteHTTPRequestRule(ctx, id, parentName, parentType, optional)

Delete a HTTP Request Rule

Deletes a HTTP Request Rule configuration by it's ID from the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| HTTP Request Rule ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***DeleteHTTPRequestRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DeleteHTTPRequestRuleOpts struct


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


## DeleteTCPRequestRule

> DeleteTCPRequestRule(ctx, id, parentName, parentType, optional)

Delete a TCP Request Rule

Deletes a TCP Request Rule configuration by it's ID from the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| TCP Request Rule ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***DeleteTCPRequestRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DeleteTCPRequestRuleOpts struct


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


## DeleteTCPResponseRule

> DeleteTCPResponseRule(ctx, id, backend, optional)

Delete a TCP Response Rule

Deletes a TCP Response Rule configuration by it's ID from the specified backend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| TCP Response Rule ID | 
**backend** | **string**| Parent backend name | 
 **optional** | ***DeleteTCPResponseRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DeleteTCPResponseRuleOpts struct


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


## GetAcl

> InlineResponse20031 GetAcl(ctx, id, parentName, parentType, optional)

Return one ACL line

Returns one ACL line configuration by it's ID in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| ACL line ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***GetAclOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetAclOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20031**](inline_response_200_31.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetAcls

> InlineResponse20030 GetAcls(ctx, parentName, parentType, optional)

Return an array of all ACL lines

Returns all ACL lines that are configured in specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***GetAclsOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetAclsOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20030**](inline_response_200_30.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetBackendSwitchingRule

> InlineResponse20021 GetBackendSwitchingRule(ctx, id, frontend, optional)

Return one Backend Switching Rule

Returns one Backend Switching Rule configuration by it's ID in the specified frontend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| Switching Rule ID | 
**frontend** | **string**| Frontend name | 
 **optional** | ***GetBackendSwitchingRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetBackendSwitchingRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20021**](inline_response_200_21.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetBackendSwitchingRules

> InlineResponse20020 GetBackendSwitchingRules(ctx, frontend, optional)

Return an array of all Backend Switching Rules

Returns all Backend Switching Rules that are configured in specified frontend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**frontend** | **string**| Frontend name | 
 **optional** | ***GetBackendSwitchingRulesOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetBackendSwitchingRulesOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20020**](inline_response_200_20.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetBind

> InlineResponse2009 GetBind(ctx, name, frontend, optional)

Return one bind

Returns one bind configuration by it's name in the specified frontend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**name** | **string**| Bind name | 
**frontend** | **string**| Parent frontend name | 
 **optional** | ***GetBindOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetBindOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse2009**](inline_response_200_9.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetBinds

> InlineResponse2008 GetBinds(ctx, frontend, optional)

Return an array of binds

Returns an array of all binds that are configured in specified frontend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**frontend** | **string**| Parent frontend name | 
 **optional** | ***GetBindsOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetBindsOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse2008**](inline_response_200_8.md)

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


## GetHTTPRequestRule

> InlineResponse20013 GetHTTPRequestRule(ctx, id, parentName, parentType, optional)

Return one HTTP Request Rule

Returns one HTTP Request Rule configuration by it's ID in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| HTTP Request Rule ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***GetHTTPRequestRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetHTTPRequestRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20013**](inline_response_200_13.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetHTTPRequestRules

> InlineResponse20012 GetHTTPRequestRules(ctx, parentName, parentType, optional)

Return an array of all HTTP Request Rules

Returns all HTTP Request Rules that are configured in specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***GetHTTPRequestRulesOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetHTTPRequestRulesOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20012**](inline_response_200_12.md)

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


## GetTCPRequestRule

> InlineResponse20017 GetTCPRequestRule(ctx, id, parentName, parentType, optional)

Return one TCP Request Rule

Returns one TCP Request Rule configuration by it's ID in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| TCP Request Rule ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***GetTCPRequestRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetTCPRequestRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20017**](inline_response_200_17.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetTCPRequestRules

> InlineResponse20016 GetTCPRequestRules(ctx, parentName, parentType, optional)

Return an array of all TCP Request Rules

Returns all TCP Request Rules that are configured in specified parent and parent type.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
 **optional** | ***GetTCPRequestRulesOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetTCPRequestRulesOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20016**](inline_response_200_16.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetTCPResponseRule

> InlineResponse20019 GetTCPResponseRule(ctx, id, backend, optional)

Return one TCP Response Rule

Returns one TCP Response Rule configuration by it's ID in the specified backend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| TCP Response Rule ID | 
**backend** | **string**| Parent backend name | 
 **optional** | ***GetTCPResponseRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetTCPResponseRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20019**](inline_response_200_19.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetTCPResponseRules

> InlineResponse20018 GetTCPResponseRules(ctx, backend, optional)

Return an array of all TCP Response Rules

Returns all TCP Response Rules that are configured in specified backend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**backend** | **string**| Parent backend name | 
 **optional** | ***GetTCPResponseRulesOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetTCPResponseRulesOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 

### Return type

[**InlineResponse20018**](inline_response_200_18.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ReplaceAcl

> Acl ReplaceAcl(ctx, id, parentName, parentType, acl, optional)

Replace a ACL line

Replaces a ACL line configuration by it's ID in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| ACL line ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
**acl** | [**Acl**](Acl.md)|  | 
 **optional** | ***ReplaceAclOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ReplaceAclOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------




 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**Acl**](acl.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ReplaceBackendSwitchingRule

> BackendSwitchingRule ReplaceBackendSwitchingRule(ctx, id, frontend, backendSwitchingRule, optional)

Replace a Backend Switching Rule

Replaces a Backend Switching Rule configuration by it's ID in the specified frontend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| Switching Rule ID | 
**frontend** | **string**| Frontend name | 
**backendSwitchingRule** | [**BackendSwitchingRule**](BackendSwitchingRule.md)|  | 
 **optional** | ***ReplaceBackendSwitchingRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ReplaceBackendSwitchingRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**BackendSwitchingRule**](backend_switching_rule.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ReplaceBind

> Bind ReplaceBind(ctx, name, frontend, bind, optional)

Replace a bind

Replaces a bind configuration by it's name in the specified frontend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**name** | **string**| Bind name | 
**frontend** | **string**| Parent frontend name | 
**bind** | [**Bind**](Bind.md)|  | 
 **optional** | ***ReplaceBindOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ReplaceBindOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**Bind**](bind.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
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


## ReplaceHTTPRequestRule

> HttpRequestRule ReplaceHTTPRequestRule(ctx, id, parentName, parentType, httpRequestRule, optional)

Replace a HTTP Request Rule

Replaces a HTTP Request Rule configuration by it's ID in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| HTTP Request Rule ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
**httpRequestRule** | [**HttpRequestRule**](HttpRequestRule.md)|  | 
 **optional** | ***ReplaceHTTPRequestRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ReplaceHTTPRequestRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------




 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**HttpRequestRule**](http_request_rule.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
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


## ReplaceTCPRequestRule

> TcpRequestRule ReplaceTCPRequestRule(ctx, id, parentName, parentType, tcpRequestRule, optional)

Replace a TCP Request Rule

Replaces a TCP Request Rule configuration by it's ID in the specified parent.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| TCP Request Rule ID | 
**parentName** | **string**| Parent name | 
**parentType** | **string**| Parent type | 
**tcpRequestRule** | [**TcpRequestRule**](TcpRequestRule.md)|  | 
 **optional** | ***ReplaceTCPRequestRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ReplaceTCPRequestRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------




 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**TcpRequestRule**](tcp_request_rule.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ReplaceTCPResponseRule

> TcpResponseRule ReplaceTCPResponseRule(ctx, id, backend, tcpResponseRule, optional)

Replace a TCP Response Rule

Replaces a TCP Response Rule configuration by it's ID in the specified backend.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**id** | **int32**| TCP Response Rule ID | 
**backend** | **string**| Parent backend name | 
**tcpResponseRule** | [**TcpResponseRule**](TcpResponseRule.md)|  | 
 **optional** | ***ReplaceTCPResponseRuleOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ReplaceTCPResponseRuleOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **transactionId** | **optional.String**| ID of the transaction where we want to add the operation. Cannot be used when version is specified. | 
 **version** | **optional.Int32**| Version used for checking configuration version. Cannot be used when transaction is specified, transaction has it&#39;s own version. | 
 **forceReload** | **optional.Bool**| If set, do a force reload, do not wait for the configured reload-delay. Cannot be used when transaction is specified, as changes in transaction are not applied directly to configuration. | [default to false]

### Return type

[**TcpResponseRule**](tcp_response_rule.md)

### Authorization

[basic_auth](../README.md#basic_auth)

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

