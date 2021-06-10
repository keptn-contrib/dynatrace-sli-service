package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	keptncommon "github.com/keptn/go-utils/pkg/lib"

	"github.com/keptn-contrib/dynatrace-sli-service/pkg/common"
	"github.com/keptn-contrib/dynatrace-sli-service/pkg/lib/dynatrace"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/kelseyhightower/envconfig"

	"gopkg.in/yaml.v2"

	"github.com/keptn/go-utils/pkg/lib/keptn"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"
	// configutils "github.com/keptn/go-utils/pkg/configuration-service/utils"
	// keptnevents "github.com/keptn/go-utils/pkg/events"
	// keptnutils "github.com/keptn/go-utils/pkg/utils"
	// v1 "k8s.io/client-go/kubernetes/typed/core/// "
)

const ProblemOpenSLI = "problem_open"

type envConfig struct {
	// Port on which to listen for cloudevents
	Port int    `envconfig:"RCV_PORT" default:"8080"`
	Path string `envconfig:"RCV_PATH" default:"/"`
}

func main() {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatalf("Failed to process env var: %s", err)
	}

	if common.RunLocal || common.RunLocalTest {
		log.Println("env=runlocal: Running with local filesystem to fetch resources")
	}

	os.Exit(_main(os.Args[1:], env))
}

func _main(args []string, env envConfig) int {

	ctx := context.Background()
	ctx = cloudevents.WithEncodingStructured(ctx)

	p, err := cloudevents.NewHTTP(cloudevents.WithPath(env.Path), cloudevents.WithPort(env.Port))
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}
	c, err := cloudevents.NewClient(p)
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}
	log.Fatal(c.StartReceiver(ctx, gotEvent))
	return 0
}

/**
 * Handles Events
 */
func gotEvent(ctx context.Context, event cloudevents.Event) error {

	switch event.Type() {
	case keptnv2.GetTriggeredEventType(keptnv2.GetSLITaskName):
		// prepare event
		eventData := &keptnv2.GetSLITriggeredEventData{}
		err := event.DataAs(eventData)
		if err != nil {
			return err
		}

		//
		// do not continue if SLIProvider is not dynatrace
		if eventData.GetSLI.SLIProvider != "dynatrace" {
			return nil
		}

		go retrieveMetrics(event, eventData)

		return nil
	default:
		return errors.New("received unknown event type")
	}
}

/**
 * AG-27052020: When using keptn send event start-evaluation and clocks are not 100% in sync, e.g: workstation is 1-2 seconds off
 *              we might run into the issue that we detect the endtime to be in the future. I ran into this problem after my laptop ran out of sync for about 1.5s
 *              to circumvent this issue I am changing the check to also allow a time difference of up to 2 minutes (120 seconds). This shouldnt be a problem as our SLI Service retries the DYnatrace API anyway
 * Here is the issue: https://github.com/keptn-contrib/dynatrace-sli-service/issues/55
 */
