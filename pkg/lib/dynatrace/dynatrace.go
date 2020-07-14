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

/**
 * Struct for SLI.yaml
 */
type SLI struct {
	SpecVersion string            `yaml:"spec_version"`
	Indicators  map[string]string `yaml:"indicators"`
}

/**
 * Output of /dashboards
 */
type DynatraceDashboards struct {
	Dashboards []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Owner string `json:"owner"`
	} `json:"dashboards"`
}

/**
 * Output of /dashboards/<dashboardID>
 */
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
		DashboardFilter struct {
			Timeframe      string      `json:"timeframe"`
			ManagementZone interface{} `json:"managementZone"`
		} `json:"dashboardFilter"`
		Tags []string `json:"tags"`
	} `json:"dashboardMetadata"`
	Tiles []struct {
		Name       string `json:"name"`
		TileType   string `json:"tileType"`
		Configured bool   `json:"configured"`
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
						ID              string      `json:"id"`
						Name            string      `json:"name"`
						Values          interface{} `json:"values"`
						EntityDimension bool        `json:"entitiyDimension"`
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

/**
 * Defines the output of /metrics/<metricID>
 */
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

/**
 * Result of /metrics/query
 */
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
}

// NewDynatraceHandler returns a new dynatrace handler that interacts with the Dynatrace REST API
func NewDynatraceHandler(apiURL string, keptnEvent *common.BaseKeptnEvent, headers map[string]string, customFilters []*keptn.SLIFilter) *Handler {
	ph := &Handler{
		ApiURL:        apiURL,
		KeptnEvent:    keptnEvent,
		HTTPClient:    &http.Client{},
		Headers:       headers,
		CustomFilters: customFilters,
	}

	return ph
}

/**
 * Queries all Dynatrace Dashboards and returns the dashboard ID that matches KQG;project=%project%;service=%service%;stage=%stage;xxx
 */
func (ph *Handler) findDynatraceDashboard(project string, stage string, service string) (string, error) {
	// Lets query the list of all Dashboards and find the one that matches project, stage, service based on the title (in the future - we can do it via tags)
	// create dashboard query URL and set additional headers
	fmt.Printf("Query all dashboards\n")
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

	fmt.Println("Request finished, parsing dashboard list response body...")
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
	dashboardsJSON := &DynatraceDashboards{}

	// parse json
	err = json.Unmarshal(body, &dashboardsJSON)

	if err != nil {
		return "", err
	}

	// now - lets iterate through the list and find one that matches our project, stage, service ...
	findValues := []string{fmt.Sprintf("project=%s", project), fmt.Sprintf("service=%s", service), fmt.Sprintf("stage=%s", stage)}
	for _, dashboard := range dashboardsJSON.Dashboards {
		if strings.HasPrefix(dashboard.Name, "KQG;") {
			fmt.Println("Analyzing if Dashboard matches: " + dashboard.Name)

			nameSplits := strings.Split(dashboard.Name, ";")

			// now lets see if we can find all our name/value pairs for project, service & stage
			dashboardMatch := true
			for _, findValue := range findValues {
				foundValue := false
				for _, nameSplitValue := range nameSplits {
					if strings.Compare(findValue, nameSplitValue) == 0 {
						foundValue = true
					}
				}
				if foundValue == false {
					dashboardMatch = false
					continue
				}
			}

			if dashboardMatch {
				fmt.Println("Found Dashboard Match: " + dashboard.ID)
				return dashboard.ID, nil
			}
		}
	}

	return "", nil
}

/**
 * getDynatraceDashboard: will either query the passed dashboard id - or - if none is passed - will try to find a dashboard that matches project, stage, service (based on name or tags)
 * the parsed JSON object is returned
 */
func (ph *Handler) getDynatraceDashboard(project string, stage string, service string, dashboard string) (*DynatraceDashboard, error) {
	if dashboard == "" {
		dashboard, _ = ph.findDynatraceDashboard(project, stage, service)
	}

	if dashboard == "" {
		return nil, nil
	}

	// create dashboard query URL and set additional headers
	fmt.Printf("Query dashboard with ID: %s", dashboard)
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

	fmt.Println("Request finished, parsing dashboard response body...")
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
	dashboardJSON := &DynatraceDashboard{}

	// parse json
	err = json.Unmarshal(body, &dashboardJSON)

	if err != nil {
		return nil, err
	}

	return dashboardJSON, err
}

/**
 * Calls the /metrics/<metricID> API call to retrieve Metric Definition Details
 */
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

	fmt.Println("Request finished, parsing body...")

	body, _ := ioutil.ReadAll(resp.Body)

	fmt.Println(string(body) + "\n")

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

