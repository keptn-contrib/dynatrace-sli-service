# Dynatrace SLI Service
![GitHub release (latest by date)](https://img.shields.io/github/v/release/keptn-contrib/dynatrace-sli-service)
[![Build Status](https://travis-ci.org/keptn-contrib/dynatrace-sli-service.svg?branch=master)](https://travis-ci.org/keptn-contrib/dynatrace-sli-service)
[![Go Report Card](https://goreportcard.com/badge/github.com/keptn-contrib/dynatrace-sli-service)](https://goreportcard.com/report/github.com/keptn-contrib/dynatrace-sli-service)

The *dynatrace-sli-service* is a [Keptn](https://keptn.sh) service that is responsible for retrieving the values of Keptn-supported SLIs from a Dynatrace Metrics API endpoint.

By default, the following SLIs are automatically supported:

 - Throughput
 - Error rate
 - Response time p50
 - Response time p90
 - Response time p95


## Compatibility Matrix

| Keptn Version    | [Dynatrace-SLI-Service Service Image](https://hub.docker.com/r/keptncontrib/dynatrace-sli-service/tags) |
|:----------------:|:----------------------------------------:|
|       0.6.0      | keptncontrib/dynatrace-sli-service:0.3.0 |
|       0.6.1      | keptncontrib/dynatrace-sli-service:0.4.0[^1] |

[^1] Not available yet

## Installation

The *dynatrace-sli-service* can be installed as a part of [Keptn's uniform](https://keptn.sh).

### Deploy in your Kubernetes cluster

To deploy the current version of the *dynatrace-sli-service* in your Keptn Kubernetes cluster, use the `deploy/*.yaml` files from this repository and apply them:

```console
kubectl apply -f deploy/service.yaml
```

This should install the `dynatrace-sli-service` together with a Keptn `distributor` into the `keptn` namespace, which you can verify using

```console
kubectl -n keptn get deployment dynatrace-sli-service -o wide
kubectl -n keptn get pods -l run=dynatrace-sli-service
```

### Up- or Downgrading

Adapt and use the following command in case you want to up- or downgrade your installed version (specified by the `$VERSION` placeholder):

```console
kubectl -n keptn set image deployment/dynatrace-sli-service dynatrace-sli-service=keptncontrib/dynatrace-sli-service:$VERSION --record
```

### Uninstall

To delete a deployed *dynatrace-sli-service*, use the file `deploy/*.yaml` files from this repository and delete the Kubernetes resources:

```console
kubectl delete -f deploy/service.yaml
```


## Setup

By default, the dynatrace-sli-service will use the same tenant and api token as provided for the [dynatrace-service](https://github.com/keptn-contrib/dynatrace-service).
In case you do not use the dynatrace-service, or if you want to use another Dynatrace tenant for a certain project, a secret containing the tenant ID and API token has to be deployed into the `keptn` namespace. The secret must be stored in a file, e.g., `your-dynatrace-creds.yaml` with the following format:

```yaml
DT_TENANT: your_tenant_id.live.dynatracelabs.com
DT_API_TOKEN: XYZ123456789
```

You need to add this file as a secret to the `keptn` namespace as follows (replace the `<project>` placeholder with the name of your project):

```console
kubectl create secret generic dynatrace-credentials-<project> -n "keptn" --from-file=dynatrace-credentials=your-dynatrace-creds.yaml
```

Please note that there is a naming convention for the secret because this can be configured per **project**. Therefore, the secret has to have the name `dynatrace-credentials-<project>`. An example credentials file is available in: [misc/dynatrace-credentials.yaml](misc/dynatrace-credentials.yaml).

## SLI Configuration

The default SLI queries for this service are defined as follows: 

```yaml
indicators:
 throughput: "metricSelector=builtin:service.requestCount.total:merge(0):sum&entitySelector=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT),type(SERVICE)"
 error_rate: "metricSelector=builtin:service.errors.total.count:merge(0):avg&entitySelector=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT),type(SERVICE)"
 response_time_p50: "metricSelector=builtin:service.response.time:merge(0):percentile(50)&entitySelector=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT),type(SERVICE)"
 response_time_p90: "metricSelector=builtin:service.response.time:merge(0):percentile(90)&entitySelector=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT),type(SERVICE)"
 response_time_p95: "metricSelector=builtin:service.response.time:merge(0):percentile(95)&entitySelector=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT),type(SERVICE)"
```

Please note that we require the following tags on the services and within the query:

* `keptn_project`
* `keptn_stage`
* `keptn_service`
* `keptn_deployment`

If you want to use other tags, you need to overwrite the SLI configuration (see below).

### Overwrite SLI Configuration / Custom SLI queries

Users can override the predefined queries, as well as add custom queries by creating a SLI configuration: 

* A custom SLI configuration is a yaml file as shown below:

    ```yaml
    ---
    spec_version: '1.0'
    indicators:
      your_metric: "metricSelector=your_metric:count&entitySelector=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT),type(SERVICE)"
    ```

* To store this configuration, you need to add this file to a Keptn's configuration store. This is done by using the Keptn CLI with the [add-resource](https://keptn.sh/docs/0.6.0/reference/cli/#keptn-add-resource) command. 

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

These tags are used for querying the service in question using the `entitySelector=` parameter of the metrics API, e.g.:

```
entitySelector=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT),type(SERVICE)
```

## Known Limitations

* The Dynatrace Metrics API provides data with the "eventually consistency" approach. Therefore, the metrics data retrieved can be incomplete or even contain inconsistencies in case of time frames that are within two hours of the current datetime. Usually it takes a minute to catch up, but in extreme situations this might not be enough. We try to mitigate that by delaying the API Call to the metrics API by 60 seconds.
* The Dynatrace Metrics API is available as an early-access feature.