func ensureRightTimestamps(start string, end string, logger keptn.LoggerInterface) (time.Time, time.Time, error) {

	startUnix, err := common.ParseUnixTimestamp(start)
	if err != nil {
		return time.Now(), time.Now(), errors.New("Error parsing start date: " + err.Error())
	}
	endUnix, err := common.ParseUnixTimestamp(end)
	if err != nil {
		return startUnix, time.Now(), errors.New("Error parsing end date: " + err.Error())
	}

	// ensure end time is not in the future
	now := time.Now()
	timeDiffInSeconds := now.Sub(endUnix).Seconds()
	if timeDiffInSeconds < -120 { // used to be 0
		return startUnix, endUnix, fmt.Errorf("error validating time range: Supplied end-time %v is too far (>120seconds) in the future (now: %v - diff in sec: %v)\n", endUnix, now, timeDiffInSeconds)
	}

	// ensure start time is before end time
	timeframeInSeconds := endUnix.Sub(startUnix).Seconds()
	if timeframeInSeconds < 0 {
		return startUnix, endUnix, errors.New("error validating time range: start time needs to be before end time")
	}

	// AG-2020-07-16: Wait so Dynatrace has enough data but dont wait every time to shorten processing time
	// if we have a very short evaluation window and the end timestampe is now then we need to give Dynatrace some time to make sure we have relevant data
	// if the evalutaion timeframe is > 2 minutes we dont wait and just live with the fact that we may miss one minute or two at the end

	waitForSeconds := 120.0        // by default lets make sure we are at least 120 seconds away from "now()"
	if timeframeInSeconds >= 300 { // if our evaluated timeframe however is larger than 5 minutes its ok to continue right away. 5 minutes is the default timeframe for most evaluations
		waitForSeconds = 0.0
	} else if timeframeInSeconds >= 120 { // if the evaluation span is between 2 and 5 minutes make sure we at least have the last minute of data
		waitForSeconds = 60.0
	}

	// log output while we are waiting
	if time.Now().Sub(endUnix).Seconds() < waitForSeconds {
		logger.Debug(fmt.Sprintf("As the end date is too close to Now() we are going to wait to make sure we have all the data for the requested timeframe(start-end)\n"))
	}

	// make sure the end timestamp is at least waitForSeconds seconds in the past such that dynatrace metrics API has processed data
	for time.Now().Sub(endUnix).Seconds() < waitForSeconds {
		logger.Debug(fmt.Sprintf("Sleeping for %d seconds... (waiting for Dynatrace Metrics API)\n", int(waitForSeconds-time.Now().Sub(endUnix).Seconds())))
		time.Sleep(10 * time.Second)
	}

	return startUnix, endUnix, nil
}

/**
 * Adds an SLO Entry to the SLO.yaml
 */
func addSLO(keptnEvent *common.BaseKeptnEvent, newSLO *keptncommon.SLO, logger *keptn.Logger) error {

	// this is the default SLO in case none has yet been uploaded
	dashboardSLO := &keptncommon.ServiceLevelObjectives{
		Objectives: []*keptncommon.SLO{},
		TotalScore: &keptncommon.SLOScore{Pass: "90%", Warning: "75%"},
		Comparison: &keptncommon.SLOComparison{CompareWith: "single_result", IncludeResultWithScore: "pass", NumberOfComparisonResults: 1, AggregateFunction: "avg"},
	}

	// first - lets load the SLO.yaml from the config repo
	sloContent, err := common.GetKeptnResource(keptnEvent, common.KeptnSLOFilename, logger)
	if err == nil && sloContent != "" {
		err := json.Unmarshal([]byte(sloContent), dashboardSLO)
		if err != nil {
			return fmt.Errorf("Couldnt parse existing SLO.yaml: %v", err)
		}
	}

	// now we add the SLO Definition to the objectives - but first validate if it is not already there
	for _, objective := range dashboardSLO.Objectives {
		if objective.SLI == newSLO.SLI {
			return nil
		}
	}

	// now - lets add our newSLO to the list
	dashboardSLO.Objectives = append(dashboardSLO.Objectives, newSLO)

	// and now we save it back to Keptn
	if dashboardSLO != nil {
		yamlAsByteArray, _ := yaml.Marshal(dashboardSLO)

		err := common.UploadKeptnResource(yamlAsByteArray, common.KeptnSLOFilename, keptnEvent, logger)
		if err != nil {
			return fmt.Errorf("could not store %s : %v", common.KeptnSLOFilename, err)
		}
	}

	return nil
}

/**
 * Tries to find a dynatrace dashboard that matches our project. If so - returns the SLI, SLO and SLIResults
 */
