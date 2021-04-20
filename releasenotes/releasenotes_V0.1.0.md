# Release Notes 0.1.0

This service is used for retrieving metrics from the Dynatrace Metrics API endpoint. 

By default, the following metrics are available and pre-configured for Dynatrace:

 - throughput
 - error_rate
 - request_latency_p50
 - request_latency_p90
 - request_latency_p95

## New Features
- 

## Fixed Issues
- 

## Known Limitations

The Dynatrace Metrics API provides data with the "eventually consistency" approach. Therefore, the metrics data 
 retrieved can be incomplete or even contain inconsistencies in case of time frames that are within two hours of the
 current datetime. Usually it takes a minute to catch up, but in extreme situations this might not be enough. 
 We try to mitigate that by delaying the API Call to the metrics API by 60 seconds.