# Dynatrace SLI Service
![GitHub release (latest by date)](https://img.shields.io/github/v/release/keptn-contrib/dynatrace-sli-service)
[![Build Status](https://travis-ci.org/keptn-contrib/dynatrace-sli-service.svg?branch=master)](https://travis-ci.org/keptn-contrib/dynatrace-sli-service)
[![Go Report Card](https://goreportcard.com/badge/github.com/keptn-contrib/dynatrace-sli-service)](https://goreportcard.com/report/github.com/keptn-contrib/dynatrace-sli-service)

The *dynatrace-sli-service* is a [Keptn](https://keptn.sh) service that is responsible for retrieving the values of Keptn-supported SLIs from a Dynatrace API endpoint.

## Installation

The *dynatrace-sli-service* is installed as a part of [Keptn's uniform](https://keptn.sh).

## Deploy in your Kubernetes cluster

To deploy the current version of the *dynatrace-sli-service* in your Keptn Kubernetes cluster, use the file `deploy/service.yaml` from this repository and apply it:

```console
kubectl apply -f deploy/service.yaml
kubectl apply -f deploy/distributor.yaml
```

## Delete in your Kubernetes cluster

To delete a deployed *dynatrace-sli-service*, use the file `deploy/service.yaml` from this repository and delete the Kubernetes resources:

```console
kubectl delete -f deploy/service.yaml
kubectl delete -f deploy/distributor.yaml
```

## Settings / Secrets

The following settings are needed in the form as a credentials file:

* tenant id for Dynatrace API
* API token for Dynatrace API

E.g.:
```yaml
DT_TENANT: your_tenant_id.live.dynatracelabs.com
DT_API_TOKEN: XYZ123456789
```
Add the credential in the **keptn namespace** using
```console
kubectl create secret generic dynatrace-credentials-${PROJECTNAME} -n "keptn" --from-file=dynatrace-credentials=your_credential_file.yaml
```
where `${PROJECTNAME}` is the name of the project (e.g., `sockshop`). An example credentials file is available in 
 [misc/dynatrace-credentials.yaml](misc/dynatrace-credentials.yaml).

## How does this service work internally

To fetch data this service queries ``https://{DT_TENANT_ID}/api/v1/timeseries/{timeseriesIdentifier}/``, where 
 `timeseriesIdentifier` can be one of

* `com.dynatrace.builtin:dcrum.service.serverthroughput` for throughput
* `com.dynatrace.builtin:app.custom.webrequest.errorcount` for error count
* `com.dynatrace.builtin:service.responsetime` for response time (p50, p90 and p95)

## Custom Metrics/Timeseries Identifier

You can overwrite each metric/timeseries identifier as well as the aggregation method using Kubernetes Config Maps on
 for the whole Keptn installation as well as per project. You can also add new metrics.


### Global
```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: dynatrace-sli-config
  namespace: keptn
data:
  custom-queries: |
    throughput: "com.dynatrace.builtin:service.requests,count,0"
    errorRate: "com.dynatrace.builtin:service.failurerate,avg,0"
    myMetric: "whatever.io.metrics:foo.bar,..."
```


### Per Project

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: dynatrace-sli-config-${PROJECT_NAME}
  namespace: keptn
data:
  custom-queries: |
    throughput: "com.dynatrace.builtin:foo.bar,avg,0"
    myMetric: "whatever.io.metrics:foo.bar,..."
```
where `${PROJECT_NAME}` is the name of the project (e.g., `sockshop`). 

## Custom Metrics/Timeseries Identifier

You can overwrite each metric/timeseries identifier as well as the aggregation method using Kubernetes Config Maps on
 for the whole Keptn installation as well as per project. You can also add new metrics.
 

### Default Configuration
The following is the default configuration (which is currently handled within the code, you don't need to apply it). 
 This only serves demonstration purpose and can be used to extend metrics easily.
 
```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: dynatrace-sli-config
  namespace: keptn
data:
  custom-queries: |
    throughput: "com.dynatrace.builtin:service.requestspermin,count,0"
    error_rate: "com.dynatrace.builtin:service.failurerate,avg,0"
    response_time_P50: "com.dynatrace.builtin:service.responsetime,percentile,50"
    response_time_P90: "com.dynatrace.builtin:service.responsetime,percentile,90"
    response_time_P95: "com.dynatrace.builtin:service.responsetime,percentile,95"
```

Please see [docs/DynatraceIntegration.md](docs/DynatraceIntegration.md) for more details about the metrics used.

### Global
For example, we re-define throughput and add two new metrics `errorCount4xx` and `errorCount5xx`:
```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: dynatrace-sli-config
  namespace: keptn
data:
  custom-queries: |
    throughput: "com.dynatrace.builtin:service.requestspermin,count,0"
    error_count_4xx: "com.dynatrace.builtin:service.errorcounthttp4xx,,0"
    error_count_5xx: "com.dynatrace.builtin:service.errorcounthttp5xx,,0"
```


### Per Project
Here we overwrite the `response_time_p50` metric to not use median (p50), but average, and we introduce a new (made up)
 metric `my_metric`.
```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: dynatrace-sli-config-${PROJECT_NAME}
  namespace: keptn
data:
  custom-queries: |
    response_time_p50: "com.dynatrace.builtin:service.responsetime,avg,0"
    my_metric: "whatever.io.metrics:foo.bar,..."
```
where `${PROJECT_NAME}` is the name of the project (e.g., `sockshop`). 

## Integration with Dynatrace

We expect each service to have the following tags within Dynatrace:

* ``service:${event.data.service}``
* ``environment:${event.data.project}-${event.data.stage}``

E.g., the service `carts` in the stage `dev` within project `sockshop` would have the following tags:

* ``service:carts``
* ``environment:sockshop-dev`` (this is essentially the kubernetes namespace)

See [docs/DynatraceIntegration.md](docs/DynatraceIntegration.md) for more details.

