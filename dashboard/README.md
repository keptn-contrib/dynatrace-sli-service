# Create a Dynatrace Dashboard with meta-data for SLO/SLI extraction

This short how-to creates a pre-defined Dashboard in Dynatrace that is configured for retrieving SLO/SLI by using the dynatrace-sli-service.

Dynatrace Dashboard endpoint: '/api/config/v1/dashboards/'

## Using CURL

```console
DYNATRACE_TENANT=*****
DYNATRACE_ENDPOINT=$DYNATRACE_TENANT/api/config/v1/dashboards
DYNATRACE_TOKEN=*****
```

```console
curl -X POST  ${DYNATRACE_ENDPOINT} -H "accept: application/json; charset=utf-8" -H "Authorization: Api-Token ${DYNATRACE_TOKEN}" -H "Content-Type: application/json; charset=utf-8" -d "{\"metadata\":{\"configurationVersions\":[4,2],\"clusterVersion\":\"Mock version\"},\"dashboardMetadata\":{\"name\":\"Example Dashboard\",\"shared\":true,\"sharingDetails\":{\"linkShared\":true,\"published\":false},\"dashboardFilter\":{\"timeframe\":\"l_72_HOURS\",\"managementZone\":{\"id\":\"3438779970106539862\",\"name\":\"Example Management Zone\"}}},\"tiles\":[{\"name\":\"Hosts\",\"tileType\":\"HEADER\",\"configured\":true,\"bounds\":{\"top\":0,\"left\":0,\"width\":304,\"height\":38},\"tileFilter\":{}},{\"name\":\"Applications\",\"tileType\":\"HEADER\",\"configured\":true,\"bounds\":{\"top\":0,\"left\":304,\"width\":304,\"height\":38},\"tileFilter\":{}},{\"name\":\"Host health\",\"tileType\":\"HOSTS\",\"configured\":true,\"bounds\":{\"top\":38,\"left\":0,\"width\":304,\"height\":304},\"tileFilter\":{\"managementZone\":{\"id\":\"3438779970106539862\",\"name\":\"Example Management Zone\"}},\"chartVisible\":true},{\"name\":\"Application health\",\"tileType\":\"APPLICATIONS\",\"configured\":true,\"bounds\":{\"top\":38,\"left\":304,\"width\":304,\"height\":304},\"tileFilter\":{\"managementZone\":{\"id\":\"3438779970106539862\",\"name\":\"Example Management Zone\"}},\"chartVisible\":true}]}"
```

```console
curl -X POST  ${DYNATRACE_ENDPOINT} -H "accept: application/json; charset=utf-8" -H "Authorization: Api-Token ${DYNATRACE_TOKEN}" -H "Content-Type: application/json; charset=utf-8" -d @./slo_sli_dashboard.json
```
