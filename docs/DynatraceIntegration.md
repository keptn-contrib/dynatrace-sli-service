# Dynatrace Integration

Within this document the integration of this service with Dynatrace is described.

## Tenant and API Token

The Dynatrace Tenant and an API Token need to be stored in a `.yaml` file and added as a Kubernetes secret using
```console
kubectl create secret generic dynatrace-credentials-<project> -n "keptn" --from-literal="DT_TENANT=$DT_TENANT" --from-literal="DT_API_TOKEN=$DT_API_TOKEN"
```
where `<project>` is the name of the project as specified within the `keptn create project` command (e.g., `sockshop`).

## Tags

The following tags are expected to be set on the monitored **services** within Dynatrace:

* `keptn_project`, e.g., `sockshop` (maps to the project name defined by `keptn create project ...`)
* `keptn_stage`, e.g., `dev`, `staging`, ... (maps to the stages defined in your shipyard.yaml)
* `keptn_service`, e.g., `carts` (maps to the service onboarded/created within Keptn)
* `keptn_deployment`, e.g., `primary`, `canary` or `direct` (depends on the deployment-strategy defined in your shipyard.yaml)

![alt text](assets/dynatrace_service_tags.png)

These tags are used for querying the service in question using the `entitySelector=` parameter of the metrics API, e.g.:

```
entitySelector=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT),type(SERVICE)
```

For these tags to show up in Dynatrace, two steps are necessary:

1. Add these tags to your deployments environment variable `DT_CUSTOM_PROP` (e.g., [keptn/examples/onboarding-carts/carts/templates/deployment.yaml](https://github.com/keptn/examples/blob/8c1aeb70bf17a826f02fb181db03a6c5947d803d/onboarding-carts/carts/templates/deployment.yaml#L29-L30))
1. Add an automated tagging rule (see https://keptn.sh/docs/0.6.0/reference/monitoring/dynatrace/ for a reference)

## Metrics/Timeseries Mapping

The following metrics/timeseries are automatically supported by the Dynatrace-SLI-Service:

* Throughput (number of requests per second that have been processed)
* ErrorRate (fraction of all received requests that produced an error)
* ResponseTime (how long it takes to return a response to a request)
    * ResponseTimeP50 
    * ResponseTimeP90
    * ResponseTimeP95

and mapped to Dynatrace metrics as follows:

| Name               | Metric                                          | AggregationType[^2]           |
|--------------------|-------------------------------------------------|-------------------------------|
| Throughput         | builtin:service.requestCount.total              | `:merge(0):sum`               |
| ErrorRate          | builtin:service.errors.total.count              | `:merge(0):avg`               |
| ResponseTimeP50    | builtin:service.response.time[^1]               | `:merge(0):percentile(50)`    |
| ResponseTimeP90    | builtin:service.response.time[^1]               | `:merge(0):percentile(90)`    |
| ResponseTimeP95    | builtin:service.response.time[^1]               | `:merge(0):percentile(95)`    |

More information about timeseries and available metrics can be found 
[here](https://www.dynatrace.com/support/help/extend-dynatrace/dynatrace-api/environment-api/metric/).

[^1] service.response.time is returned in microseconds by Dynatrace API, and converted to milliseconds within this service.
[^2] AggregationType needs to contain `merge(0)` such that the first dimension (which is the entity) is aggregated into a single value

## Result Data

A result looks as follows (e.g., for the metric ResponseTimeP50):

```json
{
    "totalCount": 1,
    "nextPageKey": null,
    "result": [
        {
            "metricId": "builtin:service.response.time:merge(0):percentile(50)",
            "data": [
                {
                    "dimensions": [],
                    "timestamps": [
                        1579097520000
                    ],
                    "values": [
                        8433.40
                    ]
                }
            ]
        }
    ]
}
```
