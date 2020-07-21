# Sample for Dynatrace Dashboard Integration

This should work for any Keptn Project where *dynatrace-sli-service* is enabled.
For a new project where you never ran any evaluation you need to make sure to have an empty slo.yaml uploaded - otherwise the lighthouse service will skip the evaluation:
You can use the empty SLO.yaml that is part of this sample folder:

```
keptn add-resource --project=dynatrace --stage=qualitygate --service=sampleservice --resource=slo.yaml
```

## Create a test project just for SLI retrieval

There is a way to bypass the initial lighthouse step and just trigger the SLI Retrieval and final SLO Evaluation by sending an internal getsli event.
To setup a new project like this you can do the following -> just make sure you have *dynatrace-sli-service* properly configured and that you have a Dashboard in Dynatrace that matches your project name, serivce and stage!

```
Step 1: Create Project and Service
keptn create project dynatrace --shipyard=./shipyard.yaml
keptn create service sampleservice --project=dynatrace

Step 2: Enable Dynatrace-SLI-Provider
CHECK DOC for creating the ConfigMap Entry

Step 3: Ensure Dynatrace Dashboard on your tenant

Step 4: Trigger getsli event -> you need to EDIT that json to match your project, service, stage as well as timeframe
keptn send event --file=./send-getsli.json
```