func getDataFromDynatraceDashboard(dynatraceHandler *dynatrace.Handler, keptnEvent *common.BaseKeptnEvent, startUnix time.Time, endUnix time.Time, dashboardConfig string) (string, []*keptnv2.SLIResult, error) {

	//
	// Option 1: We query the data from a dashboard instead of the uploaded SLI.yaml
	// ==============================================================================
	// Lets see if we have a Dashboard in Dynatrace that we should parse
	dashboardLinkAsLabel, dashboardJSON, dashboardSLI, dashboardSLO, sliResults, err := dynatraceHandler.QueryDynatraceDashboardForSLIs(keptnEvent, dashboardConfig, startUnix, endUnix)
	if err != nil {
		return dashboardLinkAsLabel, sliResults, fmt.Errorf("could not query Dynatrace dashboard for SLIs: %v", err)
	}

	// lets store the dashboard as well
	if dashboardJSON != nil {
		jsonAsByteArray, _ := json.MarshalIndent(dashboardJSON, "", "  ")

		err := common.UploadKeptnResource(jsonAsByteArray, common.DynatraceDashboardFilename, keptnEvent, dynatraceHandler.Logger)
		if err != nil {
			return dashboardLinkAsLabel, sliResults, fmt.Errorf("could not store %s : %v", common.DynatraceDashboardFilename, err)
		}
	}

	// lets write the SLI to the config repo
	if dashboardSLI != nil {
		yamlAsByteArray, _ := yaml.Marshal(dashboardSLI)

		err := common.UploadKeptnResource(yamlAsByteArray, common.DynatraceSLIFilename, keptnEvent, dynatraceHandler.Logger)
		if err != nil {
			return dashboardLinkAsLabel, sliResults, fmt.Errorf("could not store %s : %v", common.DynatraceSLIFilename, err)
		}
	}

	// lets write the SLO to the config repo
	if dashboardSLO != nil {
		yamlAsByteArray, _ := yaml.Marshal(dashboardSLO)

		err := common.UploadKeptnResource(yamlAsByteArray, common.KeptnSLOFilename, keptnEvent, dynatraceHandler.Logger)
		if err != nil {
			return dashboardLinkAsLabel, sliResults, fmt.Errorf("could not store %s : %v", common.KeptnSLOFilename, err)
		}
	}

	// lets also write the result to a local file in local test mode
	if sliResults != nil {
		if common.RunLocal || common.RunLocalTest {
			dynatraceHandler.Logger.Info("(RunLocal Output) Write SLIResult to sliresult.json")
			jsonAsByteArray, _ := json.MarshalIndent(sliResults, "", "  ")

			common.UploadKeptnResource(jsonAsByteArray, "sliresult.json", keptnEvent, dynatraceHandler.Logger)
		}
	}

	return dashboardLinkAsLabel, sliResults, nil
}

/**
 * getDynatraceProblemContext
 *
 * Will evaluate the event and - if it finds a dynatrace problem ID - will return this - otherwise it will return 0
 */
func getDynatraceProblemContext(eventData *keptnv2.GetSLITriggeredEventData) string {

	// iterate through the labels and find Problem URL
	if eventData.Labels == nil || len(eventData.Labels) == 0 {
		return ""
	}

	for labelName, labelValue := range eventData.Labels {
		if strings.ToLower(labelName) == "problem url" {
			// the value should be of form https://dynatracetenant/#problems/problemdetails;pid=8485558334848276629_1604413609638V2
			// so - lets get the last part after pid=

			ix := strings.LastIndex(labelValue, ";pid=")
			if ix > 0 {
				return labelValue[ix+5:]
			}
		}
	}

	return ""
}

/**
 * Handles keptn.InternalGetSLIEventType
 *
 * First tries to find a Dynatrace dashboard and then parses it for SLIs and SLOs
 * Second will go to parse the SLI.yaml and returns the SLI as passed in by the event
 */
