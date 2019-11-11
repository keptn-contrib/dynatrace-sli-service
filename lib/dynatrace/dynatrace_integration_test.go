package dynatrace

import (
	"errors"
	"github.com/keptn/go-utils/pkg/events"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
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
		    "result": {
		        "unit": "MicroSeconds (µs)",
		        "dataPoints": {
					"ENTITY-123": [[ 123, 456 ]],
					"ENTITY-456": [[ 123, 789 ]],
					"ENTITY-789": [[ 123, 999 ]]
				},
				"aggregationType": "PERCENTILE",
				"entities": {
					"ENTITY-123": "NotMyService",
					"ENTITY-456": "MyService",
					"ENTITY-789": "AlsoNotMyService"
				},
				"timeseriesId": "com.dynatrace.builtin:service.responsetime"
		    }
		}`

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okResponse))
	})

	httpClient, teardown := testingHTTPClient(h)
	defer teardown()

	dh := NewDynatraceHandler("http://dynatrace", "sockshop", "dev", "carts", nil)
	dh.HTTPClient = httpClient

	start := strconv.FormatInt(time.Unix(1571649084, 0).UTC().UnixNano(), 10)
	end := strconv.FormatInt(time.Unix(1571649085, 0).UTC().UnixNano(), 10)
	value, err := dh.GetSLIValue(ResponseTimeP50, start, end, []*events.SLIFilter{{Key: "dynatraceEntityName", Value: "MyService"}})

	assert.EqualValues(t, nil, err)

	assert.EqualValues(t, 0.789, value)
}

// Tests GetSLIValue without a dynatrace entity name
func TestGetSLIValueWithoutDynatraceEntityName(t *testing.T) {
	expected := "dynatraceEntityName is either empty or undefined. Please define it using customFilters."

	dh := NewDynatraceHandler("http://dynatrace", "sockshop", "dev", "carts", nil)

	start := strconv.FormatInt(time.Unix(1571649084, 0).UTC().UnixNano(), 10)
	end := strconv.FormatInt(time.Unix(1571649085, 0).UTC().UnixNano(), 10)
	value, err := dh.GetSLIValue(ResponseTimeP50, start, end, []*events.SLIFilter{})

	assert.EqualValues(t, 0.0, value)

	assert.EqualError(t, err, expected)
}

// Tests GetSLIValue with an empty result (no datapoints)
func TestGetSLIValueWithEmptyResult(t *testing.T) {

	okResponse := `{
		    "result": {
		        "unit": "MicroSeconds (µs)",
		        "dataPoints": {},
				"aggregationType": "PERCENTILE",
				"entities": {},
				"timeseriesId": "com.dynatrace.builtin:service.responsetime"
		    }
		}`

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okResponse))
	})

	httpClient, teardown := testingHTTPClient(h)
	defer teardown()

	dh := NewDynatraceHandler("http://dynatrace", "sockshop", "dev", "carts", nil)
	dh.HTTPClient = httpClient

	start := strconv.FormatInt(time.Unix(1571649084, 0).UTC().UnixNano(), 10)
	end := strconv.FormatInt(time.Unix(1571649085, 0).UTC().UnixNano(), 10)
	value, err := dh.GetSLIValue(ResponseTimeP50, start, end, []*events.SLIFilter{{Key: "dynatraceEntityName", Value: "MyService"}})

	assert.EqualValues(t, errors.New("Dynatrace API returned no DataPoints"), err)

	assert.EqualValues(t, 0.0, value)
}

// Tests GetSLIValue without the expected entity id
func TestGetSLIValueWithoutExpectedEntityId(t *testing.T) {

	okResponse := `{
		    "result": {
		        "unit": "MicroSeconds (µs)",
		        "dataPoints": {
					"ENTITY-123": [[ 123, 456 ]]
				},
				"aggregationType": "PERCENTILE",
				"entities": {
					"ENTITY-123": "NotMyService"
				},
				"timeseriesId": "com.dynatrace.builtin:service.responsetime"
		    }
		}`

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okResponse))
	})

	httpClient, teardown := testingHTTPClient(h)
	defer teardown()

	dh := NewDynatraceHandler("http://dynatrace", "sockshop", "dev", "carts", nil)
	dh.HTTPClient = httpClient

	start := strconv.FormatInt(time.Unix(1571649084, 0).UTC().UnixNano(), 10)
	end := strconv.FormatInt(time.Unix(1571649085, 0).UTC().UnixNano(), 10)
	value, err := dh.GetSLIValue(ResponseTimeP50, start, end, []*events.SLIFilter{{Key: "dynatraceEntityName", Value: "MyService"}})

	assert.EqualValues(t, errors.New("could not find entity with name 'MyService' in result"), err)

	assert.EqualValues(t, 0.0, value)
}

// Tests the behaviour of the GetSLIValue function in case of a HTTP 400 return code
func TestGetSLIValueWithErrorResponse(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// w.Write([]byte(response))
		w.WriteHeader(http.StatusBadRequest)
	})

	httpClient, teardown := testingHTTPClient(h)
	defer teardown()

	dh := NewDynatraceHandler("http://dynatrace", "sockshop", "dev", "carts", nil)
	dh.HTTPClient = httpClient

	start := strconv.FormatInt(time.Unix(1571649084, 0).UTC().UnixNano(), 10)
	end := strconv.FormatInt(time.Unix(1571649085, 0).UTC().UnixNano(), 10)
	value, err := dh.GetSLIValue(Throughput, start, end, []*events.SLIFilter{})

	assert.EqualValues(t, 0.0, value)
	assert.NotNil(t, err, nil)
}