/**
 * Executes the passed Metrics API Call, validates that the call returns data and returns the data set
 */
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

	fmt.Println("Request finished, parsing body...")

	body, _ := ioutil.ReadAll(resp.Body)

	fmt.Println(string(body) + "\n")

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

/**
* Builds the complete query string based on start, end and filters
* metricQuery should contain metricSelector and entitySelector
* Returns:
  #1: Finalized Dynatrace API Query
  #2: MetricID that this query will return, e.g: builtin:host.cpu
  #3: error
*/
func (ph *Handler) BuildDynatraceMetricsQuery(metricquery string, startUnix time.Time, endUnix time.Time, customFilters []*keptn.SLIFilter) (string, string) {
	fmt.Printf("Finalize query for %s\n", metricquery)

	// replace query params (e.g., $PROJECT, $STAGE, $SERVICE ...)
	metricquery = ph.replaceQueryParameters(metricquery)

	if strings.HasPrefix(metricquery, "?metricSelector=") {
		fmt.Printf("COMPATIBILITY WARNING: Provided query string %s is not compatible. Auto-removing the ? in front (see %s for details).\n", metricquery, MetricsAPIOldFormatNewFormatDoc)
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
		fmt.Printf("COMPATIBILITY WARNING: Your query %s still uses the old format (see %s for details).\n", metricQueryParams, MetricsAPIOldFormatNewFormatDoc)
		// old format with "?" - everything left of the ? is the identifier, everything right are query params
		metricSelector = querySplit[0]

		// build the new query
		metricQueryParams = fmt.Sprintf("metricSelector=%s&%s", querySplit[0], querySplit[1])
	}

	targetUrl := ph.ApiURL + fmt.Sprintf("/api/v2/metrics/query/?%s", metricQueryParams)

	// default query params that are required: resolution, from and to
	queryParams := map[string]string{
		"resolution": "Inf", // resolution=Inf means that we only get 1 datapoint (per service)
		"from":       common.TimestampToString(startUnix),
		"to":         common.TimestampToString(endUnix),
	}
	// fmt.Println("Query Params initially:")
	// fmt.Println(queryParams)

	// append queryParams to targetUrl
	u, _ := url.Parse(targetUrl)
	q, _ := url.ParseQuery(u.RawQuery)

	for param, value := range queryParams {
		q.Add(param, value)
	}

	// check if q contains "scope"
	scopeData := q.Get("scope")

	// compatibility with old scope=... custom queries
	if scopeData != "" {
		fmt.Printf("COMPATIBILITY WARNING: You are still using scope=... - querying the new metrics API requires use of entitySelector=... instead (see %s for details).", MetricsAPIOldFormatNewFormatDoc)
		// scope is no longer supported in the new API, it needs to be called "entitySelector" and contain type(SERVICE)
		if !strings.Contains(scopeData, "type(SERVICE)") {
			fmt.Printf("COMPATIBILITY WARNING: Automatically adding type(SERVICE) to entitySelector=... for compatibility with the new Metrics API (see %s for details).", MetricsAPIOldFormatNewFormatDoc)
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
	fmt.Println("Final Query=", u.String())

	return u.String(), metricSelector
}

/**
 * Takes a value such as "teststep_rt;pass=<500ms,<+10%;warning=<1000ms,<+20%"
 * can also take a value like "KQG;project=myproject;pass=90%;warning=75%"
 * This will return
 * #1: teststep_rt
 * #2: ["<500ms","<+10%"]
 * #3: ["<1000ms","<+20%"]
 */
func ParsePassAndWarningFromString(customName string, defaultPass []string, defaultWarning []string) (string, []string, []string) {
	splits := strings.Split(customName, ";")

	if len(splits) == 0 {
		return "", nil, nil
	}

	passSplit := defaultPass
	warningSplit := defaultWarning
	for i := 1; i < len(splits); i++ {
		sloSplit := strings.Split(splits[i], "=")
		if sloSplit[0] == "pass" {
			passSplit = strings.Split(sloSplit[1], ",")
		}
		if sloSplit[0] == "warning" {
			warningSplit = strings.Split(sloSplit[1], ",")
		}
	}

	return splits[0], passSplit, warningSplit
}

/**  Implements - https://github.com/keptn-contrib/dynatrace-sli-service/issues/60
* Queries Dynatrace for the existance of a dashboard tagged with keptn_project:project, keptn_stage:stage, keptn_service:service, SLI
* if this dashboard exists it will be parsed and a custom SLI_dashboard.yaml and an SLO_dashboard.yaml will be created
* Returns:
  #1: Link to Dashboard
  #2: SLI
  #3: ServiceLevelObjectives
  #4: SLIResult
  #5: Error
*/
func (ph *Handler) QueryDynatraceDashboardForSLIs(project string, stage string, service string, dashboard string, startUnix time.Time, endUnix time.Time, customFilters []*keptn.SLIFilter, logger *keptn.Logger) (string, *SLI, *keptn.ServiceLevelObjectives, []*keptn.SLIResult, error) {
	dashboardJSON, err := ph.getDynatraceDashboard(project, stage, service, dashboard)
	if err != nil {
		return "", nil, nil, nil, err
	}

	if dashboardJSON == nil {
		return "", nil, nil, nil, nil
	}

	// we generate our own SLIResult array based on the dashboard configuration
	var sliResults []*keptn.SLIResult
	dashboardSLI := &SLI{}
	dashboardSLI.Indicators = make(map[string]string)
	dashboardSLO := &keptn.ServiceLevelObjectives{
		Objectives: []*keptn.SLO{},
		TotalScore: &keptn.SLOScore{},
	}

	// Lets parse the dashboards title and get total score pass and warning
	_, globalPassSplit, globalWarningSplit := ParsePassAndWarningFromString(dashboardJSON.DashboardMetadata.Name, []string{"90%"}, []string{"75%"})
	dashboardSLO.TotalScore.Pass = globalPassSplit[0]
	dashboardSLO.TotalScore.Warning = globalWarningSplit[0]

	// lets also generate the dashboard link for that timeframe to pass back as label to Keptn
	dashboardLinkAsLabel := fmt.Sprintf("%s#dashboard;id=%s;gtf=c_%s_%s", ph.ApiURL, dashboardJSON.ID, common.TimestampToString(startUnix), common.TimestampToString(endUnix))
	fmt.Printf("Dashboard Link: %s\n", dashboardLinkAsLabel)

	// now lets iterate through the dashboard to find our SLIs
	for _, tile := range dashboardJSON.Tiles {

		// only interested in custom charts
		if tile.TileType == "CUSTOM_CHARTING" {

			fmt.Printf("Processing custom chart: %s\n", tile.FilterConfig.CustomName)
			// lets start by extracting the base SLI Indicator name from the tile header, e.g: teststep_rt: pass: <500ms;<+10%; warning: <1000ms;<+20% translates to teststep_rt
			baseIndicatorName, passSLOs, warningSLOs := ParsePassAndWarningFromString(tile.FilterConfig.CustomName, []string{}, []string{})

			// we can potentially have multiple series on that chart
			for _, series := range tile.FilterConfig.ChartConfig.Series {

				// Lets query the metric definition as we need to know how many dimension the metric has
				metricDefinition, err := ph.ExecuteMetricAPIDescribe(series.Metric)
				if err != nil {
					fmt.Printf("Error retrieving Metric Description for %s: %s\n", series.Metric, err.Error())
					continue
				}

				// building the merge aggregator string, e.g: merge(1):merge(0) - or merge(0)
				metricDimensionCount := len(metricDefinition.DimensionDefinitions)
				mergeAggregator := ""

				// now we need to merge all the dimensions that are not part of the series.dimensions, e.g: if the metric has two dimensions but only one dimension is used in the chart we need to merge the others
				// as multiple-merges are possible but as they are executed in sequence we have to use the right index
				for metricDimIx := metricDimensionCount - 1; metricDimIx >= 0; metricDimIx-- {
					doMergeDimension := true
					metricDimIxAsString := strconv.Itoa(metricDimIx)
					// lets check if this dimension is in the chart
					for _, seriesDim := range series.Dimensions {
						fmt.Printf("seriesDim.id: %s; metricDimIx: %s\n", seriesDim.ID, metricDimIxAsString)
						if strings.Compare(seriesDim.ID, metricDimIxAsString) == 0 {
							// this is a dimension we want to keep and not merge
							fmt.Printf("not merging dimension %s\n", metricDefinition.DimensionDefinitions[metricDimIx].Name)
							doMergeDimension = false
						}
					}

					if doMergeDimension {
						// this is a dimension we want to merge as it is not split by in the chart
						fmt.Printf("merging dimension %s\n", metricDefinition.DimensionDefinitions[metricDimIx].Name)
						mergeAggregator = mergeAggregator + fmt.Sprintf(":merge(%d)", metricDimIx)
					}
				}

				// lets create the metricSelector and entitySelector
				metricQuery := fmt.Sprintf("metricSelector=%s%s:%s;entitySelector=type(%s)",
					series.Metric, mergeAggregator, strings.ToLower(series.Aggregation),
					series.EntityType)

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
					fmt.Printf("received query result\n")
					for _, singleResult := range queryResult.Result {
						fmt.Printf("Processing result for %s\n", singleResult.MetricID)
						if singleResult.MetricID == metricID {
							if len(singleResult.Data) == 0 {
								fmt.Printf("No data for this metric!\n")
							}
							for _, singleDataEntry := range singleResult.Data {
								// we need to generate the indicator name based on the base name + all dimensions
								indicatorName := baseIndicatorName
								for _, dimension := range singleDataEntry.Dimensions {
									indicatorName = indicatorName + "_" + dimension
								}

								// make sure we have a valid indicator name by getting rid of special characters
								// TODO: check more than just blanks
								baseIndicatorName = strings.ReplaceAll(baseIndicatorName, " ", "_")

								// calculating the value
								value := 0.0
								for _, singleValue := range singleDataEntry.Values {
									value = value + singleValue
								}
								value = value / float64(len(singleDataEntry.Values))

								// we got our metric, slos and the value
								fmt.Printf("%s: pass=%s, warning=%s, value=%0.2f\n", indicatorName, passSLOs, warningSLOs, value)

								// lets add the value to our SLIResult array
								sliResults = append(sliResults, &keptn.SLIResult{
									Metric:  indicatorName,
									Value:   value,
									Success: true,
								})

								// add this to our SLI Indicator JSON in case we need to generate an SLI.yaml
								dashboardSLI.Indicators[indicatorName] = metricQuery

								// lets add the SLO definitin in case we need to generate an SLO.yaml
								sloDefinition := &keptn.SLO{
									SLI:     indicatorName,
									Weight:  1,
									Pass:    []*keptn.SLOCriteria{{Criteria: passSLOs}},
									Warning: []*keptn.SLOCriteria{{Criteria: warningSLOs}},
								}
								dashboardSLO.Objectives = append(dashboardSLO.Objectives, sloDefinition)
							}
						} else {
							fmt.Printf("Retrieving unintened metric %s while expecting %s\n", singleResult.MetricID, metricID)
						}
					}
				}
			}
		}
	}

	return dashboardLinkAsLabel, dashboardSLI, dashboardSLO, sliResults, nil
}

func (ph *Handler) GetSLIValue(metric string, startUnix time.Time, endUnix time.Time, customFilters []*keptn.SLIFilter) (float64, error) {
	// disable SSL verification (probably not a good idea for dynatrace)
	// http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	// AG-27052020: When using keptn send event start-evaluation and clocks are not 100% in sync, e.g: workstation is 1-2 seconds off
	//              we might run into the issue that we detect the endtime to be in the future. I ran into this problem after my laptop ran out of sync for about 1.5s
	//              to circumvent this issue I am changing the check to also allow a time difference of up to 2 minutes (120 seconds). This shouldnt be a problem as our SLI Service retries the DYnatrace API anyway
	// Here is the issue: https://github.com/keptn-contrib/dynatrace-sli-service/issues/55
	// ensure end time is not in the future
	now := time.Now()
	timeDiffInSeconds := now.Sub(endUnix).Seconds()
	if timeDiffInSeconds < -120 { // used to be 0
		fmt.Printf("ERROR: Supplied end-time %v is in the future (now: %v - diff in sec: %v)\n", endUnix, now, timeDiffInSeconds)
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

	// first we get the query from the SLI configuration based on its logical name
	fmt.Printf("Getting SLI config for %s\n", metric)
	metricsQuery, err := ph.getTimeseriesConfig(metric)
	if err != nil {
		fmt.Printf("Error when fetching timeseries config: %s\n", err.Error())
		return 0, err
	}

	// now we are enriching it with all the additonal parameters, e.g: time, filters ...
	metricsQuery, metricID := ph.BuildDynatraceMetricsQuery(metricsQuery, startUnix, endUnix, customFilters)

	fmt.Println("trying to fetch metric", metricID)
	result, err := ph.ExecuteMetricsAPIQuery(metricsQuery)

	var (
		metricIDExists    = false
		actualMetricValue = 0.0
	)
	for _, i := range result.Result {

		if i.MetricID == metricID {
			metricIDExists = true

			if len(i.Data) != 1 {
				return 0, fmt.Errorf("Dynatrace Metrics API returned %d result values, expected 1. Please ensure the response contains exactly one value (e.g., by using :merge(0):avg for the metric)", len(i.Data))
			}

			actualMetricValue = i.Data[0].Values[0]
		}
	}

	if !metricIDExists {
		return 0, fmt.Errorf("Dynatrace Metrics API result does not contain identifier %s", metricID)
	}

	return scaleData(metricID, actualMetricValue), nil
}

// scales data based on the timeseries identifier (e.g., service.responsetime needs to be scaled from microseconds
// to milliseocnds)
func scaleData(metricId string, value float64) float64 {
	if strings.Contains(metricId, "builtin:service.response.time") {
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