func retrieveMetrics(event cloudevents.Event, eventData *keptnv2.GetSLITriggeredEventData) error {
	// extract keptn context id
	var shkeptncontext string
	event.Context.ExtensionAs("shkeptncontext", &shkeptncontext)

	// send get-sli.started event
	if err := sendGetSLIStartedEvent(event, eventData); err != nil {
		return sendGetSLIFinishedEvent(event, eventData, nil, err)
	}

	//
	// Lets get a Logger
	stdLogger := keptn.NewLogger(shkeptncontext, event.Context.GetID(), "dynatrace-sli-service")
	stdLogger.Info(fmt.Sprintf("Processing sh.keptn.internal.event.get-sli for %s.%s.%s", eventData.Project, eventData.Stage, eventData.Service))

	keptnEvent := &common.BaseKeptnEvent{}
	keptnEvent.Project = eventData.Project
	keptnEvent.Stage = eventData.Stage
	keptnEvent.Service = eventData.Service
	keptnEvent.Labels = eventData.Labels
	keptnEvent.Deployment = eventData.Deployment
	keptnEvent.Context = shkeptncontext

	dynatraceConfigFile := common.GetDynatraceConfig(keptnEvent, stdLogger)

	// Adding DtCreds as a label so users know which DtCreds was used
	if eventData.Labels == nil {
		eventData.Labels = make(map[string]string)
	}
	eventData.Labels["DtCreds"] = dynatraceConfigFile.DtCreds

	dtCredentials, err := getDynatraceCredentials(dynatraceConfigFile.DtCreds, eventData.Project, stdLogger)
	if err != nil {
		stdLogger.Error("Failed to fetch Dynatrace credentials: " + err.Error())
		// Implementing: https://github.com/keptn-contrib/dynatrace-sli-service/issues/49
		return sendGetSLIFinishedEvent(event, eventData, nil, err)
	}

	//
	// creating Dynatrace Handler which allows us to call the Dynatrace API
	dynatraceHandler := dynatrace.NewDynatraceHandler(
		dtCredentials.Tenant,
		keptnEvent,
		map[string]string{
			"Authorization": "Api-Token " + dtCredentials.ApiToken,
			"User-Agent":    "keptn-contrib/dynatrace-sli-service:" + os.Getenv("version"),
		},
		eventData.GetSLI.CustomFilters, shkeptncontext, event.ID())

	//
	// parse start and end (which are datetime strings) and convert them into unix timestamps
	startUnix, endUnix, err := ensureRightTimestamps(eventData.GetSLI.Start, eventData.GetSLI.End, stdLogger)
	if err != nil {
		stdLogger.Error(err.Error())
		return sendGetSLIFinishedEvent(event, eventData, nil, err)
	}

	//
	// THIS IS OUR RETURN OBJECT: sliResult
	// Whether option 1 or option 2 - this will hold our SLIResults
	var sliResults []*keptnv2.SLIResult

	//
	// Option 1 - see if we can get the data from a Dnatrace Dashboard
	dashboardLinkAsLabel, sliResults, err := getDataFromDynatraceDashboard(dynatraceHandler, keptnEvent, startUnix, endUnix, dynatraceConfigFile.Dashboard)
	if err != nil {
		// log the error, but continue with loading sli.yaml
		stdLogger.Error(err.Error())
	}

	// add link to dynatrace dashboard to labels
	if dashboardLinkAsLabel != "" {
		if eventData.Labels == nil {
			eventData.Labels = make(map[string]string)
		}
		eventData.Labels["Dashboard Link"] = dashboardLinkAsLabel
	}

	//
	// Option 2: If we have not received any data via a Dynatrace Dashboard lets query the SLIs based on the SLI.yaml definition
	if sliResults == nil {
		// get custom metrics for project if they exist
		projectCustomQueries, _ := common.GetCustomQueries(keptnEvent, stdLogger)

		// set our list of queries on the handler
		if projectCustomQueries != nil {
			dynatraceHandler.CustomQueries = projectCustomQueries
		}

		// query all indicators
		for _, indicator := range eventData.GetSLI.Indicators {
			if strings.Compare(indicator, ProblemOpenSLI) == 0 {
				stdLogger.Info("Skip " + indicator + " as it is handled later!")
			} else {
				stdLogger.Info("Fetching indicator: " + indicator)
				sliValue, err := dynatraceHandler.GetSLIValue(indicator, startUnix, endUnix)
				if err != nil {
					stdLogger.Error(err.Error())
					// failed to fetch metric
					sliResults = append(sliResults, &keptnv2.SLIResult{
						Metric:  indicator,
						Value:   0,
						Success: false, // Mark as failure
						Message: err.Error(),
					})
				} else {
					// successfully fetched metric
					sliResults = append(sliResults, &keptnv2.SLIResult{
						Metric:  indicator,
						Value:   sliValue,
						Success: true, // mark as success
					})
				}
			}
		}

		if common.RunLocal || common.RunLocalTest {
			log.Println("(RunLocal Output) Here are the results:")
			for _, v := range sliResults {
				log.Println(fmt.Sprintf("%s:%.2f - Success: %t - Error: %s", v.Metric, v.Value, v.Success, v.Message))
			}
			return nil
		}
	}

	//
	// ARE WE CALLED IN CONTEXT OF A PROBLEM REMEDIATION??
	// If so - we should try to query the status of the Dynatrace Problem that triggered this evaluation
	problemID := getDynatraceProblemContext(eventData)
	if problemID != "" {
		problemIndicator := ProblemOpenSLI
		openProblemValue := 0.0
		success := false
		message := ""

		// lets query the status of this problem and add it to the SLI Result
		dynatraceProblem, err := dynatraceHandler.ExecuteGetDynatraceProblemById(problemID)
		if err != nil {
			message = err.Error()
		}

		if dynatraceProblem != nil {
			success = true
			if dynatraceProblem.Status == "OPEN" {
				openProblemValue = 1.0
			}
		}

		// lets add this to the sliResults
		sliResults = append(sliResults, &keptnv2.SLIResult{
			Metric:  problemIndicator,
			Value:   openProblemValue,
			Success: success,
			Message: message,
		})

		// lets add this to the SLO in case this indicator is not yet in SLO.yaml. Becuase if it doesnt get added the lighthouse wont evaluate the SLI values
		// we default it to open_problems<=0
		sloString := fmt.Sprintf("sli=%s;pass=<=0;key=true", problemIndicator)
		_, passSLOs, warningSLOs, weight, keySli := common.ParsePassAndWarningFromString(sloString, []string{}, []string{})
		sloDefinition := &keptncommon.SLO{
			SLI:     problemIndicator,
			Weight:  weight,
			KeySLI:  keySli,
			Pass:    passSLOs,
			Warning: warningSLOs,
		}
		addSLO(keptnEvent, sloDefinition, dynatraceHandler.Logger)
	}

	// now - lets see if we have captured any result values - if not - return send an error
	err = nil
	if sliResults == nil {
		err = errors.New("Couldn't retrieve any SLI Results")
	}

	stdLogger.Info("Finished fetching metrics; Sending SLIDone event now ...")

	return sendGetSLIFinishedEvent(event, eventData, sliResults, err)
}

