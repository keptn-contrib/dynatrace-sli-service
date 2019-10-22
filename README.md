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

## Go Utils - different branch at the moment
```bash
go get github.com/keptn/go-utils@feature/950/evaluation-events
```

## Settings / Secrets

The following settings are needed in the form as a credentials file:

* tenant id for Dynatrace API
* API token for Dynatrace API

E.g.:
```yaml
tenant: your_tenant_id.live.dynatracelabs.com
apiToken: XYZ123456789
```
Add the credential in the **keptn namespace** using
```console
kubectl create secret generic dynatrace-credentials-${PROJECTNAME} -n "keptn" --from-file=dynatrace-credentials=your_credential_file.yaml
```
where `${PROJECTNAME}` is the name of the project (e.g., `sockshop`).

## How does this service work internally

To fetch data this service queries ``https://{DT_TENANT_ID}/api/v1/timeseries/{timeseriesIdentifier}/``, where 
 `timeseriesIdentifier` can be one of

* `com.dynatrace.builtin:dcrum.service.serverthroughput` for throughput
* `com.dynatrace.builtin:app.custom.webrequest.errorcount` for error count
* `com.dynatrace.builtin:service.responsetime` for request latency (p50, p90 and p95)

## Integration with Dynatrace

We expect each service to have the following tags within Dynatrace:

* ``service:${event.data.service}``
* ``environment:${event.data.project}-${event.data.stage}``

E.g., the service `carts` in the stage `dev` within project `sockshop` would have the following tags:

* ``service:carts``
* ``environment:sockshop-dev`` (this is essentially the kubernetes namespace)

