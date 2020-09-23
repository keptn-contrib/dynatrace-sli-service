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

	"github.com/keptn-contrib/dynatrace-sli-service/pkg/common"

	// keptnevents "github.com/keptn/go-utils/pkg/events"
	// keptnutils "github.com/keptn/go-utils/pkg/utils"

	keptn "github.com/keptn/go-utils/pkg/lib"
)

const Throughput = "throughput"
const ErrorRate = "error_rate"
const ResponseTimeP50 = "response_time_p50"
const ResponseTimeP90 = "response_time_p90"
const ResponseTimeP95 = "response_time_p95"

// store url to the metrics api format migration document
const MetricsAPIOldFormatNewFormatDoc = "https://github.com/keptn-contrib/dynatrace-sli-service/blob/master/docs/CustomQueryFormatMigration.md"

type resultNumbers struct {
	Dimensions []string  `json:"dimensions"`
	Timestamps []int64   `json:"timestamps"`
	Values     []float64 `json:"values"`
}

type resultValues struct {
	MetricID string          `json:"metricId"`
	Data     []resultNumbers `json:"data"`
}

// DTUSQLResult struct
type DTUSQLResult struct {
	ExtrapolationLevel int             `json:"extrapolationLevel"`
	ColumnNames        []string        `json:"columnNames"`
	Values             [][]interface{} `json:"values"`
}

// SLI struct for SLI.yaml
type SLI struct {
	SpecVersion string            `yaml:"spec_version"`
	Indicators  map[string]string `yaml:"indicators"`
}

// DynatraceDashboards is struct for /dashboards endpoint
type DynatraceDashboards struct {
	Dashboards []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Owner string `json:"owner"`
	} `json:"dashboards"`
}

