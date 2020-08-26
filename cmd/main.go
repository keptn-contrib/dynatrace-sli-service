package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/keptn-contrib/dynatrace-sli-service/pkg/common"
	"github.com/keptn-contrib/dynatrace-sli-service/pkg/lib/dynatrace"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	cloudeventshttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"

	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"

	"gopkg.in/yaml.v2"

	keptn "github.com/keptn/go-utils/pkg/lib"
	// configutils "github.com/keptn/go-utils/pkg/configuration-service/utils"
	// keptnevents "github.com/keptn/go-utils/pkg/events"
	// keptnutils "github.com/keptn/go-utils/pkg/utils"
	// v1 "k8s.io/client-go/kubernetes/typed/core/// "
)

const eventbroker = "EVENTBROKER"
const configservice = "CONFIGURATION_SERVICE"
const sliResourceURI = "dynatrace/sli.yaml"

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

	t, err := cloudeventshttp.New(
		cloudeventshttp.WithPort(env.Port),
		cloudeventshttp.WithPath(env.Path),
	)

	if err != nil {
		log.Fatalf("failed to create transport, %v", err)
	}
	c, err := client.New(t)
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}

	log.Fatalf("failed to start receiver: %s", c.StartReceiver(ctx, gotEvent))

	return 0
}

/**
 * Handles Events
 */
