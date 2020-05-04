# Release Notes v0.4.0

## New Features

- Added support for dynatrace.conf.yaml to support multiple Dynatrace environments (sourced from [keptn-contrib/dynatrace-service](https://github.com/keptn-contrib/dynatrace-service)) 
- Refactor secret handling for global and project specific secrets
- Added support of more placeholders in SLI.yaml, e.g: $LABEL.yourlabel [#34](https://github.com/keptn-contrib/dynatrace-sli-service/issues/34)

## Fixed Issues

- Labels are being dropped between the get-sli and the get-sli.done keptn events [#37](https://github.com/keptn-contrib/dynatrace-sli-service/issues/37)
 
## Known Limitations

