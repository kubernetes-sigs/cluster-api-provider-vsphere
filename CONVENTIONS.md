# Conventions

## Logging

**Error (1)**
*Hard Errors that a user will likely need to fix*

Failed to connect, no hostname specified

**Warning (2)**
**Soft Errors that a retry might fix**

Failed to connect, connection refused

**Change (3)**
Machine created, updated

POST /api/v1/namespaces/default/events

PUT /apis/cluster.k8s.io/v1alpha1/namespaces/default/machines/capv-mgmt-example

**Info  (4)**
Finding machine=host1

GET /apis/cluster.k8s.io/v1alpha1/namespaces/default/machines

caches populated

**Trace Header (5)**
Found machine=host1 id=123

**Trace Detail (6)**
Machine JSON, Request,Response Body