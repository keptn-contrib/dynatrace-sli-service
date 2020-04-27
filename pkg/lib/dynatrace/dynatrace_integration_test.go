package dynatrace

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/keptn-contrib/dynatrace-sli-service/pkg/common"
	"github.com/keptn/go-utils/pkg/events"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

// create a fake http client for integration tests
func testingHTTPClient(handler http.Handler) (*http.Client, func()) {
	s := httptest.NewServer(handler)

	cli := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, network, _ string) (net.Conn, error) {
				return net.Dial(network, s.Listener.Addr().String())
			},
		},
	}

	return cli, s.Close
}

// tests the GETSliValue function to return the proper datapoint
func TestGetSLIValue(t *testing.T) {

	okResponse := `{
		"totalCount": 8,
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
	}`

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okResponse))
	})

	httpClient, teardown := testingHTTPClient(h)
	defer teardown()

	keptnEvent := &common.BaseKeptnEvent{}
	keptnEvent.Project = "sockshop"
	keptnEvent.Stage = "dev"
	keptnEvent.Service = "carts"
	keptnEvent.DeploymentStrategy = ""

	// dh := NewDynatraceHandler("http://dynatrace", "sockshop", "dev", "carts", nil, nil, "")
	dh := NewDynatraceHandler("http://dynatrace", keptnEvent, nil, nil)
	dh.HTTPClient = httpClient

	start := time.Unix(1571649084, 0).UTC().Format(time.RFC3339)
	end := time.Unix(1571649085, 0).UTC().Format(time.RFC3339)
	value, err := dh.GetSLIValue(ResponseTimeP50, start, end, nil)

	assert.NoError(t, err)

	assert.InDelta(t, 8.43340, value, 0.001)
}

// tests the GETSliValue function to return the proper datapoint with the old custom query format
func TestGetSLIValueWithOldandNewCustomQueryFormat(t *testing.T) {

	okResponse := `{
		"totalCount": 8,
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
	}`

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okResponse))
	})

	httpClient, teardown := testingHTTPClient(h)
	defer teardown()

	keptnEvent := &common.BaseKeptnEvent{}
	keptnEvent.Project = "sockshop"
	keptnEvent.Stage = "dev"
	keptnEvent.Service = "carts"
	keptnEvent.DeploymentStrategy = ""

	dh := NewDynatraceHandler("http://dynatrace", keptnEvent, nil, nil)
	dh.HTTPClient = httpClient

	// overwrite custom queries with the new format (starting with metricSelector=)
	dh.CustomQueries = make(map[string]string)
	dh.CustomQueries[ResponseTimeP50] = "metricSelector=builtin:service.response.time:merge(0):percentile(50)&entitySelector=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT),type(SERVICE)"

	start := time.Unix(1571649084, 0).UTC().Format(time.RFC3339)
	end := time.Unix(1571649085, 0).UTC().Format(time.RFC3339)
	value, err := dh.GetSLIValue(ResponseTimeP50, start, end, nil)

	assert.EqualValues(t, nil, err)
	assert.InDelta(t, 8.43340, value, 0.001)

	// now do the same but with the new format but with ?metricSelector= in front (the ? is not needed/wanted)
	dh.CustomQueries = make(map[string]string)
	dh.CustomQueries[ResponseTimeP50] = "?metricSelector=builtin:service.response.time:merge(0):percentile(50)&entitySelector=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT),type(SERVICE)"

	start = time.Unix(1571649084, 0).UTC().Format(time.RFC3339)
	end = time.Unix(1571649085, 0).UTC().Format(time.RFC3339)
	value, err = dh.GetSLIValue(ResponseTimeP50, start, end, nil)

	assert.EqualValues(t, nil, err)
	assert.InDelta(t, 8.43340, value, 0.001)

	// now do the same but with the old format ($metricName?scope=...)
	dh.CustomQueries = make(map[string]string)
	dh.CustomQueries[ResponseTimeP50] = "builtin:service.response.time:merge(0):percentile(50)?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT)"

	start = time.Unix(1571649084, 0).UTC().Format(time.RFC3339)
	end = time.Unix(1571649085, 0).UTC().Format(time.RFC3339)
	value, err = dh.GetSLIValue(ResponseTimeP50, start, end, nil)

	assert.EqualValues(t, nil, err)
	assert.InDelta(t, 8.43340, value, 0.001)
}

// Tests GetSLIValue with an empty result (no datapoints)
func TestGetSLIValueWithEmptyResult(t *testing.T) {

	okResponse := `{
    "totalCount": 4,
    "nextPageKey": null,
	"result": [
		{
			"metricId": "builtin:service.response.time:merge(0):percentile(50)",
			"data": [
			]
		}
	]
}`

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okResponse))
	})

	httpClient, teardown := testingHTTPClient(h)
	defer teardown()

	keptnEvent := &common.BaseKeptnEvent{}
	keptnEvent.Project = "sockshop"
	keptnEvent.Stage = "dev"
	keptnEvent.Service = "carts"
	keptnEvent.DeploymentStrategy = ""

	dh := NewDynatraceHandler("http://dynatrace", keptnEvent, nil, nil)
	dh.HTTPClient = httpClient

	start := time.Unix(1571649084, 0).UTC().Format(time.RFC3339)
	end := time.Unix(1571649085, 0).UTC().Format(time.RFC3339)
	value, err := dh.GetSLIValue(ResponseTimeP50, start, end, nil)

	assert.EqualValues(t, errors.New("Dynatrace Metrics API returned 0 result values, expected 1. Please ensure the response contains exactly one value (e.g., by using :merge(0):avg for the metric)"), err)

	assert.EqualValues(t, 0.0, value)
}