/**
 * returns the DTCredentials
 * First looks at the passed secretName. If null, validates if there is a dynatrace-credentials-%PROJECT% - if not - defaults to "dynatrace" global secret
 */
func getDynatraceCredentials(secretName string, project string, logger *keptn.Logger) (*common.DTCredentials, error) {

	secretNames := []string{secretName, fmt.Sprintf("dynatrace-credentials-%s", project), "dynatrace-credentials", "dynatrace"}

	for _, secret := range secretNames {
		if secret == "" {
			continue
		}

		dtCredentials, err := common.GetDTCredentials(secret)

		if err == nil && dtCredentials != nil {
			// lets validate if the tenant URL is
			logger.Info(fmt.Sprintf("Secret '%s' with credentials found, returning (%s) ...", secret, dtCredentials.Tenant))
			return dtCredentials, nil
		}
	}

	return nil, errors.New("Could not find any Dynatrace specific secrets with the following names: " + strings.Join(secretNames, ","))
}

/**
 * Sends the SLI Done Event. If err != nil it will send an error message
 */
func sendGetSLIFinishedEvent(inputEvent cloudevents.Event, eventData *keptnv2.GetSLITriggeredEventData, indicatorValues []*keptnv2.SLIResult, err error) error {

	source, _ := url.Parse("dynatrace-sli-service")

	// if an error was set - the indicators will be set to failed and error message is set to each
	if err != nil {
		errMessage := err.Error()

		if (indicatorValues == nil) || (len(indicatorValues) == 0) {
			if eventData.GetSLI.Indicators == nil || len(eventData.GetSLI.Indicators) == 0 {
				eventData.GetSLI.Indicators = []string{"no metric"}
			}

			for _, indicatorName := range eventData.GetSLI.Indicators {
				indicatorValues = []*keptnv2.SLIResult{
					{
						Metric: indicatorName,
						Value:  0.0,
					},
				}
			}
		}

		for _, indicator := range indicatorValues {
			indicator.Success = false
			indicator.Message = errMessage
		}
	}

	getSLIEvent := keptnv2.GetSLIFinishedEventData{
		EventData: keptnv2.EventData{
			Project: eventData.Project,
			Stage:   eventData.Stage,
			Service: eventData.Service,
			Labels:  eventData.Labels,
			Status:  keptnv2.StatusSucceeded,
			Result:  keptnv2.ResultPass,
		},

		GetSLI: keptnv2.GetSLIFinished{
			IndicatorValues: indicatorValues,
			Start:           eventData.GetSLI.Start,
			End:             eventData.GetSLI.End,
		},
	}

	keptnContext, err := inputEvent.Context.GetExtension("shkeptncontext")

	if err != nil {
		return fmt.Errorf("could not determine keptnContext of input event: %s", err.Error())
	}

	event := cloudevents.NewEvent()
	event.SetType(keptnv2.GetFinishedEventType(keptnv2.GetSLITaskName))
	event.SetSource(source.String())
	event.SetDataContentType(cloudevents.ApplicationJSON)
	event.SetExtension("shkeptncontext", keptnContext)
	event.SetExtension("triggeredid", inputEvent.ID())
	event.SetData(cloudevents.ApplicationJSON, getSLIEvent)

	return sendEvent(event)
}

