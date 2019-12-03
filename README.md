# Dynatrace SLI Service
![GitHub release (latest by date)](https://img.shields.io/github/v/release/keptn-contrib/dynatrace-sli-service)
[![Build Status](https://travis-ci.org/keptn-contrib/dynatrace-sli-service.svg?branch=master)](https://travis-ci.org/keptn-contrib/dynatrace-sli-service)
[![Go Report Card](https://goreportcard.com/badge/github.com/keptn-contrib/dynatrace-sli-service)](https://goreportcard.com/report/github.com/keptn-contrib/dynatrace-sli-service)

The *dynatrace-sli-service* is a [Keptn](https://keptn.sh) service that is responsible for retrieving the values of Keptn-supported SLIs from a Dynatrace API endpoint.

## Installation

The *dynatrace-sli-service* is installed as a part of [Keptn's uniform](https://keptn.sh).

## Deploy in your Kubernetes cluster

To deploy the current version of the *dynatrace-sli-service* in your Keptn Kubernetes cluster, use the `deploy/*.yaml` files from this repository and apply them:

```console
kubectl apply -f deploy/service.yaml
kubectl apply -f deploy/distributor.yaml
```

## Delete in your Kubernetes cluster

To delete a deployed *dynatrace-sli-service*, use the file `deploy/*.yaml` files from this repository and delete the Kubernetes resources:

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

To fetch data this service queries ``https://{DT_TENANT_ID}/api/v2/metrics/series/{timeseriesIdentifier}``. You can
find more information in [docs/DynatraceIntegration.md](docs/DynatraceIntegration.md).


## Custom Metrics/Timeseries Identifier

You can overwrite each metric/timeseries identifier as well as the aggregation method using Kubernetes Config Maps on
 for the whole Keptn installation as well as per project. You can also add new metrics.


### Global

As an example, here is the default configuration:
```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: dynatrace-sli-config
  namespace: keptn
data:
  custom-queries: |
    throughput: "builtin:service.requestCount.total:merge(0):count?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:canary)"
    error_rate: "builtin:service.errors.total.count:merge(0):avg?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:canary)"
    response_time_p50: "builtin:service.response.time:merge(0):percentile(50)?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:canary)"
    response_time_p90: "builtin:service.response.time:merge(0):percentile(90)?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:canary)"
    response_time_p95: "builtin:service.response.time:merge(0):percentile(95)?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:canary)"

```

Please note that the tags that are used are:

* `keptn_project`
* `keptn_stage`
* `keptn_service`
* `keptn_deployment`

If you want to use other tags, you need to overwrite those metrics (see next example).

### Per Project

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: dynatrace-sli-config-PROJECTNAME
  namespace: keptn
data:
  custom-queries: |
    foo_bar: "custom.metric:merge(0):percentile(50)?scope=tag(my-service-tag:$SERVICE),tag(my-environment-tag:$PROJECT-$STAGE)"

```
where `PROJECTNAME` is the name of the project (e.g., `sockshop`). 


## Tags on Dynatrace

We expect each service to have the following tags within Dynatrace:

* `keptn_project`
* `keptn_stage`
* `keptn_service`
* `keptn_deployment`

E.g., the service `carts` in the stage `dev` within project `sockshop` would have the following tags:

* `keptn_project:sockshop`
* `keptn_stage:dev`
* `keptn_service:carts`
* `keptn_deployment:primary` (or `keptn_deployment:canary` during tests)

These tags are used for querying the service in question using the `scope=` parameter of the metrics API, e.g.:
```
scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT)
```

## Known Limitations

The Dynatrace Metrics API provides data with the "eventually consistency" approach. Therefore, the metrics data 
 retrieved can be incomplete or even contain inconsistencies in case of time frames that are within two hours of the
 current datetime. Usually it takes a minute to catch up, but in extreme situations this might not be enough. 
 We try to mitigate that by delaying the API Call to the metrics API by 60 seconds.

