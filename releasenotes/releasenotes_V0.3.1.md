# Release Notes develop

## New Features

- Support both, the EAP and the new metric syntax (see [docs/CustomQueryFormatMigration.md](../docs/CustomQueryFormatMigration.md))

## Fixed Issues

- [#50](https://github.com/keptn-contrib/dynatrace-sli-service/issues/30) Predefined metric `throughput` was misleading as it returned the number of affected entities rather than the number of requests (`:count` vs. `.sum`)
 
## Known Limitations

