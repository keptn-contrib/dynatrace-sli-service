package dynatrace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	keptnevents "github.com/keptn/go-utils/pkg/events"
)

const Throughput = "throughput"
const ErrorRate = "error_rate"
const ResponseTimeP50 = "response_time_p50"
const ResponseTimeP90 = "response_time_p90"
const ResponseTimeP95 = "response_time_p95"

type resultNumbers struct {
	Dimensions []string  `json:"dimensions"`
	Timestamps []int64   `json:"timestamps"`
	Values     []float64 `json:"values"`
}

type resultValues struct {
	MetricID string          `json:"metricId"`
	Data     []resultNumbers `json:"data"`
}

type dtMetricsAPIError struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

/**
{
    "totalCount": 8,
    "nextPageKey": null,
    "result": [
        {
            "metricId": "builtin:service.response.time:percentile(50):merge(0)",
            "data": [
                {
                    "dimensions": [],
                    "timestamps": [
                        1579097520000
                    ],
                    "values": [
                        65005.48481639812
                    ]
                }
            ]
        }
    ]
}
*/
type dynatraceResult struct {
	TotalCount  int            `json:"totalCount"`
	NextPageKey string         `json:"nextPageKey"`
	Result      []resultValues `json:"result"`
}

// Handler interacts with a dynatrace API endpoint
type Handler struct {
	ApiURL        string
	Username      string
	Password      string
	Project       string
	Stage         string
	Service       string
	Deployment    string
	HTTPClient    *http.Client
	Headers       map[string]string
	CustomQueries map[string]string
	CustomFilters []*keptnevents.SLIFilter
}

// NewDynatraceHandler returns a new dynatrace handler that interacts with the Dynatrace REST API
func NewDynatraceHandler(apiURL string, project string, stage string, service string, headers map[string]string, customFilters []*keptnevents.SLIFilter, deployment string) *Handler {
	ph := &Handler{
		ApiURL:        apiURL,
		Project:       project,
		Stage:         stage,
		Service:       service,
		HTTPClient:    &http.Client{},
		Headers:       headers,
		CustomFilters: customFilters,
		Deployment:    deployment,
	}

	return ph
}

func (ph *Handler) GetSLIValue(metric string, start string, end string, customFilters []*keptnevents.SLIFilter) (float64, error) {
	// disable SSL verification (probably not a good idea for dynatrace)
	// http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	fmt.Printf("Querying metric %s\n", metric)

	// parse start and end (which are datetime strings) and convert them into unix timestamps
	startUnix, err := parseUnixTimestamp(start)
	if err != nil {
		return 0, errors.New("Error parsing start date: " + err.Error())
	}
	endUnix, err := parseUnixTimestamp(end)
	if err != nil {
		return 0, errors.New("Error parsing end date: " + err.Error())
	}

	// ensure end time is not in the future
	if time.Now().Sub(endUnix).Seconds() < 0 {
		fmt.Printf("ERROR: Supplied end-time %v is in the future\n", endUnix)
		return 0, errors.New("end time must not be in the future")
	}

	// ensure start time is before end time
	if endUnix.Sub(startUnix).Seconds() < 0 {
		fmt.Printf("ERROR: Start time needs to be before end time\n")
		return 0, errors.New("start time needs to be before end time")
	}

	// make sure the end timestamp is at least 120 seconds in the past such that dynatrace metrics API has processed data
	for time.Now().Sub(endUnix).Seconds() < 120 {
		// ToDo: this should be done in main.go
		fmt.Printf("Sleeping for %d seconds... (waiting for Dynatrace Metrics API)\n", int(120-time.Now().Sub(endUnix).Seconds()))
		time.Sleep(10 * time.Second)
	}

	fmt.Printf("Getting timeseries config for metric %s\n", metric)

	timeseriesQueryString, err := ph.getTimeseriesConfig(metric)

	if err != nil {
		fmt.Printf("Error when fetching timeseries config: %s\n", err.Error())
		return 0, err
	}

	// replace query params
	timeseriesQueryString = ph.replaceQueryParameters(timeseriesQueryString)

	// split query string by first occurance of "?"
	timeseriesIdentifier := strings.Split(timeseriesQueryString, "?")[0]

	timeseriesIdentifierEncoded := url.QueryEscape(timeseriesIdentifier)

	timeseriesQueryString = strings.Replace(timeseriesQueryString, timeseriesIdentifier, timeseriesIdentifierEncoded, 1)

	fmt.Printf("Old=%s, new=%s\n", timeseriesIdentifier, timeseriesIdentifierEncoded)

	targetUrl := ph.ApiURL + fmt.Sprintf("/api/v2/metrics/query/")

	queryParams := map[string]string{
		"metricSelector": timeseriesQueryString,
		"resolution":     "Inf", // resolution=Inf means that we only get 1 datapoint (per service)
		"from":           timestampToString(startUnix),
		"to":             timestampToString(endUnix),
	}
	fmt.Println(queryParams)

	// append queryParams to URL
	u, _ := url.Parse(targetUrl)
	q, _ := url.ParseQuery(u.RawQuery)

	for param, value := range queryParams {
		q.Add(param, value)
	}

	u.RawQuery = q.Encode()

	fmt.Println("TargetURL=", u.String())

	req, err := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("Content-Type", "application/json")

	// set additional headers
	for headerName, headerValue := range ph.Headers {
		req.Header.Set(headerName, headerValue)
	}

	// perform the request
	resp, err := ph.HTTPClient.Do(req)

	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	fmt.Println("Request finished, parsing body...")

	body, _ := ioutil.ReadAll(resp.Body)

	var result dynatraceResult

	// parse json
	err = json.Unmarshal(body, &result)

	if err != nil {
		return 0, err
	}

	// make sure the status code from the API is 200
	if resp.StatusCode != 200 {
		dtMetricsErr := &dtMetricsAPIError{}
		err := json.Unmarshal(body, dtMetricsErr)
		if err == nil {
			return 0, fmt.Errorf("Dynatrace API returned status code %d: %s", dtMetricsErr.Error.Code, dtMetricsErr.Error.Message)
		}
		return 0, fmt.Errorf("Dynatrace API returned status code %d - Metric could not be received.", resp.StatusCode)
	}

	if len(result.Result) == 0 {
		// datapoints is empty - try again?
		return 0, errors.New("Dynatrace Metrics API returned no DataPoints")
	}

	fmt.Println("trying to fetch metric", timeseriesIdentifier)

	var (
		metricIdExists    = false
		actualMetricValue = 0.0
	)
	for _, i := range result.Result {

		if i.MetricID == timeseriesIdentifier {
			metricIdExists = true

			if len(i.Data) != 1 {
				return 0, fmt.Errorf("Dynatrace Metrics API returned %d result values, expected 1", len(i.Data))
			}

			actualMetricValue = i.Data[0].Values[0]
		}
	}

	if !metricIdExists {
		return 0, fmt.Errorf("Dynatrace Metrics API result does not contain identifier %s", timeseriesIdentifier)
	}

	return scaleData(timeseriesIdentifier, actualMetricValue), nil
}