// DynatraceDashboard is struct for /dashboards/<dashboardID> endpoint
type DynatraceDashboard struct {
	Metadata struct {
		ConfigurationVersions []int  `json:"configurationVersions"`
		ClusterVersion        string `json:"clusterVersion"`
	} `json:"metadata"`
	ID                string `json:"id"`
	DashboardMetadata struct {
		Name           string `json:"name"`
		Shared         bool   `json:"shared"`
		Owner          string `json:"owner"`
		SharingDetails struct {
			LinkShared bool `json:"linkShared"`
			Published  bool `json:"published"`
		} `json:"sharingDetails"`
		DashboardFilter *struct {
			Timeframe      string `json:"timeframe"`
			ManagementZone *struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"managementZone,omitempty"`
		} `json:"dashboardFilter,omitempty"`
		Tags []string `json:"tags"`
	} `json:"dashboardMetadata"`
	Tiles []struct {
		Name       string `json:"name"`
		TileType   string `json:"tileType"`
		Configured bool   `json:"configured"`
		Query      string `json:"query"`
		Type       string `json:"type"`
		CustomName string `json:"customName`
		Markdown   string `json:"markdown`
		Bounds     struct {
			Top    int `json:"top"`
			Left   int `json:"left"`
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"bounds"`
		TileFilter struct {
			Timeframe      interface{} `json:"timeframe"`
			ManagementZone interface{} `json:"managementZone"`
		} `json:"tileFilter"`
		FilterConfig struct {
			Type        string `json:"type"`
			CustomName  string `json:"customName"`
			DefaultName string `json:"defaultName"`
			ChartConfig struct {
				LegendShown bool   `json:"legendShown"`
				Type        string `json:"type"`
				Series      []struct {
					Metric      string      `json:"metric"`
					Aggregation string      `json:"aggregation"`
					Percentile  interface{} `json:"percentile"`
					Type        string      `json:"type"`
					EntityType  string      `json:"entityType"`
					Dimensions  []struct {
						ID              string   `json:"id"`
						Name            string   `json:"name"`
						Values          []string `json:"values"`
						EntityDimension bool     `json:"entitiyDimension"`
					} `json:"dimensions"`
					SortAscending   bool   `json:"sortAscending"`
					SortColumn      bool   `json:"sortColumn"`
					AggregationRate string `json:"aggregationRate"`
				} `json:"series"`
				ResultMetadata struct {
				} `json:"resultMetadata"`
			} `json:"chartConfig"`
			FiltersPerEntityType struct {
			} `json:"filtersPerEntityType"`
		} `json:"filterConfig"`
	} `json:"tiles"`
}

// MetricDefinition defines the output of /metrics/<metricID>
type MetricDefinition struct {
	MetricID           string   `json:"metricId"`
	DisplayName        string   `json:"displayName"`
	Description        string   `json:"description"`
	Unit               string   `json:"unit"`
	AggregationTypes   []string `json:"aggregationTypes"`
	Transformations    []string `json:"transformations"`
	DefaultAggregation struct {
		Type string `json:"type"`
	} `json:"defaultAggregation"`
	DimensionDefinitions []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"dimensionDefinitions"`
	EntityType []string `json:"entityType"`
}

type DtMetricsAPIError struct {
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

// DynatraceResult is struct for /metrics/query
type DynatraceResult struct {
	TotalCount  int            `json:"totalCount"`
	NextPageKey string         `json:"nextPageKey"`
	Result      []resultValues `json:"result"`
}

// Handler interacts with a dynatrace API endpoint
type Handler struct {
	ApiURL        string
	Username      string
	Password      string
	KeptnEvent    *common.BaseKeptnEvent
	HTTPClient    *http.Client
	Headers       map[string]string
	CustomQueries map[string]string
	CustomFilters []*keptn.SLIFilter
	Logger        keptn.LoggerInterface
}

// NewDynatraceHandler returns a new dynatrace handler that interacts with the Dynatrace REST API
func NewDynatraceHandler(apiURL string, keptnEvent *common.BaseKeptnEvent, headers map[string]string, customFilters []*keptn.SLIFilter, keptnContext string, eventID string) *Handler {
	ph := &Handler{
		ApiURL:        apiURL,
		KeptnEvent:    keptnEvent,
		HTTPClient:    &http.Client{},
		Headers:       headers,
		CustomFilters: customFilters,
		Logger:        keptn.NewLogger(keptnContext, eventID, "dynatrace-sli-service"),
	}

	return ph
}

/**
 * Queries all Dynatrace Dashboards and returns the dashboard ID that matches KQG;project=%project%;service=%service%;stage=%stage;xxx
 */
func (ph *Handler) findDynatraceDashboard(project string, stage string, service string) (string, error) {
	// Lets query the list of all Dashboards and find the one that matches project, stage, service based on the title (in the future - we can do it via tags)
	// create dashboard query URL and set additional headers
	ph.Logger.Debug(fmt.Sprintf("Query all dashboards\n"))
	dashboardAPIUrl := ph.ApiURL + fmt.Sprintf("/api/config/v1/dashboards")
	req, err := http.NewRequest("GET", dashboardAPIUrl, nil)
	for headerName, headerValue := range ph.Headers {
		req.Header.Set(headerName, headerValue)
	}

	// perform the request
	resp, err := ph.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Dynatrace API returned status code %d", resp.StatusCode)
	}

	ph.Logger.Debug("Request finished, parsing dashboard list response body...")
	body, _ := ioutil.ReadAll(resp.Body)
	dashboardsJSON := &DynatraceDashboards{}

	// parse json
	err = json.Unmarshal(body, &dashboardsJSON)

	if err != nil {
		return "", err
	}

	// now - lets iterate through the list and find one that matches our project, stage, service ...
	findValues := []string{strings.ToLower(fmt.Sprintf("project=%s", project)), strings.ToLower(fmt.Sprintf("service=%s", service)), strings.ToLower(fmt.Sprintf("stage=%s", stage))}
	for _, dashboard := range dashboardsJSON.Dashboards {

		// lets see if the dashboard matches our name
		if strings.HasPrefix(strings.ToLower(dashboard.Name), "kqg;") {
			ph.Logger.Debug("Analyzing if Dashboard matches: " + dashboard.Name)

			nameSplits := strings.Split(dashboard.Name, ";")

			// now lets see if we can find all our name/value pairs for project, service & stage
			dashboardMatch := true
			for _, findValue := range findValues {
				foundValue := false
				for _, nameSplitValue := range nameSplits {
					if strings.Compare(findValue, strings.ToLower(nameSplitValue)) == 0 {
						foundValue = true
					}
				}
				if foundValue == false {
					dashboardMatch = false
					continue
				}
			}

			if dashboardMatch {
				ph.Logger.Debug("Found Dashboard Match: " + dashboard.ID)
				return dashboard.ID, nil
			}
		}
	}

	return "", nil
}

/**
 * loadDynatraceDashboard: will either query the passed dashboard id - or - if none is passed - will try to find a dashboard that matches project, stage, service (based on name or tags)
 * the parsed JSON object is returned
 */
func (ph *Handler) loadDynatraceDashboard(project string, stage string, service string, dashboard string) (*DynatraceDashboard, error) {
	if dashboard == "" {
		dashboard, _ = ph.findDynatraceDashboard(project, stage, service)
	}

	if dashboard == "" {
		return nil, nil
	}

	// create dashboard query URL and set additional headers
	ph.Logger.Debug(fmt.Sprintf("Query dashboard with ID: %s\n", dashboard))
	dashboardAPIUrl := ph.ApiURL + fmt.Sprintf("/api/config/v1/dashboards/%s", dashboard)
	req, err := http.NewRequest("GET", dashboardAPIUrl, nil)
	for headerName, headerValue := range ph.Headers {
		req.Header.Set(headerName, headerValue)
	}

	// perform the request
	resp, err := ph.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Dynatrace API returned status code %d", resp.StatusCode)
	}

	ph.Logger.Debug("Request finished, parsing dashboard response body...")
	body, _ := ioutil.ReadAll(resp.Body)

	dashboardJSON := &DynatraceDashboard{}

	// parse json
	err = json.Unmarshal(body, &dashboardJSON)

	if err != nil {
		return nil, fmt.Errorf("could not decode response payload: %v", err)
	}

	return dashboardJSON, nil
}

// ExecuteMetricAPIDescribe calls the /metrics/<metricID> API call to retrieve Metric Definition Details
func (ph *Handler) ExecuteMetricAPIDescribe(metricID string) (*MetricDefinition, error) {
	targetURL := ph.ApiURL + fmt.Sprintf("/api/v2/metrics/%s", metricID)
	req, err := http.NewRequest("GET", targetURL, nil)

	// set additional headers
	for headerName, headerValue := range ph.Headers {
		req.Header.Set(headerName, headerValue)
	}

	// perform the request
	resp, err := ph.HTTPClient.Do(req)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	ph.Logger.Debug("Request finished, parsing body...")

	body, _ := ioutil.ReadAll(resp.Body)

	ph.Logger.Debug(string(body) + "\n")

	// parse response json
	var result MetricDefinition
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	// make sure the status code from the API is 200
	if resp.StatusCode != 200 {
		dtMetricsErr := &DtMetricsAPIError{}
		err := json.Unmarshal(body, dtMetricsErr)
		if err == nil {
			return nil, fmt.Errorf("Dynatrace API returned status code %d: %s", dtMetricsErr.Error.Code, dtMetricsErr.Error.Message)
		}
		return nil, fmt.Errorf("Dynatrace API returned status code %d - Metric could not be received.", resp.StatusCode)
	}

	return &result, nil
}

// ExecuteMetricsAPIQuery executes the passed Metrics API Call, validates that the call returns data and returns the data set
func (ph *Handler) ExecuteMetricsAPIQuery(metricsQuery string) (*DynatraceResult, error) {
	// now we execute the query against the Dynatrace API
	req, err := http.NewRequest("GET", metricsQuery, nil)
	req.Header.Set("Content-Type", "application/json")

	// set additional headers
	for headerName, headerValue := range ph.Headers {
		req.Header.Set(headerName, headerValue)
	}

	// perform the request
	resp, err := ph.HTTPClient.Do(req)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	ph.Logger.Debug("Request finished, parsing body...")

	body, _ := ioutil.ReadAll(resp.Body)

	ph.Logger.Debug(string(body) + "\n")

	// parse response json
	var result DynatraceResult
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	// make sure the status code from the API is 200
	if resp.StatusCode != 200 {
		dtMetricsErr := &DtMetricsAPIError{}
		err := json.Unmarshal(body, dtMetricsErr)
		if err == nil {
			return nil, fmt.Errorf("Dynatrace API returned status code %d: %s", dtMetricsErr.Error.Code, dtMetricsErr.Error.Message)
		}
		return nil, fmt.Errorf("Dynatrace API returned status code %d - Metric could not be received.", resp.StatusCode)
	}

	if len(result.Result) == 0 {
		// datapoints is empty - try again?
		return nil, errors.New("Dynatrace Metrics API returned no DataPoints")
	}

	return &result, nil
}

// ExecuteUSQLQuery executes the passed Metrics API Call, validates that the call returns data and returns the data set
func (ph *Handler) ExecuteUSQLQuery(usql string) (*DTUSQLResult, error) {
	// now we execute the query against the Dynatrace API
	req, err := http.NewRequest("GET", usql, nil)
	req.Header.Set("Content-Type", "application/json")

	// set additional headers
	for headerName, headerValue := range ph.Headers {
		req.Header.Set(headerName, headerValue)
	}

	// perform the request
	resp, err := ph.HTTPClient.Do(req)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	ph.Logger.Debug(("Request finished, parsing body..."))

	body, _ := ioutil.ReadAll(resp.Body)

	ph.Logger.Debug(string(body) + "\n")

	// parse response json
	var result DTUSQLResult
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	// make sure the status code from the API is 200
	if resp.StatusCode != 200 {
		dtMetricsErr := &DtMetricsAPIError{}
		err := json.Unmarshal(body, dtMetricsErr)
		if err == nil {
			return nil, fmt.Errorf("Dynatrace API returned status code %d: %s", dtMetricsErr.Error.Code, dtMetricsErr.Error.Message)
		}
		return nil, fmt.Errorf("Dynatrace API returned status code %d - Metric could not be received.", resp.StatusCode)
	}

	// if no data comes back
	if len(result.Values) == 0 {
		// datapoints is empty - try again?
		return nil, errors.New("Dynatrace USQL Query didnt return any DataPoints")
	}

	return &result, nil
}

// BuildDynatraceUSQLQuery builds a USQL query based on the incoming values
func (ph *Handler) BuildDynatraceUSQLQuery(query string, startUnix time.Time, endUnix time.Time) string {
	ph.Logger.Debug(fmt.Sprintf("Finalize USQL query for %s\n", query))

	// replace query params (e.g., $PROJECT, $STAGE, $SERVICE ...)
	usql := ph.replaceQueryParameters(query)

	// default query params that are required: resolution, from and to
	queryParams := map[string]string{
		"query":             usql,
		"explain":           "false",
		"addDeepLinkFields": "false",
		"startTimestamp":    common.TimestampToString(startUnix),
		"endTimestamp":      common.TimestampToString(endUnix),
	}

	targetURL := fmt.Sprintf("%s/api/v1/userSessionQueryLanguage/table", ph.ApiURL)

	// append queryParams to targetURL
	u, _ := url.Parse(targetURL)
	q, _ := url.ParseQuery(u.RawQuery)

	for param, value := range queryParams {
		q.Add(param, value)
	}

	u.RawQuery = q.Encode()
	ph.Logger.Debug(fmt.Sprintf("Final USQL Query=%s", u.String()))

	return u.String()
}

// BuildDynatraceMetricsQuery builds the complete query string based on start, end and filters
// metricQuery should contain metricSelector and entitySelector
// Returns:
//  #1: Finalized Dynatrace API Query
//  #2: MetricID that this query will return, e.g: builtin:host.cpu
//  #3: error
func (ph *Handler) BuildDynatraceMetricsQuery(metricquery string, startUnix time.Time, endUnix time.Time, customFilters []*keptn.SLIFilter) (string, string) {
	ph.Logger.Debug(fmt.Sprintf("Finalize query for %s\n", metricquery))

	// replace query params (e.g., $PROJECT, $STAGE, $SERVICE ...)
	metricquery = ph.replaceQueryParameters(metricquery)

	if strings.HasPrefix(metricquery, "?metricSelector=") {
		ph.Logger.Debug(fmt.Sprintf("COMPATIBILITY WARNING: Provided query string %s is not compatible. Auto-removing the ? in front (see %s for details).\n", metricquery, MetricsAPIOldFormatNewFormatDoc))
		metricquery = strings.Replace(metricquery, "?metricSelector=", "metricSelector=", 1)
	}

	// split query string by first occurrence of "?"
	querySplit := strings.Split(metricquery, "?")
	metricSelector := ""
	metricQueryParams := ""

	// support the old format with "metricSelector:someFilters()?scope=..." as well as the new format with
	// "?metricSelector=metricSelector&entitySelector=...&scope=..."
	if len(querySplit) == 1 {
		// new format without "?" -> everything within the query string are query parameters
		metricQueryParams = querySplit[0]
	} else {
		ph.Logger.Debug(fmt.Sprintf("COMPATIBILITY WARNING: Your query %s still uses the old format (see %s for details).\n", metricQueryParams, MetricsAPIOldFormatNewFormatDoc))
		// old format with "?" - everything left of the ? is the identifier, everything right are query params
		metricSelector = querySplit[0]

		// build the new query
		metricQueryParams = fmt.Sprintf("metricSelector=%s&%s", querySplit[0], querySplit[1])
	}

	targetURL := ph.ApiURL + fmt.Sprintf("/api/v2/metrics/query/?%s", metricQueryParams)

	// default query params that are required: resolution, from and to
	queryParams := map[string]string{
		"resolution": "Inf", // resolution=Inf means that we only get 1 datapoint (per service)
		"from":       common.TimestampToString(startUnix),
		"to":         common.TimestampToString(endUnix),
	}
	// append queryParams to targetURL
	u, _ := url.Parse(targetURL)
	q, _ := url.ParseQuery(u.RawQuery)

	for param, value := range queryParams {
		q.Add(param, value)
	}

	// check if q contains "scope"
	scopeData := q.Get("scope")

	// compatibility with old scope=... custom queries
	if scopeData != "" {
		ph.Logger.Debug(fmt.Sprintf("COMPATIBILITY WARNING: You are still using scope=... - querying the new metrics API requires use of entitySelector=... instead (see %s for details).", MetricsAPIOldFormatNewFormatDoc))
		// scope is no longer supported in the new API, it needs to be called "entitySelector" and contain type(SERVICE)
		if !strings.Contains(scopeData, "type(SERVICE)") {
			ph.Logger.Debug(fmt.Sprintf("COMPATIBILITY WARNING: Automatically adding type(SERVICE) to entitySelector=... for compatibility with the new Metrics API (see %s for details).", MetricsAPIOldFormatNewFormatDoc))
			scopeData = fmt.Sprintf("%s,type(SERVICE)", scopeData)
		}
		// add scope as entitySelector
		q.Add("entitySelector", scopeData)
	}

	// check metricSelector
	if metricSelector == "" {
		metricSelector = q.Get("metricSelector")
	}

	u.RawQuery = q.Encode()
	ph.Logger.Debug(fmt.Sprintf("Final Query=%s", u.String()))

	return u.String(), metricSelector
}

// ParsePassAndWarningFromString takes a value such as "Some description;sli=teststep_rt;pass=<500ms,<+10%;warning=<1000ms,<+20%;weight=1;key=true"
// can also take a value like "KQG;project=myproject;pass=90%;warning=75%;"
// This will return
// #1: teststep_rt
// #2: []SLOCriteria { Criteria{"<500ms","<+10%"}}
// #3: []SLOCriteria { ["<1000ms","<+20%" }}
// #4: 1
// #5: true
func ParsePassAndWarningFromString(customName string, defaultPass []string, defaultWarning []string) (string, []*keptn.SLOCriteria, []*keptn.SLOCriteria, int, bool) {
	nameValueSplits := strings.Split(customName, ";")

	// lets initialize it
	sliName := ""
	weight := 1
	keySli := false
	passCriteria := []*keptn.SLOCriteria{}
	warnCriteria := []*keptn.SLOCriteria{}

	for i := 0; i < len(nameValueSplits); i++ {
		nameValueSplit := strings.Split(nameValueSplits[i], "=")
		switch nameValueSplit[0] {
		case "sli":
			sliName = nameValueSplit[1]
		case "pass":
			passCriteria = append(passCriteria, &keptn.SLOCriteria{
				Criteria: strings.Split(nameValueSplit[1], ","),
			})
		case "warning":
			warnCriteria = append(warnCriteria, &keptn.SLOCriteria{
				Criteria: strings.Split(nameValueSplit[1], ","),
			})
		case "key":
			keySli, _ = strconv.ParseBool(nameValueSplit[1])
		case "weight":
			weight, _ = strconv.Atoi(nameValueSplit[1])
		}
	}

	// use the defaults if nothing was specified
	if (len(passCriteria) == 0) && (len(defaultPass) > 0) {
		passCriteria = append(passCriteria, &keptn.SLOCriteria{
			Criteria: defaultPass,
		})
	}

	if (len(warnCriteria) == 0) && (len(defaultWarning) > 0) {
		warnCriteria = append(warnCriteria, &keptn.SLOCriteria{
			Criteria: defaultWarning,
		})
	}

	// if we have no criteria for warn or pass we just return nil
	if len(passCriteria) == 0 {
		passCriteria = nil
	}
	if len(warnCriteria) == 0 {
		warnCriteria = nil
	}

	return sliName, passCriteria, warnCriteria, weight, keySli
}

// ParseMarkdownConfiguration parses a text that can be used in a Markdown tile to specify global SLO properties
func ParseMarkdownConfiguration(markdown string, slo *keptn.ServiceLevelObjectives) {
	markdownSplits := strings.Split(markdown, ";")

	for _, markdownSplitValue := range markdownSplits {
		configValueSplits := strings.Split(markdownSplitValue, "=")
		if len(configValueSplits) != 2 {
			continue
		}

		// lets get configname and value
		configName := strings.ToLower(configValueSplits[0])
		configValue := configValueSplits[1]

		switch configName {
		case "kqg.total.pass":
			slo.TotalScore.Pass = configValue
		case "kqg.total.warning":
			slo.TotalScore.Warning = configValue
		case "kqg.compare.withscore":
			slo.Comparison.IncludeResultWithScore = configValue
			if (configValue == "pass") || (configValue == "pass_or_warn") || (configValue == "all") {
				slo.Comparison.IncludeResultWithScore = configValue
			} else {
				slo.Comparison.IncludeResultWithScore = "pass"
			}
		case "kqg.compare.results":
			noresults, err := strconv.Atoi(configValue)
			if err != nil {
				slo.Comparison.NumberOfComparisonResults = 1
			} else {
				slo.Comparison.NumberOfComparisonResults = noresults
			}
			if slo.Comparison.NumberOfComparisonResults > 1 {
				slo.Comparison.CompareWith = "several_results"
			} else {
				slo.Comparison.CompareWith = "single_result"
			}
		case "kqg.compare.function":
			if (configValue == "avg") || (configValue == "p50") || (configValue == "p90") || (configValue == "p95") {
				slo.Comparison.AggregateFunction = configValue
			} else {
				slo.Comparison.AggregateFunction = "avg"
			}
		}
	}
}

// cleanIndicatorName makes sure we have a valid indicator name by getting rid of special characters
func cleanIndicatorName(indicatorName string) string {
	// TODO: check more than just blanks
	indicatorName = strings.ReplaceAll(indicatorName, " ", "_")
	indicatorName = strings.ReplaceAll(indicatorName, "/", "_")
	indicatorName = strings.ReplaceAll(indicatorName, "%", "_")

	return indicatorName
}

/**
 * When passing a query to dynatrace using filter expressions - the dimension names in a filter will be escaped with specifal characters, e.g: filter(dt.entity.browser,IE) becomes filter(dt~entity~browser,ie)
 * This function here tries to come up with a better matching algorithm
 * WHILE NOT PERFECT - HERE IS THE FIRST IMPLEMENTATION
 */
func (ph *Handler) isMatchingMetricID(singleResultMetricID string, queryMetricID string) bool {
	if strings.Compare(singleResultMetricID, queryMetricID) == 0 {
		return true
	}

	// lets do some basic fuzzy matching
	if strings.Contains(singleResultMetricID, "~") {
		ph.Logger.Debug(fmt.Sprintf("Need Fuzzy Matching between %s and %s\n", singleResultMetricID, queryMetricID))

		//
		// lets just see whether everything until the first : matches
		if strings.Contains(singleResultMetricID, ":") && strings.Contains(singleResultMetricID, ":") {
			ph.Logger.Debug(fmt.Sprintf("Just compare before first :\n"))

			fuzzyResultMetricID := strings.Split(singleResultMetricID, ":")[0]
			fuzzyQueryMetricID := strings.Split(queryMetricID, ":")[0]
			if strings.Compare(fuzzyResultMetricID, fuzzyQueryMetricID) == 0 {
				ph.Logger.Debug(fmt.Sprintf("FUZZY MATCH!!\n"))
				return true
			}
		}

		// TODO - more fuzzy checks
	}

	return false
}

// QueryDynatraceDashboardForSLIs implements - https://github.com/keptn-contrib/dynatrace-sli-service/issues/60
// Queries Dynatrace for the existance of a dashboard tagged with keptn_project:project, keptn_stage:stage, keptn_service:service, SLI
// if this dashboard exists it will be parsed and a custom SLI_dashboard.yaml and an SLO_dashboard.yaml will be created
// Returns:
//  #1: Link to Dashboard
//  #2: SLI
//  #3: ServiceLevelObjectives
//  #4: SLIResult
//  #5: Error
func (ph *Handler) QueryDynatraceDashboardForSLIs(project string, stage string, service string, dashboard string, startUnix time.Time, endUnix time.Time, customFilters []*keptn.SLIFilter, logger *keptn.Logger) (string, *DynatraceDashboard, *SLI, *keptn.ServiceLevelObjectives, []*keptn.SLIResult, error) {
	dashboardJSON, err := ph.loadDynatraceDashboard(project, stage, service, dashboard)
	if err != nil {
		return "", nil, nil, nil, nil, fmt.Errorf("could not load Dynatrace dashboard: %v", err)
	}

	if dashboardJSON == nil {
		return "", nil, nil, nil, nil, nil
	}

	// generate our own SLIResult array based on the dashboard configuration
	var sliResults []*keptn.SLIResult
	dashboardSLI := &SLI{}
	dashboardSLI.Indicators = make(map[string]string)
	dashboardSLO := &keptn.ServiceLevelObjectives{
		Objectives: []*keptn.SLO{},
		TotalScore: &keptn.SLOScore{Pass: "90%", Warning: "75%"},
		Comparison: &keptn.SLOComparison{CompareWith: "single_result", IncludeResultWithScore: "pass", NumberOfComparisonResults: 1, AggregateFunction: "avg"},
	}

	// parse the dashboards title and get total score pass and warning
	managementZoneFilter := ""
	if dashboardJSON.DashboardMetadata.DashboardFilter.ManagementZone != nil {
		managementZoneFilter = fmt.Sprintf(",mzId(%s)", dashboardJSON.DashboardMetadata.DashboardFilter.ManagementZone.ID)
	}

	// Dashboard Link
	// lets also generate the dashboard link for that timeframe (gtf=c_START_END) as well as management zone (gf=MZID) to pass back as label to Keptn
	mgmtZone := ""
	if dashboardJSON.DashboardMetadata.DashboardFilter.ManagementZone != nil {
		mgmtZone = ";gf=" + dashboardJSON.DashboardMetadata.DashboardFilter.ManagementZone.ID
	}
	dashboardLinkAsLabel := fmt.Sprintf("%s#dashboard;id=%s;gtf=c_%s_%s%s", ph.ApiURL, dashboardJSON.ID, common.TimestampToString(startUnix), common.TimestampToString(endUnix), mgmtZone)
	ph.Logger.Debug(fmt.Sprintf("Dashboard Link: %s\n", dashboardLinkAsLabel))

	// now lets iterate through the dashboard to find our SLIs
	for _, tile := range dashboardJSON.Tiles {
		if tile.TileType == "SYNTHETIC_TESTS" {
			// we dont do markdowns or synthetic tests
			continue
		}

		if tile.TileType == "MARKDOWN" {
			// we allow the user to use a markdown to specify SLI/SLO properties, e.g: KQG.Total.Pass
			// if we find KQG. we process the markdown
			if strings.Contains(tile.Markdown, "KQG.") {
				ParseMarkdownConfiguration(tile.Markdown, dashboardSLO)
			}

			continue
		}

		// custom chart and usql have different ways to define their tile names - so - lets figure it out by looking at the potential values
		tileTitle := tile.FilterConfig.CustomName // this is for all custom charts
		if tileTitle == "" {
			tileTitle = tile.CustomName
		}

		// first - lets figure out if this tile should be included in SLI validation or not - we parse the title and look for "sli=sliname"
		baseIndicatorName, passSLOs, warningSLOs, weight, keySli := ParsePassAndWarningFromString(tileTitle, []string{}, []string{})
		if baseIndicatorName == "" {
			logger.Debug(fmt.Sprintf("Chart Tile %s - NOT included as name doesnt include sli=SLINAME\n", tileTitle))
			continue
		}

		// only interested in custom charts
		if tile.TileType == "CUSTOM_CHARTING" {
			logger.Debug(fmt.Sprintf("Processing custom chart tile %s, sli=%s", tileTitle, baseIndicatorName))

			// we can potentially have multiple series on that chart
			for _, series := range tile.FilterConfig.ChartConfig.Series {

				// Lets query the metric definition as we need to know how many dimension the metric has
				metricDefinition, err := ph.ExecuteMetricAPIDescribe(series.Metric)
				if err != nil {
					logger.Debug(fmt.Sprintf("Error retrieving Metric Description for %s: %s\n", series.Metric, err.Error()))
					continue
				}

				// building the merge aggregator string, e.g: merge(1):merge(0) - or merge(0)
				metricDimensionCount := len(metricDefinition.DimensionDefinitions)
				metricAggregation := metricDefinition.DefaultAggregation.Type
				mergeAggregator := ""
				filterAggregator := ""
				filterSLIDefinitionAggregator := ""

				// now we need to merge all the dimensions that are not part of the series.dimensions, e.g: if the metric has two dimensions but only one dimension is used in the chart we need to merge the others
				// as multiple-merges are possible but as they are executed in sequence we have to use the right index
				for metricDimIx := metricDimensionCount - 1; metricDimIx >= 0; metricDimIx-- {
					doMergeDimension := true
					metricDimIxAsString := strconv.Itoa(metricDimIx)
					// lets check if this dimension is in the chart
					for _, seriesDim := range series.Dimensions {
						logger.Debug(fmt.Sprintf("seriesDim.id: %s; metricDimIx: %s\n", seriesDim.ID, metricDimIxAsString))
						if strings.Compare(seriesDim.ID, metricDimIxAsString) == 0 {
							// this is a dimension we want to keep and not merge
							logger.Debug(fmt.Sprintf("not merging dimension %s\n", metricDefinition.DimensionDefinitions[metricDimIx].Name))
							doMergeDimension = false

							// lets check if we need to apply a dimension filter
							// TODO: support multiple filters - right now we only support 1
							if len(seriesDim.Values) > 0 {
								filterAggregator = fmt.Sprintf(":filter(eq(%s,%s))", seriesDim.Name, seriesDim.Values[0])
							} else {
								// we need this for the generation of the SLI for each individual dimension value
								filterSLIDefinitionAggregator = fmt.Sprintf(":filter(eq(%s,FILTERDIMENSIONVALUE))", seriesDim.Name)
							}
						}
					}

					if doMergeDimension {
						// this is a dimension we want to merge as it is not split by in the chart
						logger.Debug(fmt.Sprintf("merging dimension %s\n", metricDefinition.DimensionDefinitions[metricDimIx].Name))
						mergeAggregator = mergeAggregator + fmt.Sprintf(":merge(%d)", metricDimIx)
					}
				}

				// handle aggregation. If "NONE" is specified we go to the defaultAggregration
				if series.Aggregation != "NONE" {
					metricAggregation = series.Aggregation
				}
				// for percentile we need to specify the percentile itself
				if metricAggregation == "PERCENTILE" {
					metricAggregation = fmt.Sprintf("%s(%f)", metricAggregation, series.Percentile)
				}
				// for rate measures such as failure rate we take average if it is "OF_INTEREST_RATIO"
				if metricAggregation == "OF_INTEREST_RATIO" {
					metricAggregation = "avg"
				}
				// for rate measures charting also provides the "OTHER_RATIO" option which is the inverse
				// TODO: not supported via API - so we default to avg
				if metricAggregation == "OTHER_RATIO" {
					metricAggregation = "avg"
				}

				// TODO - handle aggregation rates -> probably doesnt make sense as we always evalute a short timeframe
				// if series.AggregationRate

				// lets get the true entity type as the one in the dashboard might not be accurate, e.g: IOT might be used instead of CUSTOM_DEVICE
				// so - if the metric definition has EntityTypes defined we take the first one
				entityType := series.EntityType
				if len(metricDefinition.EntityType) > 0 {
					entityType = metricDefinition.EntityType[0]
				}

				// lets create the metricSelector and entitySelector
				// ATTENTION: adding :names so we also get the names of the dimensions and not just the entities. This means we get two values for each dimension
				metricQuery := fmt.Sprintf("metricSelector=%s%s%s:%s:names;entitySelector=type(%s)%s",
					series.Metric, mergeAggregator, filterAggregator, strings.ToLower(metricAggregation),
					entityType, managementZoneFilter)

				// lets build the Dynatrace API Metric query for the proposed timeframe and additonal filters!
				fullMetricQuery, metricID := ph.BuildDynatraceMetricsQuery(metricQuery, startUnix, endUnix, customFilters)

				// Lets run the Query and iterate through all data per dimension. Each Dimension will become its own indicator
				queryResult, err := ph.ExecuteMetricsAPIQuery(fullMetricQuery)
				if err != nil {
					// ERROR-CASE: Metric API return no values or an error
					// we couldnt query data - so - we return the error back as part of our SLIResults
					sliResults = append(sliResults, &keptn.SLIResult{
						Metric:  baseIndicatorName,
						Value:   0,
						Success: false, // Mark as failure
						Message: err.Error(),
					})

					// add this to our SLI Indicator JSON in case we need to generate an SLI.yaml
					dashboardSLI.Indicators[baseIndicatorName] = metricQuery
				} else {
					// SUCCESS-CASE: we retrieved values - now we interate through the results and create an indicator result for every dimension
					logger.Debug(fmt.Sprintf("received query result\n"))
					for _, singleResult := range queryResult.Result {
						logger.Debug(fmt.Sprintf("Processing result for %s\n", singleResult.MetricID))
						if ph.isMatchingMetricID(singleResult.MetricID, metricID) {
							dataResultCount := len(singleResult.Data)
							if dataResultCount == 0 {
								logger.Debug(fmt.Sprintf("No data for this metric!\n"))
							}
							for _, singleDataEntry := range singleResult.Data {
								//
								// we need to generate the indicator name based on the base name + all dimensions, e.g: teststep_MYTESTSTEP, teststep_MYOTHERTESTSTEP
								// EXCEPTION: If there is only ONE data value then we skip this and just use the base SLI name
								indicatorName := baseIndicatorName

								// we need this one to "fake" the MetricQuery for the SLi.yaml to include the dynamic dimension name for each value
								// we initialize it with ":names" as this is the part of the metric query string we will replace
								filterSLIDefinitionAggregatorValue := ":names"

								if dataResultCount > 1 {
									// because we use the ":names" transformation we always get two dimension names. First is the NAme, then the ID
									// lets first validate that we really received Dimension Names
									dimensionCount := len(singleDataEntry.Dimensions)
									dimensionIncrement := 2
									if dimensionCount != (len(series.Dimensions) * 2) {
										logger.Debug(fmt.Sprintf("DIDNT RECEIVE ID and Names. Lets assume we just received the dimension IDs"))
										dimensionIncrement = 1
									}

									// lets iterate through the list and get all names
									for dimIx := 0; dimIx < len(singleDataEntry.Dimensions); dimIx = dimIx + dimensionIncrement {
										dimensionName := singleDataEntry.Dimensions[dimIx]
										indicatorName = indicatorName + "_" + dimensionName

										filterSLIDefinitionAggregatorValue = ":names" + strings.Replace(filterSLIDefinitionAggregator, "FILTERDIMENSIONVALUE", dimensionName, 1)
									}
								}

								// make sure we have a valid indicator name by getting rid of special characters
								indicatorName = cleanIndicatorName(indicatorName)

								// calculating the value
								value := 0.0
								for _, singleValue := range singleDataEntry.Values {
									value = value + singleValue
								}
								value = value / float64(len(singleDataEntry.Values))

								// lets scale the metric
								value = scaleData(metricDefinition.MetricID, metricDefinition.Unit, value)

								// we got our metric, slos and the value

								logger.Debug(fmt.Sprintf("%s: %0.2f\n", indicatorName, value))

								// lets add the value to our SLIResult array
								sliResults = append(sliResults, &keptn.SLIResult{
									Metric:  indicatorName,
									Value:   value,
									Success: true,
								})

								// add this to our SLI Indicator JSON in case we need to generate an SLI.yaml
								// we use ":names" to find the right spot to add our custom dimension filter
								dashboardSLI.Indicators[indicatorName] = strings.Replace(metricQuery, ":names", filterSLIDefinitionAggregatorValue, 1)

								//
								// grabnerandi - Aug 26th
								// if passSLOs or warningSLOs are an empty list dont pass them at all. otherwise this will cause an issue with the lighthouse
								if len(passSLOs) == 0 {
									passSLOs = nil
								}
								if len(warningSLOs) == 0 {
									warningSLOs = nil
								}

								// lets add the SLO definitin in case we need to generate an SLO.yaml
								sloDefinition := &keptn.SLO{
									SLI:     indicatorName,
									Weight:  weight,
									KeySLI:  keySli,
									Pass:    passSLOs,
									Warning: warningSLOs,
								}
								dashboardSLO.Objectives = append(dashboardSLO.Objectives, sloDefinition)
							}
						} else {
							logger.Debug(fmt.Sprintf("Retrieving unintened metric %s while expecting %s\n", singleResult.MetricID, metricID))
						}
					}
				}
			}
		}

		// Dynatrace Query Language
		if tile.TileType == "DTAQL" {

			// for Dynatrace Query Language we currently support the following
			// SINGLE_VALUE: we just take the one value that comes back
			// PIE_CHART, COLUMN_CHART: we assume the first column is the dimension and the second column is the value column
			// TABLE: we assume the first column is the dimension and the last is the value

			usql := ph.BuildDynatraceUSQLQuery(tile.Query, startUnix, endUnix)
			usqlResult, err := ph.ExecuteUSQLQuery(usql)

			if err != nil {

			} else {

				for _, rowValue := range usqlResult.Values {
					dimensionName := ""
					dimensionValue := 0.0

					if tile.Type == "SINGLE_VALUE" {
						dimensionValue = rowValue[0].(float64)
					} else if tile.Type == "PIE_CHART" {
						dimensionName = rowValue[0].(string)
						dimensionValue = rowValue[1].(float64)
					} else if tile.Type == "COLUMN_CHART" {
						dimensionName = rowValue[0].(string)
						dimensionValue = rowValue[1].(float64)
					} else if tile.Type == "TABLE" {
						dimensionName = rowValue[0].(string)
						dimensionValue = rowValue[len(rowValue)-1].(float64)
					} else {
						ph.Logger.Debug(fmt.Sprintf("USQL Tile Type %s currently not supported!", tile.Type))
						continue
					}

					// lets scale the metric
					// value = scaleData(metricDefinition.MetricID, metricDefinition.Unit, value)

					// we got our metric, slos and the value
					indicatorName := baseIndicatorName
					if dimensionName != "" {
						indicatorName = indicatorName + "_" + dimensionName
					}

					ph.Logger.Debug(fmt.Sprintf("%s: %0.2f\n", indicatorName, dimensionValue))

					// lets add the value to our SLIResult array
					sliResults = append(sliResults, &keptn.SLIResult{
						Metric:  indicatorName,
						Value:   dimensionValue,
						Success: true,
					})

					// add this to our SLI Indicator JSON in case we need to generate an SLI.yaml
					dashboardSLI.Indicators[indicatorName] = tile.Query

					// lets add the SLO definitin in case we need to generate an SLO.yaml
					sloDefinition := &keptn.SLO{
						SLI:     indicatorName,
						Weight:  weight,
						KeySLI:  keySli,
						Pass:    passSLOs,
						Warning: warningSLOs,
					}
					dashboardSLO.Objectives = append(dashboardSLO.Objectives, sloDefinition)
				}
			}
		}
	}

	return dashboardLinkAsLabel, dashboardJSON, dashboardSLI, dashboardSLO, sliResults, nil
}

// GetSLIValue queries a single metric value from Dynatrace API
func (ph *Handler) GetSLIValue(metric string, startUnix time.Time, endUnix time.Time, customFilters []*keptn.SLIFilter) (float64, error) {
	// first we get the query from the SLI configuration based on its logical name
	ph.Logger.Debug(fmt.Sprintf("Getting SLI config for %s\n", metric))
	metricsQuery, err := ph.getTimeseriesConfig(metric)
	if err != nil {
		return 0, fmt.Errorf("Error when fetching timeseries config: %s\n", err.Error())
	}

	// now we are enriching it with all the additonal parameters, e.g: time, filters ...
	metricsQuery, metricID := ph.BuildDynatraceMetricsQuery(metricsQuery, startUnix, endUnix, customFilters)

	ph.Logger.Debug(fmt.Sprintf("trying to fetch metric %s", metricID))
	result, err := ph.ExecuteMetricsAPIQuery(metricsQuery)

	if err != nil {
		return 0, fmt.Errorf("error from Execute Metrics API Query: %s\n", err.Error())
	}

	var (
		metricIDExists    = false
		actualMetricValue = 0.0
	)

	if result != nil {
		for _, i := range result.Result {

			if ph.isMatchingMetricID(i.MetricID, metricID) {
				metricIDExists = true

				if len(i.Data) != 1 {
					jsonString, _ := json.Marshal(i)
					return 0, fmt.Errorf("Dynatrace Metrics API returned %d result values, expected 1. Please ensure the response contains exactly one value (e.g., by using :merge(0):avg for the metric). Here is the output for troubleshooting: %s", len(i.Data), string(jsonString))
				}

				actualMetricValue = i.Data[0].Values[0]
				break
			}
		}
	}

	if !metricIDExists {
		return 0, fmt.Errorf("Dynatrace Metrics API result does not contain identifier %s", metricID)
	}

	return scaleData(metricID, "", actualMetricValue), nil
}

// scales data based on the timeseries identifier (e.g., service.responsetime needs to be scaled from microseconds to milliseocnds)
func scaleData(metricID string, unit string, value float64) float64 {
	if (strings.Compare(unit, "MicroSecond") == 0) || strings.Contains(metricID, "builtin:service.response.time") {
		// scale from microseconds to milliseconds
		return value / 1000.0
	}

	// convert Bytes to Kilobyte
	if strings.Compare(unit, "Byte") == 0 {
		return value / 1024
	}

	/*
		if strings.Compare(unit, "NanoSecond") {

		}
	*/

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
	/* query = strings.Replace(query, "$PROJECT", ph.Project, -1)
	query = strings.Replace(query, "$STAGE", ph.Stage, -1)
	query = strings.Replace(query, "$SERVICE", ph.Service, -1)
	query = strings.Replace(query, "$DEPLOYMENT", ph.Deployment, -1)*/

	query = common.ReplaceKeptnPlaceholders(query, ph.KeptnEvent)

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
		// ?metricSelector=builtin:service.requestCount.total:merge(0):count&
		return "builtin:service.requestCount.total:merge(0):sum?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT)", nil
	case ErrorRate:
		return "builtin:service.errors.total.count:merge(0):avg?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT)", nil
	case ResponseTimeP50:
		return "builtin:service.response.time:merge(0):percentile(50)?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT)", nil
	case ResponseTimeP90:
		return "builtin:service.response.time:merge(0):percentile(90)?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT)", nil
	case ResponseTimeP95:
		return "builtin:service.response.time:merge(0):percentile(95)?scope=tag(keptn_project:$PROJECT),tag(keptn_stage:$STAGE),tag(keptn_service:$SERVICE),tag(keptn_deployment:$DEPLOYMENT)", nil
	default:
		fmt.Sprintf("Unknown metric %s\n", metric)
		return "", fmt.Errorf("unsupported SLI metric %s", metric)
	}
}