func gotEvent(ctx context.Context, event cloudevents.Event) error {

	switch event.Type() {
	case keptn.InternalGetSLIEventType:
		return retrieveMetrics(event)
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
func ensureRightTimestamps(start string, end string) (time.Time, time.Time, error) {

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
		fmt.Printf("ERROR: Supplied end-time %v is too far (>120seconds) in the future (now: %v - diff in sec: %v)\n", endUnix, now, timeDiffInSeconds)
		return startUnix, endUnix, errors.New("end time must not be in the future")
	}

	// ensure start time is before end time
	timeframeInSeconds := endUnix.Sub(startUnix).Seconds()
	if timeframeInSeconds < 0 {
		fmt.Printf("ERROR: Start time needs to be before end time\n")
		return startUnix, endUnix, errors.New("start time needs to be before end time")
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

	// log outpout if we are waiting
	if time.Now().Sub(endUnix).Seconds() < waitForSeconds {
		fmt.Printf("As the end date is too close to Now() we are going to wait to make sure we have all the data for the requested timeframe(start-end)\n")
	}

	// make sure the end timestamp is at least waitForSeconds seconds in the past such that dynatrace metrics API has processed data
	for time.Now().Sub(endUnix).Seconds() < waitForSeconds {
		// ToDo: this should be done in main.go
		fmt.Printf("Sleeping for %d seconds... (waiting for Dynatrace Metrics API)\n", int(waitForSeconds-time.Now().Sub(endUnix).Seconds()))
		time.Sleep(10 * time.Second)
	}

	return startUnix, endUnix, nil
}

/**
 * Tries to find a dynatrace dashboard that matches our project. If so - returns the SLI, SLO and SLIResults
 */
func getDataFromDynatraceDashboard(dynatraceHandler *dynatrace.Handler, keptnEvent *common.BaseKeptnEvent, startUnix time.Time, endUnix time.Time, dashboardConfig string, customFilters []*keptn.SLIFilter, logger *keptn.Logger) (string, []*keptn.SLIResult, error) {
	//
	// Option 1: We query the data from a dashboard instead of the uploaded SLI.yaml
	// ==============================================================================
	// Lets see if we have a Dashboard in Dynatrace that we should parse
	logger.Info("Query Dynatrace Dashboards to see if there is an SLI dashboard available")
	dashboardLinkAsLabel, dashboardJSON, dashboardSLI, dashboardSLO, sliResults, err := dynatraceHandler.QueryDynatraceDashboardForSLIs(keptnEvent.Project, keptnEvent.Stage, keptnEvent.Service, dashboardConfig, startUnix, endUnix, customFilters, logger)
	if err != nil {
		return dashboardLinkAsLabel, sliResults, err
	}

	// lets store the dashboard as well
	if dashboardJSON != nil {
		logger.Info("Dynatrace Dashboard")
		jsonAsByteArray, _ := json.MarshalIndent(dashboardJSON, "", "  ")

		err := common.UploadKeptnResource(jsonAsByteArray, "dynatrace/dashboard.json", keptnEvent, logger)
		if err != nil {
			return dashboardLinkAsLabel, sliResults, err
		}
	}

	// lets write the SLI to the config repo
	if dashboardSLI != nil {
		logger.Info("Generated SLI.yaml from Dynatrace Dashboard")
		yamlAsByteArray, _ := yaml.Marshal(dashboardSLI)

		err := common.UploadKeptnResource(yamlAsByteArray, "dynatrace/sli.yaml", keptnEvent, logger)
		if err != nil {
			return dashboardLinkAsLabel, sliResults, err
		}
	}

	// lets write the SLO to the config repo
	if dashboardSLO != nil {
		logger.Info("Generated SLO.yaml from Dynatrace Dashboard")
		yamlAsByteArray, _ := yaml.Marshal(dashboardSLO)

		err := common.UploadKeptnResource(yamlAsByteArray, "slo.yaml", keptnEvent, logger)
		if err != nil {
			return dashboardLinkAsLabel, sliResults, err

		}
	}

	// lets also write the result to a local file in local test mode
	if sliResults != nil {
		if common.RunLocal || common.RunLocalTest {
			logger.Info("(RunLocal Output) Write SLIResult to sliresult.json")
			jsonAsByteArray, _ := json.MarshalIndent(sliResults, "", "  ")

			common.UploadKeptnResource(jsonAsByteArray, "sliresult.json", keptnEvent, logger)
		}
	}

	return dashboardLinkAsLabel, sliResults, nil
}

/**
 * Handles keptn.InternalGetSLIEventType
 *
 * First tries to find a Dynatrace dashboard and then parses it for SLIs and SLOs
 * Second will go to parse the SLI.yaml and returns the SLI as passed in by the event
 */
func retrieveMetrics(event cloudevents.Event) error {
	var shkeptncontext string
	event.Context.ExtensionAs("shkeptncontext", &shkeptncontext)
	eventData := &keptn.InternalGetSLIEventData{}
	err := event.DataAs(eventData)
	if err != nil {
		return err
	}

	//
	// Lets get a new Keptn Handler
	keptnHandler, err := keptn.NewKeptn(&event, keptn.KeptnOpts{
		ConfigurationServiceURL: os.Getenv(configservice),
	})
	if err != nil {
		return err
	}

	//
	// do not continue if SLIProvider is not dynatrace
	if eventData.SLIProvider != "dynatrace" {
		return nil
	}

	//
	// Lets get a Logger
	stdLogger := keptn.NewLogger(shkeptncontext, event.Context.GetID(), "dynatrace-sli-service")
	stdLogger.Info("Retrieving Dynatrace timeseries metrics")

	//
	// see if there is a dynatrace.conf.yaml
	keptnEvent := &common.BaseKeptnEvent{}
	keptnEvent.Project = eventData.Project
	keptnEvent.Stage = eventData.Stage
	keptnEvent.Service = eventData.Service
	keptnEvent.TestStrategy = eventData.TestStrategy
	keptnEvent.Labels = eventData.Labels
	keptnEvent.Context = shkeptncontext
	dynatraceConfigFile, _ := common.GetDynatraceConfig(keptnEvent, stdLogger)

	dtCreds := ""
	if dynatraceConfigFile != nil {
		dtCreds = dynatraceConfigFile.DtCreds
		stdLogger.Debug("Found dynatrace.conf.yaml with DTCreds: " + dtCreds)
	} else {
		stdLogger.Debug("Using default DTCreds: dynatrace as no custom dynatrace.conf.yaml was found!")
		dynatraceConfigFile = &common.DynatraceConfigFile{}
		dynatraceConfigFile.Dashboard = ""
		dynatraceConfigFile.DtCreds = "dynatrace"
	}
	dtCredentials, err := getDynatraceCredentials(dtCreds, eventData.Project, stdLogger)

	//
	// grabnerandi - Aug 26th 2020
	// Adding DtCreds as a label so users know which DtCreds was used
	if eventData.Labels == nil {
		eventData.Labels = make(map[string]string)
	}
	eventData.Labels["DtCreds"] = dynatraceConfigFile.DtCreds

	if err != nil {
		stdLogger.Error(err.Error())
		stdLogger.Debug("Failed to fetch Dynatrace credentials, exiting.")
		// Implementing: https://github.com/keptn-contrib/dynatrace-sli-service/issues/49
		return sendInternalGetSLIDoneEvent(shkeptncontext, eventData.Project, eventData.Service, eventData.Stage,
			nil, eventData.Start, eventData.End, eventData.TestStrategy, eventData.DeploymentStrategy,
			eventData.Deployment, eventData.Labels, eventData.Indicators, err)
	}

	//
	// creating Dynatrace Handler which allows us to call the Dynatrace API
	stdLogger.Info("Dynatrace credentials (Tenant, Token) received")
	dynatraceHandler := dynatrace.NewDynatraceHandler(dtCredentials.Tenant,
		keptnEvent,
		map[string]string{
			"Authorization": "Api-Token " + dtCredentials.ApiToken,
		},
		eventData.CustomFilters,
	)

	//
	// parse start and end (which are datetime strings) and convert them into unix timestamps
	startUnix, endUnix, err := ensureRightTimestamps(eventData.Start, eventData.End)
	if err != nil {
		return sendInternalGetSLIDoneEvent(shkeptncontext, eventData.Project, eventData.Service, eventData.Stage,
			nil, eventData.Start, eventData.End, eventData.TestStrategy, eventData.DeploymentStrategy,
			eventData.Deployment, eventData.Labels, eventData.Indicators, err)
	}

	//
	// THIS IS OUR RETURN OBJECT: sliResult
	// Whether option 1 or option 2 - this will hold our SLIResults
	var sliResults []*keptn.SLIResult

	//
	// Option 1 - see if we can get the data from a Dnatrace Dashboard
	dashboardLinkAsLabel, sliResults, err := getDataFromDynatraceDashboard(dynatraceHandler, keptnEvent, startUnix, endUnix, dynatraceConfigFile.Dashboard, eventData.CustomFilters, stdLogger)

	// IF we received data from processing the dashboard send it back - we are done here :-)
	if sliResults != nil {
		// add link to dynatrace dashboard to labels
		if dashboardLinkAsLabel != "" {
			if eventData.Labels == nil {
				eventData.Labels = make(map[string]string)
			}
			eventData.Labels["Dashboard Link"] = dashboardLinkAsLabel
		}

		/* stdLogger.Info("Captured SLIResults from Dynatrace Dashboard")
		jsonAsString, _ := json.Marshal(sliResults)
		stdLogger.Info(string(jsonAsString)) */
	} else {
		//
		// Option 2: We query the SLIs based on the definitions stored in SLI.yaml
		// ========================================================================0
		// get custom metrics for project
		stdLogger.Info("Load indicators from SLI.yaml")
		projectCustomQueries, err := getCustomQueries(keptnEvent, keptnHandler, stdLogger)
		if err != nil {
			stdLogger.Error("Failed to get custom queries for project " + eventData.Project)
			stdLogger.Error(err.Error())
			return sendInternalGetSLIDoneEvent(shkeptncontext, eventData.Project, eventData.Service, eventData.Stage,
				nil, eventData.Start, eventData.End, eventData.TestStrategy, eventData.DeploymentStrategy,
				eventData.Deployment, eventData.Labels, eventData.Indicators, err)
		}

		// set our list of queries on the handler
		if projectCustomQueries != nil {
			dynatraceHandler.CustomQueries = projectCustomQueries
		}

		// query all indicators
		for _, indicator := range eventData.Indicators {
			stdLogger.Info("Fetching indicator: " + indicator)
			sliValue, err := dynatraceHandler.GetSLIValue(indicator, startUnix, endUnix, eventData.CustomFilters)
			if err != nil {
				stdLogger.Error(err.Error())
				// failed to fetch metric
				sliResults = append(sliResults, &keptn.SLIResult{
					Metric:  indicator,
					Value:   0,
					Success: false, // Mark as failure
					Message: err.Error(),
				})
			} else {
				// successfully fetched metric
				sliResults = append(sliResults, &keptn.SLIResult{
					Metric:  indicator,
					Value:   sliValue,
					Success: true, // mark as success
				})
			}
		}

		log.Println("Finished fetching metrics; Sending event now ...")

		if common.RunLocal || common.RunLocalTest {
			log.Println("(RunLocal Output) Here are the results:")
			for _, v := range sliResults {
				log.Println(fmt.Sprintf("%s:%.2f - Success: %t - Error: %s", v.Metric, v.Value, v.Success, v.Message))
			}
			return nil
		}

	}

	// now - lets see if we have captured any result values - if not - return send an error
	err = nil
	if sliResults == nil {
		err = errors.New("Couldnt retrieve any SLI Results")
	}

	//

	return sendInternalGetSLIDoneEvent(shkeptncontext, eventData.Project, eventData.Service, eventData.Stage,
		sliResults, eventData.Start, eventData.End, eventData.TestStrategy, eventData.DeploymentStrategy,
		eventData.Deployment, eventData.Labels, eventData.Indicators, err)
}

/**
 * Loads SLIs from a local file and adds it to the SLI map
 */
func addResourceContentToSLIMap(SLIs map[string]string, sliFilePath string, sliFileContent string, logger *keptn.Logger) (map[string]string, error) {

	if sliFilePath != "" {
		localFileContent, err := ioutil.ReadFile(sliFilePath)
		if err != nil {
			logMessage := fmt.Sprintf("Couldn't load file content from %s", sliFilePath)
			logger.Info(logMessage)
			return nil, nil
		}
		logger.Info("Loaded LOCAL file " + sliFilePath)
		sliFileContent = string(localFileContent)
	} else {
		// we just take what was passed in the sliFileContent
	}

	if sliFileContent != "" {
		sliConfig := keptn.SLIConfig{}
		err := yaml.Unmarshal([]byte(sliFileContent), &sliConfig)
		if err != nil {
			return nil, err
		}

		for key, value := range sliConfig.Indicators {
			SLIs[key] = value
		}
	}
	return SLIs, nil
}

// getCustomQueries returns custom queries as stored in configuration store
func getCustomQueries(keptnEvent *common.BaseKeptnEvent, keptnHandler *keptn.Keptn, logger *keptn.Logger) (map[string]string, error) {
	logger.Info(fmt.Sprintf("Checking for custom SLI queries for project=%s,stage=%s,service=%s", keptnEvent.Project, keptnEvent.Stage, keptnEvent.Service))

	var sliMap = map[string]string{}
	if common.RunLocal || common.RunLocalTest {
		sliMap, _ = addResourceContentToSLIMap(sliMap, "dynatrace/sli.yaml", "", logger)
		return sliMap, nil
	}

	sliContent, err := common.GetKeptnResource(keptnEvent, sliResourceURI, logger)
	if err != nil {
		return nil, err
	}

	sliMap, _ = addResourceContentToSLIMap(sliMap, "", sliContent, logger)
	return sliMap, nil

	// AG-2020-07-17 - after migrating to the new GoUtil SDK this function did no longer return the SLI Configuration - always returned "resource not found"
	/* customQueries, err := keptnHandler.GetSLIConfiguration(project, stage, service, sliResourceURI)
	if err != nil {
		return nil, err
	}

	return customQueries, nil*/
}

/**
 * returns the DTCredentials
 * First looks at the passed secretName. If null validates if there is a dynatrace-credentials-%PROJECT% - if not - defaults to "dynatrace" global secret
 */
func getDynatraceCredentials(secretName string, project string, logger *keptn.Logger) (*common.DTCredentials, error) {

	secretNames := []string{secretName, fmt.Sprintf("dynatrace-credentials-%s", project), "dynatrace-credentials", "dynatrace"}

	for _, secret := range secretNames {
		if secret == "" {
			continue
		}

		logger.Info(fmt.Sprintf("Trying to fetch secret containing Dynatrace credentials with name '%s'", secret))
		dtCredentials, err := common.GetDTCredentials(secret)

		// write in log if fetching Dynatrace Credentials failed
		if err != nil {
			logger.Error(fmt.Sprintf("Error fetching secret containing Dynatrace credentials with name '%s': %s", secret, err.Error()))
		}

		if dtCredentials != nil {
			// lets validate if the tenant URL is
			logger.Info(fmt.Sprintf("Secret with credentials found, returning (%s) ...", dtCredentials.Tenant))
			return dtCredentials, nil
		}
	}

	return nil, errors.New("Could not find any Dynatrace specific secrets")
}

/**
 * Sends the SLI Done Event. If err != nil it will send an error message
 */
func sendInternalGetSLIDoneEvent(shkeptncontext string, project string,
	service string, stage string, indicatorValues []*keptn.SLIResult, start string, end string,
	teststrategy string, deploymentStrategy string, deployment string, labels map[string]string, indicators []string, err error) error {

	source, _ := url.Parse("dynatrace-sli-service")
	contentType := "application/json"

	// if an error was set - the indicators will be set to failed and error message is set to each
	if err != nil {
		errMessage := err.Error()

		if (indicatorValues == nil) || (len(indicatorValues) == 0) {
			if indicators == nil || len(indicators) == 0 {
				indicators = []string{"no metric"}
			}

			for _, indicatorName := range indicators {
				indicatorValues = []*keptn.SLIResult{
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

	getSLIEvent := keptn.InternalGetSLIDoneEventData{
		Project:            project,
		Service:            service,
		Stage:              stage,
		IndicatorValues:    indicatorValues,
		Start:              start,
		End:                end,
		TestStrategy:       teststrategy,
		DeploymentStrategy: deploymentStrategy,
		Deployment:         deployment,
		Labels:             labels,
	}
	event := cloudevents.Event{
		Context: cloudevents.EventContextV02{
			ID:          uuid.New().String(),
			Time:        &types.Timestamp{Time: time.Now()},
			Type:        keptn.InternalGetSLIDoneEventType,
			Source:      types.URLRef{URL: *source},
			ContentType: &contentType,
			Extensions:  map[string]interface{}{"shkeptncontext": shkeptncontext},
		}.AsV02(),
		Data: getSLIEvent,
	}

	return sendEvent(event)
}

func sendEvent(event cloudevents.Event) error {

	keptnHandler, err := keptn.NewKeptn(&event, keptn.KeptnOpts{})
	if err != nil {
		return err
	}

	_ = keptnHandler.SendCloudEvent(event)

	return nil
}
