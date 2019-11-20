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
		"totalCount": 3,
		"nextPageKey": null,
		"metrics": {
			"builtin:service.response.time:merge(0):percentile(50)": {
				"values": [
					{
						"dimensions": [],
						"timestamp": 1573808100000,
						"value": 8433.40
					}
				]
			}
		}
	}`

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okResponse))
	})

	httpClient, teardown := testingHTTPClient(h)
	defer teardown()

	dh := NewDynatraceHandler("http://dynatrace", "sockshop", "dev", "carts", nil, nil)
	dh.HTTPClient = httpClient

	start := strconv.FormatInt(time.Unix(1571649084, 0).UTC().Unix()*1000, 10)
	end := strconv.FormatInt(time.Unix(1571649085, 0).UTC().Unix()*1000, 10)
	value, err := dh.GetSLIValue(ResponseTimeP50, start, end, nil)

	assert.EqualValues(t, nil, err)

	assert.InDelta(t, 8.43340, value, 0.001)
}

// Tests GetSLIValue with an empty result (no datapoints)
func TestGetSLIValueWithEmptyResult(t *testing.T) {

	okResponse := `{
    "totalCount": 4,
    "nextPageKey": null,
    "metrics": {
        "builtin:service.response.time:merge(0):percentile(50)": {
            "values": [
            ]
        }
    }
}`

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okResponse))
	})

	httpClient, teardown := testingHTTPClient(h)
	defer teardown()

	dh := NewDynatraceHandler("http://dynatrace", "sockshop", "dev", "carts", nil, nil)
	dh.HTTPClient = httpClient

	start := strconv.FormatInt(time.Unix(1571649084, 0).UTC().UnixNano(), 10)
	end := strconv.FormatInt(time.Unix(1571649085, 0).UTC().UnixNano(), 10)
	value, err := dh.GetSLIValue(ResponseTimeP50, start, end, nil)

	assert.EqualValues(t, errors.New("Dynatrace Metrics API returned 0 result values, expected 1"), err)

	assert.EqualValues(t, 0.0, value)
}

// Tests GetSLIValue without the expected metric in it
func TestGetSLIValueWithoutExpectedMetric(t *testing.T) {

	okResponse := `{
		"totalCount": 4,
		"nextPageKey": null,
		"metrics": {
			"something_else": {
				"values": [
					{
						"dimensions": [],
						"timestamp": 1574092860000,
						"value": 1364.0454545454545
					}
				]
			}
		}
	}`

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okResponse))
	})

	httpClient, teardown := testingHTTPClient(h)
	defer teardown()

	dh := NewDynatraceHandler("http://dynatrace", "sockshop", "dev", "carts", nil, nil)
	dh.HTTPClient = httpClient

	start := strconv.FormatInt(time.Unix(1571649084, 0).UTC().UnixNano(), 10)
	end := strconv.FormatInt(time.Unix(1571649085, 0).UTC().UnixNano(), 10)
	value, err := dh.GetSLIValue(ResponseTimeP50, start, end, nil)

	assert.EqualValues(t, errors.New("Dynatrace Metrics API result does not contain identifier builtin:service.response.time:merge(0):percentile(50)"), err)

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

	dh := NewDynatraceHandler("http://dynatrace", "sockshop", "dev", "carts", nil, nil)
	dh.HTTPClient = httpClient

	start := strconv.FormatInt(time.Unix(1571649084, 0).UTC().UnixNano(), 10)
	end := strconv.FormatInt(time.Unix(1571649085, 0).UTC().UnixNano(), 10)
	value, err := dh.GetSLIValue(Throughput, start, end, []*events.SLIFilter{})

	assert.EqualValues(t, 0.0, value)
	assert.NotNil(t, err, nil)
}