// Tests GetSLIValue without the expected metric in it
func TestGetSLIValueWithoutExpectedMetric(t *testing.T) {

	okResponse := `{
		"totalCount": 4,
		"nextPageKey": null,
		"result": [
			{
				"metricId": "something_else",
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
	}`

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okResponse))
	})

	httpClient, teardown := testingHTTPClient(h)
	defer teardown()

	keptnEvent := &common.BaseKeptnEvent{}
	keptnEvent.Project = "sockshop"
	keptnEvent.Stage = "dev"
	keptnEvent.Service = "carts"
	keptnEvent.DeploymentStrategy = ""

	dh := NewDynatraceHandler("http://dynatrace", keptnEvent, nil, nil)
	dh.HTTPClient = httpClient

	start := time.Unix(1571649084, 0).UTC().Format(time.RFC3339)
	end := time.Unix(1571649085, 0).UTC().Format(time.RFC3339)
	value, err := dh.GetSLIValue(ResponseTimeP50, start, end, nil)

	assert.EqualValues(t, errors.New("Dynatrace Metrics API result does not contain identifier builtin:service.response.time:merge(0):percentile(50)"), err)

	assert.EqualValues(t, 0.0, value)
}

// Tests what happens if the end-time is in the future
func TestGetSLIEndTimeFuture(t *testing.T) {
	keptnEvent := &common.BaseKeptnEvent{}
	keptnEvent.Project = "sockshop"
	keptnEvent.Stage = "dev"
	keptnEvent.Service = "carts"
	keptnEvent.DeploymentStrategy = ""

	dh := NewDynatraceHandler("http://dynatrace", keptnEvent, nil, nil)

	start := time.Now().Format(time.RFC3339)
	// artificially increase end time to be in the future
	end := time.Now().Add(1 * time.Minute).Format(time.RFC3339)
	value, err := dh.GetSLIValue(Throughput, start, end, []*events.SLIFilter{})

	assert.EqualValues(t, 0.0, value)
	assert.NotNil(t, err, nil)
	assert.EqualValues(t, "end time must not be in the future", err.Error())
}

// Tests what happens if start-time is after end-time
func TestGetSLIStartTimeAfterEndTime(t *testing.T) {
	keptnEvent := &common.BaseKeptnEvent{}
	keptnEvent.Project = "sockshop"
	keptnEvent.Stage = "dev"
	keptnEvent.Service = "carts"
	keptnEvent.DeploymentStrategy = ""

	dh := NewDynatraceHandler("http://dynatrace", keptnEvent, nil, nil)

	start := time.Now().Format(time.RFC3339)
	// artificially increase end time to be in the future
	end := time.Now().Add(-1 * time.Minute).Format(time.RFC3339)
	value, err := dh.GetSLIValue(Throughput, start, end, []*events.SLIFilter{})

	assert.EqualValues(t, 0.0, value)
	assert.NotNil(t, err, nil)
	assert.EqualValues(t, "start time needs to be before end time", err.Error())
}

// Tests what happens when end time is too close to now
func TestGetSLISleep(t *testing.T) {
	okResponse := `{
		"totalCount": 3,
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
	}`

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okResponse))
	})

	httpClient, teardown := testingHTTPClient(h)
	defer teardown()

	keptnEvent := &common.BaseKeptnEvent{}
	keptnEvent.Project = "sockshop"
	keptnEvent.Stage = "dev"
	keptnEvent.Service = "carts"
	keptnEvent.DeploymentStrategy = ""

	dh := NewDynatraceHandler("http://dynatrace", keptnEvent, nil, nil)
	dh.HTTPClient = httpClient

	start := time.Now().Add(-5 * time.Minute).Format(time.RFC3339)
	// artificially increase end time to be in the future
	end := time.Now().Add(-80 * time.Second).Format(time.RFC3339)
	value, err := dh.GetSLIValue(ResponseTimeP50, start, end, []*events.SLIFilter{})

	assert.InDelta(t, 8.43340, value, 0.001)
	assert.Nil(t, err)
}

// Tests the behaviour of the GetSLIValue function in case of a HTTP 400 return code
func TestGetSLIValueWithErrorResponse(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// w.Write([]byte(response))
		w.WriteHeader(http.StatusBadRequest)
	})

	httpClient, teardown := testingHTTPClient(h)
	defer teardown()

	keptnEvent := &common.BaseKeptnEvent{}
	keptnEvent.Project = "sockshop"
	keptnEvent.Stage = "dev"
	keptnEvent.Service = "carts"
	keptnEvent.DeploymentStrategy = ""

	dh := NewDynatraceHandler("http://dynatrace", keptnEvent, nil, nil)
	dh.HTTPClient = httpClient

	start := time.Unix(1571649084, 0).UTC().Format(time.RFC3339)
	end := time.Unix(1571649085, 0).UTC().Format(time.RFC3339)
	value, err := dh.GetSLIValue(Throughput, start, end, []*events.SLIFilter{})

	assert.EqualValues(t, 0.0, value)
	assert.NotNil(t, err, nil)
}
