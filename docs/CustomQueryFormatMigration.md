# March 2020 Format Migration

With `dynatrace-sli-service` versions 0.3.0 and 0.2.0 custom SLIs had the following format:

```yaml
indicators:
 generic: "$metricId?scope=$scope"
 throughput: "builtin:service.requestCount.total:splitby():count?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT)"
```

Due to changes within the Dynatrace metrics API this format is no longer valid and needs to be converted. The changes
 include:
 
* use of `metricSelector` for the `$metricId`
* use of `entitySelector` instead of `scope`
* use of `type(SERVICE)` within the `entitySelector`.

Therefore the new format should look as follows:

```yaml
indicators:
 generic: "metricSelector=$metricId&entitySelector=$scope,TYPE(SERVICE)"
 throughput: "metricSelector=builtin:service.requestCount.total:splitby():count&entitySelector=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT),type(SERVICE)"
```

