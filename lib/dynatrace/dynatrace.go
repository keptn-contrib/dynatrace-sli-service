package dynatrace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/keptn/go-utils/pkg/events"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const Throughput = "throughput"
const ErrorRate = "error_rate"
const ResponseTimeP50 = "request_latency_p50"
const ResponseTimeP90 = "request_latency_p90"
const ResponseTimeP95 = "request_latency_p95"

type resultNumbers [][]float64

/**
{
    "result": {
        "dataPoints": {"SERVICE-A68654B786804C0C": [ [ 1571815920000, 5861 ] ], ...},
        "unit": "MicroSecond (µs)",
        "resolutionInMillisUTC": 21600000,
        "aggregationType": "PERCENTILE",
        "entities": {
            "SERVICE-A285AD1BC4628CAF": "ItemsController",
            "SERVICE-27C5A02C6567FFC8": "HealthCheckController",
            "SERVICE-C0537525C9A3EC73": "carts",
            "SERVICE-A68654B786804C0C": "VersionController"
        },
        "timeseriesId": "com.dynatrace.builtin:service.responsetime"
    }
}
*/
type dynatraceResult struct {
	Result struct {
		DataPoints            map[string]resultNumbers `json:"dataPoints"`
		Unit                  string                   `json:"unit"`
		ResolutionInMillisUTC int64                    `json:"resolutionInMillisUTC"`
		AggregationType       string                   `json:"aggregationType"`
		Entities              map[string]string        `json:"entities"`
		TimeseriesId          string                   `json:"timeseriesId"`
	} `json:"result"`
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
}

// NewDynatraceHandler returns a new dynatrace handler that interacts with the Dynatrace REST API
func NewDynatraceHandler(apiURL string, project string, stage string, service string, headers map[string]string) *Handler {
	ph := &Handler{
		ApiURL:     apiURL,
		Project:    project,
		Stage:      stage,
		Service:    service,
		HTTPClient: &http.Client{},
		Headers:    headers,
	}

	return ph
}

func parseCustomFilters(customFilters []*events.SLIFilter) (string, string) {
	// parse customFilters
	lookForEntityName := ""
	additionalTags := ""

	for _, row := range customFilters {
		if row.Key == "dynatraceEntityName" {
			lookForEntityName = row.Value
		} else if row.Key == "tags" {
			additionalTags = row.Value
		}
	}

	return lookForEntityName, additionalTags
	// foo
}

// return tags based on the timeseries id
// e.g., dynatrace builtin service metrics have tags for environment and service
func (ph *Handler) GetTagsBasedOnTimeseriesId(timeseriesId string) []string {
	if strings.HasPrefix(timeseriesId, "com.dynatrace.builtin:service.") {
		return []string{
			// "application:" + ph.Project,
			"environment:" + ph.Project + "-" + ph.Stage,
			"service:" + ph.Service,
		}
	}

	return nil
}

func (ph *Handler) GetSLIValue(metric string, start string, end string, customFilters []*events.SLIFilter) (float64, error) {
	// disable SSL verification (probably not a good idea for dynatrace)
	// http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	fmt.Printf("Querying metric %s\n", metric)

	// parse customFilters
	lookForEntityName, additionalTags := parseCustomFilters(customFilters)

	if lookForEntityName == "" {
		return 0, errors.New(fmt.Sprintf("dynatraceEntityName is either empty or undefined. Please define it using customFilters."))
	}

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

	timeseriesIdentifier, timeseriesAggregationType, percentile, err := ph.getTimeseriesConfig(metric, startUnix, endUnix)

	if err != nil {
		fmt.Printf("Error happened: %s\n", err.Error())
		return 0, err
	}

	url := ph.ApiURL + fmt.Sprintf("/api/v1/timeseries/%s/", timeseriesIdentifier)

	// set initial tag list with application, environment and service based on the timeseries identifier
	// (e.g., environment and service)
	tags := ph.GetTagsBasedOnTimeseriesId(timeseriesIdentifier)

	if tags == nil {
		// no tags found (yet) --> unsupported metric
		return 0, errors.New(fmt.Sprintf("Could not automatically generate tags for timeseries identifier %s",
			timeseriesIdentifier))
	}

	// append additional tags for dynatrace
	if additionalTags != "" {
		tags = append(tags, strings.Split(additionalTags, ",")...)
	}

	data := map[string]interface{}{
		"aggregationType": timeseriesAggregationType,
		"queryMode":       "TOTAL",
		"tags":            tags,
		"startTimestamp":  timestampToString(startUnix),
		"endTimestamp":    timestampToString(endUnix),
	}

	if timeseriesAggregationType == "percentile" {
		data["percentile"] = percentile
	}

	// build json
	reqbody, _ := json.Marshal(data)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqbody))
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

	if len(result.Result.DataPoints) == 0 {
		// datapoints is empty - try again?
		return 0, errors.New("Dynatrace API returned no DataPoints")
	}

	lookForEntityId := ""

	// iterate over result.Result.Entities
	for entityId, entityName := range result.Result.Entities {
		fmt.Printf("    Found entity %s with name %s\n", entityId, entityName)

		if entityName == lookForEntityName {
			fmt.Println("      -> ENTITY NAME MATCH")
			lookForEntityId = entityId
		}
	}

	if lookForEntityId == "" {
		return 0, errors.New(fmt.Sprintf("could not find entity with name '%s' in result", lookForEntityName))
	}

	// finally iterate over result.Result.DataPoints and choose the one with the key lookForEntityId
	resultData := result.Result.DataPoints[lookForEntityId]

	return scaleData(timeseriesIdentifier, timeseriesAggregationType, resultData[0][1]), nil
}

// scales data based on the timeseries identifier (e.g., service.responsetime needs to be scaled from microseconds
// to milliseocnds)
func scaleData(timeseriesIdentifier string, timeseriesAggregationType string, value float64) float64 {
	if timeseriesIdentifier == "com.dynatrace.builtin:service.responsetime" {
		// scale service.responsetime from microseconds to milliseconds
		return value / 1000.0
	}
	// default:
	return value
}

// based on the requested metric a dynatrace timeseries with its aggregation type is returned
func (ph *Handler) getTimeseriesConfig(metric string, start time.Time, end time.Time) (string, string, int, error) {
	if val, ok := ph.CustomQueries[metric]; ok {
		data := strings.Split(val, ",")

		if len(data) != 3 {
			return "", "", 0, errors.New(fmt.Sprintf("Metric %s custom query config has wrong length of %d", metric, len(data)))
		}

		percentage, err := strconv.Atoi(data[2])

		if err != nil {
			return "", "", 0, err
		}

		fmt.Printf("Returning custom metric %s with the following config: %s,%s,%d\n", metric, data[0], data[1], percentage)

		return data[0], data[1], percentage, nil
	}

	// default config

	switch metric {
	case Throughput:
		return "com.dynatrace.builtin:service.requests", "count", 0, nil
	case ErrorRate:
		return "com.dynatrace.builtin:service.failurerate", "avg", 0, nil
	case ResponseTimeP50:
		return "com.dynatrace.builtin:service.responsetime", "percentile", 50, nil
	case ResponseTimeP90:
		return "com.dynatrace.builtin:service.responsetime", "percentile", 90, nil
	case ResponseTimeP95:
		return "com.dynatrace.builtin:service.responsetime", "percentile", 95, nil
	default:
		fmt.Sprintf("Unknown metric %s\n", metric)
		return "", "", 0, errors.New(fmt.Sprintf("unsupported SLI metric %s", metric))
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
