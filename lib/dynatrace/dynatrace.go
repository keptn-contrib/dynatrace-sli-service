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
	Dimensions []string `json:"dimensions"`
	Timestamp  int64    `json:"timestamp"`
	Value      float64  `json:"value"`
}

type resultValues struct {
	Values []resultNumbers `json:"values"`
}

/**
{
    "totalCount": 3,
    "nextPageKey": null,
    "metrics": {
        "builtin:service.response.time:merge(0):percentile(95)": {
            "values": [
                {
                    "dimensions": [],
                    "timestamp": 1573808100000,
                    "value": 8433.408860322825
                }
            ]
        }
    }
}
*/
type dynatraceResult struct {
	TotalCount  int                     `json:"totalCount"`
	NextPageKey string                  `json:"nextPageKey"`
	Metrics     map[string]resultValues `json:"metrics"`
}

// Handler interacts with a dynatrace API endpoint
type Handler struct {
	ApiURL        string
	Username      string
	Password      string
	Project       string
	Stage         string
	Service       string
	HTTPClient    *http.Client
	Headers       map[string]string
	CustomQueries map[string]string
	CustomFilters []*keptnevents.SLIFilter
}

// NewDynatraceHandler returns a new dynatrace handler that interacts with the Dynatrace REST API
func NewDynatraceHandler(apiURL string, project string, stage string, service string, headers map[string]string, customFilters []*keptnevents.SLIFilter) *Handler {
	ph := &Handler{
		ApiURL:        apiURL,
		Project:       project,
		Stage:         stage,
		Service:       service,
		HTTPClient:    &http.Client{},
		Headers:       headers,
		CustomFilters: customFilters,
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
	endUnix, _ := parseUnixTimestamp(end)
	if err != nil {
		return 0, errors.New("Error parsing end date: " + err.Error())
	}

	fmt.Printf("Getting timeseries config for metric %s\n", metric)

	timeseriesQueryString, err := ph.getTimeseriesConfig(metric)

	// split query string by first occurance of "?"
	timeseriesIdentifier := strings.Split(timeseriesQueryString, "?")[0]

	if err != nil {
		fmt.Printf("Error happened: %s\n", err.Error())
		return 0, err
	}

	// replace query params
	timeseriesQueryString = ph.replaceQueryParameters(timeseriesQueryString)

	targetUrl := ph.ApiURL + fmt.Sprintf("/api/v2/metrics/series/%s", timeseriesQueryString)

	queryParams := map[string]string{
		"resolution": "Inf", // resolution=Inf means that we only get 1 datapoint (per service)
		"from":       timestampToString(startUnix),
		"to":         timestampToString(endUnix),
	}

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
	err = json.Unmarshal([]byte(body), &result)

	if err != nil {
		return 0, err
	}

	// make sure the status code from the API is 200
	if resp.StatusCode != 200 {
		return 0, errors.New(fmt.Sprintf("Dynatrace API returned status code %d - Metric could not be received.", resp.StatusCode))
	}

	if len(result.Metrics) == 0 {
		// datapoints is empty - try again?
		return 0, errors.New("Dynatrace Metrics API returned no DataPoints")
	}

	fmt.Println("trying to fetch metric", timeseriesIdentifier)

	if _, ok := result.Metrics[timeseriesIdentifier]; !ok {
		return 0, errors.New(fmt.Sprintf("Dynatrace Metrics API result does not contain identifier %s", timeseriesIdentifier))
	}

	// finally iterate over result.Result.DataPoints and choose the one with the key lookForEntityId
	resultData := result.Metrics[timeseriesIdentifier].Values

	if len(resultData) != 1 {
		return 0, errors.New(fmt.Sprintf("Dynatrace Metrics API returned %d result values, expected 1", len(resultData)))
	}

	return scaleData(timeseriesIdentifier, resultData[0].Value), nil
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
		return "builtin:service.requestCount.total:merge(0):count?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:canary)", nil
	case ErrorRate:
		return "builtin:service.errors.total.count:merge(0):avg?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:canary)", nil
	case ResponseTimeP50:
		return "builtin:service.response.time:merge(0):percentile(50)?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:canary)", nil
	case ResponseTimeP90:
		return "builtin:service.response.time:merge(0):percentile(90)?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:canary)", nil
	case ResponseTimeP95:
		return "builtin:service.response.time:merge(0):percentile(95)?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:canary)", nil
	default:
		fmt.Sprintf("Unknown metric %s\n", metric)
		return "", errors.New(fmt.Sprintf("unsupported SLI metric %s", metric))
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
