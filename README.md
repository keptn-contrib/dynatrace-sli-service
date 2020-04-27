# Dynatrace SLI Service
![GitHub release (latest by date)](https://img.shields.io/github/v/release/keptn-contrib/dynatrace-sli-service)
[![Build Status](https://travis-ci.org/keptn-contrib/dynatrace-sli-service.svg?branch=master)](https://travis-ci.org/keptn-contrib/dynatrace-sli-service)
[![Go Report Card](https://goreportcard.com/badge/github.com/keptn-contrib/dynatrace-sli-service)](https://goreportcard.com/report/github.com/keptn-contrib/dynatrace-sli-service)

The *dynatrace-sli-service* is a [Keptn](https://keptn.sh) service that is responsible for retrieving the values of Keptn-supported SLIs from a Dynatrace Metrics API endpoint.

By default, even if you do not specifiy a custom SLI.yaml, the following SLIs are automatically supported:

 - Throughput
 - Error rate
 - Response time p50
 - Response time p90
 - Response time p95

 These metrics are queried from a Dynatrace monitored Service entity with the tags keptn_project, keptn_service, keptn_stage & keptn_deployment.
![](./images/defaultdynatracetags.png)

 You can however define your own custom SLI.yaml where you are free in defining your own list of metrics coming from all monitored entities in Dynatrace (APPLICATION, SERVICE, PROCESS GROUP INSTANCE, HOST, CUSTOM DEVICE). More details on that further down in the readme.


## Compatibility Matrix

| Keptn Version    | [Dynatrace-SLI-Service Service Image](https://hub.docker.com/r/keptncontrib/dynatrace-sli-service/tags) | Description |
|:----------------:|:----------------------------------------:|--------------------------------------------------|
|       0.6.0      | keptncontrib/dynatrace-sli-service:0.3.0 |  |
|       0.6.1      | keptncontrib/dynatrace-sli-service:0.4.0[^1] | Default installation with Keptn 0.6.1 |
|       0.6.1      | keptncontrib/dynatrace-sli-service:0.5.0 | Added support for dynatrace.conf.yaml to support multiple Dynatrace environments. Also added support of more placeholders in SLI.yaml, e.g: $LABEL.yourlabel |

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

**Using different Dynatrace environment per stage or service!**
If you have different Dynatrace environments to e.g: monitor pre-production and production environments and therefore want the *dynatrace-sli-service* to connect to that respective Dynatrace environment when pulling SLI metrics for a specific Keptn stage or service you can do the following:
#1: Create a secret for your additional Dynatrace environments in the same way as explained above and store them under a meaningful name, e.g: dynatrace-preprod or dynatrace-prod:
```
kubectl create secret generic dynatrace-preprod -n "keptn" --from-file=dynatrace-credentials=your-dynatrace-pre-prod-creds.yaml
```
#2: Define a dynatrace.conf.yaml resource file which allows you to specify a DTCreds value which has to be name of the secret, e.g: dynatrace-preprod
```yaml
spec_version: '0.1.0'
dtCreds: dynatrace-preprod
```
#3: Upload that dynatrace.conf.yaml to your Keptn project, stage or service via keptn add-resource (either CLI or API) into the dynatace folder, e.g: here is an example to upload it to a specific stage which means the *dynatrace-sli-service* will use the credentials stored in *dynatrace-preprod* secret for every SLI retrieval on that stage
```console
keptn add-resource --project=yourproject --stage=yourstage --resource=./dynatrace.conf.yaml --resourceUri=dynatrace/dynatrace.conf.yaml
```


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

Please note that the default SLI queries require the following tags on the services and within the query:

* `keptn_project`
* `keptn_stage`
* `keptn_service`
* `keptn_deployment`

When Keptn queries these SLIs for e.g., the service `carts` in the stage `dev` within project `sockshop` it would translate to the following tags in the query:

* `keptn_project:sockshop`
* `keptn_stage:dev`
* `keptn_service:carts`
* `keptn_deployment:primary` (or `keptn_deployment:canary` during tests)

If you use Keptn for deployment of your artifacts using Keptn's Helm Service you will have these 4 tags automatically set and detected by Dynatrace.
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

* To store this configuration, you need to add this file to a Keptn's configuration store either on project, stage or service level. The remote resourceUri needs to be dynatrace/sli.yaml. This is done by using the Keptn CLI with the [add-resource](https://keptn.sh/docs/0.6.0/reference/cli/#keptn-add-resource) command. Here is an example

```console
keptn add-resource --project=yourproject --stage=yourstage --service=yourservice --resource=./sli.yaml --resourceUri=dynatrace/sli.yaml
```

### More examples on custom SLIs

You can define your own SLI.yaml that defines ANY type of metric available in Dynatrace - on ANY entity type (APPLICATION, SERVICE, PROCESS GROUP, HOST, CUSTOM DEVICE ...). You can either "hard-code" the queries in your SLI.yaml or you can use placeholders such as $SERVICE, $STAGE, $PROJECT, $DEPLOYMENT as well as $LABEL.yourlabel1, $LABEL.yourlabel2. This is very powerful as you can define generic SLI.yamls and leverage the dynamic data of a Keptn Event. 
Here is an example where we are retrieving the tag name from a label that is passed to Keptn

```yaml
indicators:
    throughput:  "metricSelector=builtin:service.requestCount.total:merge(0):sum&entitySelector=tag($LABEL.dttag),type(SERVICE)"
```

So - if you are sending an event to Keptn and passing in a label with the name dttag and a value e.g: "evaluateforsli" then it will match a Dynatrace service that has this tag on it:
![](./images/dynatrace_tag_evaluateforsli.png)

You can also have SLIs that span multiple layers of your stack, e.g: services, process groups and host metrics. Here is an example that queries one metric from a service, one from a process group and one from a host. The tag names come from labels that are sent to Keptn
```yaml
indicators:
    throughput:  "metricSelector=builtin:service.requestCount.total:merge(0):sum&entitySelector=tag($LABEL.dtservicetag),type(SERVICE)"
    gcheapuse:   "metricSelector=builtin:tech.nodejs.v8heap.gcHeapUsed:merge(0):sum&entitySelector=tag($LABEL.dtpgtag),type(PROCESS_GROUP_INSTANCE)"
    hostmemory:  "metricSelector=builtin:host.mem.usage:merge(0):avg&entitySelector=tag($LABEL.dthosttag),type(HOST)"
```

Hope these examples help you see what is possible. If you want to explore more about Dynatrace Metrics, and the queries you need to create to extract them I suggest you explore the Dynatrace API Explorer (Swagger UI) as well as the [Metric API v2](https://www.dynatrace.com/support/help/extend-dynatrace/dynatrace-api/environment-api/metric-v2/) documentation


## Known Limitations

* The Dynatrace Metrics API provides data with the "eventually consistency" approach. Therefore, the metrics data retrieved can be incomplete or even contain inconsistencies in case of time frames that are within two hours of the current datetime. Usually it takes a minute to catch up, but in extreme situations this might not be enough. We try to mitigate that by delaying the API Call to the metrics API by 60 seconds.
* This service uses the Dynatrace Metrics v2 API by default but can also parse V1 metrics query. If you use the V1 query language you will see warning log outputs in the *dynatrace-sli-service* which encourages you to update your queries to V2. More information about Metics V2 API can be found in the [Dynatrace documentation](https://www.dynatrace.com/support/help/extend-dynatrace/dynatrace-api/environment-api/metric-v2/)
