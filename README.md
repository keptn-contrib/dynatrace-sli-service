# Dynatrace SLI Service
![GitHub release (latest by date)](https://img.shields.io/github/v/release/keptn-contrib/dynatrace-sli-service)
[![Build Status](https://travis-ci.org/keptn-contrib/dynatrace-sli-service.svg?branch=master)](https://travis-ci.org/keptn-contrib/dynatrace-sli-service)
[![Go Report Card](https://goreportcard.com/badge/github.com/keptn-contrib/dynatrace-sli-service)](https://goreportcard.com/report/github.com/keptn-contrib/dynatrace-sli-service)

The *dynatrace-sli-service* is a [Keptn](https://keptn.sh) service that is responsible for retrieving the values of Keptn-supported SLIs from a Dynatrace API endpoint.

The supported default SLIs are:

 - Throughput
 - Error rate
 - Response time p50
 - Response time p90
 - Response time p95

## Deploy in your Kubernetes cluster

To deploy the current version of the *dynatrace-sli-service* in your Keptn Kubernetes cluster, use the `deploy/*.yaml` files from this repository and apply them:

```console
kubectl apply -f deploy/service.yaml
```

## Delete in your Kubernetes cluster

To delete a deployed *dynatrace-sli-service*, use the file `deploy/*.yaml` files from this repository and delete the Kubernetes resources:

```console
kubectl delete -f deploy/service.yaml
```

## Settings / Secrets

To use a Dynatrace tenant other than the one that is being managed by Keptn for a certain project, a secret containing the tenant ID and API token has to be deployed into the `keptn` namespace. The secret must have the following format:

```yaml
DT_TENANT: your_tenant_id.live.dynatracelabs.com
DT_API_TOKEN: XYZ123456789
```

If this information is stored in a file, e.g. `dynatrace-creds.yaml`, it can be stored with the following command (don't forget to replace the `<project>` placeholder with the name of your project:

```console
kubectl create secret generic dynatrace-credentials-<project> -n "keptn" --from-file=dynatrace-credentials=your_credential_file.yaml
```

Please note that there is a naming convention for the secret because this can be configured per **project**. Therefore, the secret has to have the name `dynatrace-credentials-<project>`. An example credentials file is available in: [misc/dynatrace-credentials.yaml](misc/dynatrace-credentials.yaml).

## How does this service work internally

To fetch data this service queries ``https://{DT_TENANT_ID}/api/v2/metrics/series/{timeseriesIdentifier}``. You can find more information in [docs/DynatraceIntegration.md](docs/DynatraceIntegration.md).


### Global

The default SLI queries for this service are defined as follows: 

```yaml
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

### Custom SLI queries

Users can override the predefined queries, as well as add custom queries by creating a SLI configuration. 

* A SLI configuration is a yaml file as shown below:

    ```yaml
    ---
    spec_version: '1.0'
    indicators:
      throughput: "builtin:service.requestCount.total:merge(0):count?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT)"
      error_rate: "builtin:service.errors.total.count:merge(0):avg?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT)"
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

These tags are used for querying the service in question using the `scope=` parameter of the metrics API, e.g.:

```
scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT)
```

## Known Limitations

The Dynatrace Metrics API provides data with the "eventually consistency" approach. Therefore, the metrics data retrieved can be incomplete or even contain inconsistencies in case of time frames that are within two hours of the current datetime. Usually it takes a minute to catch up, but in extreme situations this might not be enough. We try to mitigate that by delaying the API Call to the metrics API by 60 seconds.

