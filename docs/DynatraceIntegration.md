# Dynatrace Integration

Within this document the integration of this service with Dynatrace is described.

## Tenant and API Token

The Dynatrace Tenant and an API Token need to be stored in a `.yaml` file and added as a Kubernetes secret using
```console
kubectl create secret generic dynatrace-credentials-${PROJECTNAME} -n "keptn" --from-file=dynatrace-credentials=dynatrace-credentials.yaml
```
where `${PROJECTNAME}` is the name of the project as specified within the `keptn create project` command (e.g., `sockshop`).

## Tags

The following tags are expected to be set on the monitored services within Dynatrace:

* `environment` - maps to the stage within Keptn (basically the Kubernetes namespace, e.g., `sockshop-dev`, `sockshop-staging` or `sockshop-production`)
* `service` - maps to the name of the service within Keptn (e.g., `carts`)

![alt text](assets/dynatrace_service_tags.png)

## Timeseries Mapping

The following metrics are supported by Keptn

* Throughput (number of requests per second that have been processed)
* ErrorRate (fraction of all received requests that produced an error)
* RequestLatency (how long it takes to return a response to a request)
    * RequestLatencyP50 
    * RequestLatencyP90
    * RequestLatencyP95

and mapped to Dynatrace timeseries data as follows:

| Name               | TimeseriesIdentifier                         | AggregationType               |
|--------------------|----------------------------------------------|-------------------------------|
| Throughput         | com.dynatrace.builtin:service.requestspermin | count                         | 
| ErrorRate          | com.dynatrace.builtin:service.failurerate    | avg                           |
| RequestLatencyP50  | com.dynatrace.builtin:service.responsetime   | percentile (`percentile=50`)  |
| RequestLatencyP90  | com.dynatrace.builtin:service.responsetime   | percentile (`percentile=90`)  |
| RequestLatencyP95  | com.dynatrace.builtin:service.responsetime   | percentile (`percentile=95`)  |

More information about timeseries and available metrics can be found 
[here](https://www.dynatrace.com/support/help/shortlink/api-metrics#services).

## Result Data

A result looks as follows:

```json
{
    "result": {
        "dataPoints": {
            "SERVICE-IDENTIFIER-123": [ [ TIMESTAMP1, VALUE1 ] ],
            "SERVICE-IDENTIFIER-ABC": [ [ TIMESTAMP2, VALUE2 ] ],
            "SERVICE-IDENTIFIER-XYZ": [ [ TIMESTAMP3, VALUE3 ] ]
        },
        "unit": "MicroSecond (µs)",
        "resolutionInMillisUTC": 21600000,
        "aggregationType": "AVG",
        "entities": {
            "SERVICE-IDENTIFIER-123": "ItemsController",
            "SERVICE-IDENTIFIER-ABC": "HealthCheckController",
            "SERVICE-IDENTIFIER-XYZ": "carts"
        },
        "timeseriesId": "com.dynatrace.builtin:service.responsetime"
    }
}
```

The relevant data is stored within `dataPoints` in the list `[ TIMESTAMP, VALUE ]`. However the API might return more
 than one service-identifier. To select the value of a specific service identifier a `customFilters` entry needs to be
 specified, e.g.:

```json
    "customFilters": [
      { "key" : "dynatraceEntityName", "value": "HealthCheckController" }
    ]
```

By iterating over `entities` in the response, the service identifier `SERVICE-IDENTIFIER-ABC` would be selected,
 which results in `VALUE2` to be selected.

## Keptn Performance Tests

If performance tests are triggered within Keptn, an additional tag is automatically set within Dynatrace: 
 `test-subject:true`. This enables us to separate services that have only been created for testing (e.g., new artifacts) 
 from services that have been deployed before.