func sendGetSLIStartedEvent(inputEvent cloudevents.Event, eventData *keptnv2.GetSLITriggeredEventData) error {

	source, _ := url.Parse("dynatrace-sli-service")

	getSLIStartedEvent := keptnv2.GetSLIStartedEventData{
		EventData: keptnv2.EventData{
			Project: eventData.Project,
			Stage:   eventData.Stage,
			Service: eventData.Service,
			Labels:  eventData.Labels,
			Status:  keptnv2.StatusSucceeded,
			Result:  keptnv2.ResultPass,
		},
	}

	keptnContext, err := inputEvent.Context.GetExtension("shkeptncontext")

	if err != nil {
		return fmt.Errorf("could not determine keptnContext of input event: %s", err.Error())
	}

	event := cloudevents.NewEvent()
	event.SetType(keptnv2.GetStartedEventType(keptnv2.GetSLITaskName))
	event.SetSource(source.String())
	event.SetDataContentType(cloudevents.ApplicationJSON)
	event.SetExtension("shkeptncontext", keptnContext)
	event.SetExtension("triggeredid", inputEvent.ID())
	event.SetData(cloudevents.ApplicationJSON, getSLIStartedEvent)

	return sendEvent(event)
}

/**
 * sends cloud event back to keptn
 */
func sendEvent(event cloudevents.Event) error {

	keptnHandler, err := keptnv2.NewKeptn(&event, keptn.KeptnOpts{})
	if err != nil {
		return err
	}

	_ = keptnHandler.SendCloudEvent(event)

	return nil
}