// scales data based on the timeseries identifier (e.g., service.responsetime needs to be scaled from microseconds
// to milliseocnds)
func scaleData(timeseriesIdentifier string, value float64) float64 {
	if strings.Contains(timeseriesIdentifier, "builtin:service.response.time") {
		// scale service.responsetime from microseconds to milliseconds
		return value / 1000.0
	}
	// default:
	return value
}

func (ph *Handler) replaceQueryParameters(query string) string {
	// apply customfilters
	for _, filter := range ph.CustomFilters {
		filter.Value = strings.Replace(filter.Value, "'", "", -1)
		filter.Value = strings.Replace(filter.Value, "\"", "", -1)

		// replace the key in both variants, "normal" and uppercased
		query = strings.Replace(query, "$"+filter.Key, filter.Value, -1)
		query = strings.Replace(query, "$"+strings.ToUpper(filter.Key), filter.Value, -1)
	}

	// apply default values
	query = strings.Replace(query, "$PROJECT", ph.Project, -1)
	query = strings.Replace(query, "$STAGE", ph.Stage, -1)
	query = strings.Replace(query, "$SERVICE", ph.Service, -1)
	query = strings.Replace(query, "$DEPLOYMENT", ph.Deployment, -1)

	return query
}

// based on the requested metric a dynatrace timeseries with its aggregation type is returned
func (ph *Handler) getTimeseriesConfig(metric string) (string, error) {
	if val, ok := ph.CustomQueries[metric]; ok {
		return val, nil
	}

	// default config
	switch metric {
	case Throughput:
		return "builtin:service.requestCount.total.merge(0).count&scope=tag(keptn_project.$PROJECT),tag(keptn_stage.$STAGE),tag(keptn_service.$SERVICE),tag(keptn_deployment.$DEPLOYMENT)", nil
		//"builtin:service.requestCount.total.merge(0).count&scope=tag(keptn_project.$PROJECT),tag(keptn_stage.$STAGE),tag(keptn_service.$SERVICE),tag(keptn_deployment.DEPLOYMENT)"
	case ErrorRate:
		return "builtin:service.errors.total.count.merge(0).avg?scope=tag(keptn_project.$PROJECT),tag(keptn_stage.$STAGE),tag(keptn_service.$SERVICE),tag(keptn_deployment.$DEPLOYMENT)", nil
	case ResponseTimeP50:
		return "builtin:service.response.time.merge(0).percentile(50)?scope=tag(keptn_project.$PROJECT),tag(keptn_stage.$STAGE),tag(keptn_service.$SERVICE),tag(keptn_deployment.$DEPLOYMENT)", nil
	case ResponseTimeP90:
		return "builtin:service.response.time.merge(0).percentile(90)?scope=tag(keptn_project.$PROJECT),tag(keptn_stage.$STAGE),tag(keptn_service.$SERVICE),tag(keptn_deployment.$DEPLOYMENT)", nil
	case ResponseTimeP95:
		return "builtin:service.response.time.merge(0).percentile(95)?scope=tag(keptn_project.$PROJECT),tag(keptn_stage.$STAGE),tag(keptn_service.$SERVICE),tag(keptn_deployment.$DEPLOYMENT)", nil
	default:
		fmt.Sprintf("Unknown metric %s\n", metric)
		return "", fmt.Errorf("unsupported SLI metric %s", metric)
	}
}

func parseUnixTimestamp(timestamp string) (time.Time, error) {
	parsedTime, err := time.Parse(time.RFC3339, timestamp)
	if err == nil {
		return parsedTime, nil
	}

	timestampInt, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return time.Now(), err
	}
	unix := time.Unix(timestampInt, 0)
	return unix, nil
}

func timestampToString(time time.Time) string {
	return strconv.FormatInt(time.Unix()*1000, 10)
}
