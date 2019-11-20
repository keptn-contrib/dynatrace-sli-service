package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/keptn-contrib/dynatrace-sli-service/lib/dynatrace"
	"gopkg.in/yaml.v2"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	cloudeventshttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
	keptnevents "github.com/keptn/go-utils/pkg/events"
	keptnutils "github.com/keptn/go-utils/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const eventbroker = "EVENTBROKER"
const keptnDynatraceSliConfigMapName = "dynatrace-sli-config"

type envConfig struct {
	// Port on which to listen for cloudevents
	Port int    `envconfig:"RCV_PORT" default:"8080"`
	Path string `envconfig:"RCV_PATH" default:"/"`
}

type dynatraceCredentials struct {
	Tenant   string `json:"DT_TENANT" yaml:"DT_TENANT"`
	APIToken string `json:"DT_API_TOKEN" yaml:"DT_API_TOKEN"`
}

func main() {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatalf("Failed to process env var: %s", err)
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

func gotEvent(ctx context.Context, event cloudevents.Event) error {

	switch event.Type() {
	case keptnevents.InternalGetSLIEventType:
		return retrieveMetrics(event)
	default:
		return errors.New("received unknown event type")
	}
}

func retrieveMetrics(event cloudevents.Event) error {
	var shkeptncontext string
	event.Context.ExtensionAs("shkeptncontext", &shkeptncontext)
	eventData := &keptnevents.InternalGetSLIEventData{}
	err := event.DataAs(eventData)

	if err != nil {
		return err
	}

	// don't continue if SLIProvider != dynatrace
	if eventData.SLIProvider != "dynatrace" {
		return nil
	}

	stdLogger := keptnutils.NewLogger(shkeptncontext, event.Context.GetID(), "dynatrace-sli-service")
	stdLogger.Info("Retrieving dynatrace timeseries metrics")
	kubeClient, err := keptnutils.GetKubeAPI(true)
	if err != nil {
		log.Fatal(err)
		stdLogger.Error("could not create kube client")
		return errors.New("could not create kube client")
	}

	// fetch project specific dynatrace credentials
	dynatraceAPIUrl, apiToken, err := getProjectDynatraceCredentials(kubeClient, stdLogger, eventData.Project)

	if err != nil {
		stdLogger.Debug("Failed to fetch dynatrace credentials for project, falling back to global credentials...")
		// fallback to global dynatrace credentials (e.g., installed for dynatrace service)
		dynatraceAPIUrl, apiToken, err = getGlobalDynatraceCredentials(kubeClient, stdLogger)

		if err != nil {
			stdLogger.Debug("Failed to fetch global dynatrace credentials as well... exiting.")
			log.Fatal(err)
			return err
		}
	}

	stdLogger.Info("Dynatrace Credentials (Tenant, Token) received. Getting global custom queries...")

	// get custom metrics for Keptn installation
	customQueries, err := getGlobalCustomQueries(kubeClient, stdLogger)

	if err != nil {
		log.Println("Error fetching custom queries")
		log.Fatal(err)
		return err
	}

	// get custom metrics for project
	projectCustomQueries, err := getCustomQueriesForProject(eventData.Project, kubeClient, stdLogger)

	if err != nil {
		log.Fatal(err)
		return err
	}

	log.Printf("Custom Query Config\n")

	// make sure custom queries exists
	if customQueries == nil {
		customQueries = make(map[string]string)
	} else {
		for k, v := range customQueries {
			log.Printf("\tFound custom query %s with value %s\n", k, v)
		}
	}

	if projectCustomQueries != nil {
		log.Println("Merging custom queries with projectCustomQueries")
		// merge global custom queries and project custom queries
		for k, v := range projectCustomQueries {
			// overwrite / append project custom query on global custom queries
			customQueries[k] = v
			log.Printf("\tOverwriting custom query %s with value %s\n", k, v)
		}
	}

	dynatraceHandler := dynatrace.NewDynatraceHandler(
		dynatraceAPIUrl,
		eventData.Project,
		eventData.Stage,
		eventData.Service,
		map[string]string{
			"Authorization": "Api-Token " + apiToken,
		},
		eventData.CustomFilters,
	)

	dynatraceHandler.CustomQueries = customQueries

	if err != nil {
		return err
	}

	if dynatraceHandler == nil {
		stdLogger.Error("failed to get dynatrace handler")
		return nil
	}

	// create a new CloudEvent to store SLI Results in
	var sliResults []*keptnevents.SLIResult

	// query all indicators
	for _, indicator := range eventData.Indicators {
		stdLogger.Info("Fetching indicator: " + indicator)
		sliValue, err := dynatraceHandler.GetSLIValue(indicator, eventData.Start, eventData.End, eventData.CustomFilters)
		if err != nil {
			stdLogger.Error(err.Error())
			// failed to fetch metric
			sliResults = append(sliResults, &keptnevents.SLIResult{
				Metric:  indicator,
				Value:   0,
				Success: false, // Mark as failure
				Message: err.Error(),
			})
		} else {
			// successfully fetched metric
			sliResults = append(sliResults, &keptnevents.SLIResult{
				Metric:  indicator,
				Value:   sliValue,
				Success: true, // Mark as success
			})
		}
	}

	log.Println("Finished fetching metrics; Sending event now...")

	return sendInternalGetSLIDoneEvent(shkeptncontext, eventData.Project, eventData.Service, eventData.Stage,
		sliResults, eventData.Start, eventData.End, eventData.TestStrategy, eventData.DeploymentStrategy)
}

// Return Custom Queries for Keptn Installation
func getGlobalCustomQueries(kubeClient v1.CoreV1Interface, logger *keptnutils.Logger) (map[string]string, error) {
	logger.Info(fmt.Sprintf("Checking for custom SLI queries for Keptn installation (querying %s)", keptnDynatraceSliConfigMapName))

	configMap, err := kubeClient.ConfigMaps("keptn").Get(keptnDynatraceSliConfigMapName, metav1.GetOptions{})
	if err != nil {
		logger.Info("No global custom queries defined")
		return nil, nil
	}

	customQueries := make(map[string]string)
	err = yaml.Unmarshal([]byte(configMap.Data["custom-queries"]), &customQueries)

	if err != nil {
		logger.Info("Global custom queries found, but could not parse them: " + err.Error())
		return nil, err
	}
	logger.Info("Global custom queries found and parsed")
	return customQueries, nil
}

// Return Custom Queries for Keptn Installation
func getCustomQueriesForProject(project string, kubeClient v1.CoreV1Interface, logger *keptnutils.Logger) (map[string]string, error) {
	logger.Info(fmt.Sprintf("Checking for custom SLI queries for Keptn installation (querying %s)", keptnDynatraceSliConfigMapName))

	configMap, err := kubeClient.ConfigMaps("keptn").Get(keptnDynatraceSliConfigMapName+"-"+project, metav1.GetOptions{})
	if err != nil {
		logger.Info("No custom queries defined for project " + project)
		return nil, nil
	}

	customQueries := make(map[string]string)
	err = yaml.Unmarshal([]byte(configMap.Data["custom-queries"]), &customQueries)

	if err != nil {
		logger.Info("Project custom queries found, but could not parse them: " + err.Error())
		return nil, err
	}
	logger.Info("Project custom queries found and parsed")
	return customQueries, nil
}

// get global dynatrace credentials
func getGlobalDynatraceCredentials(kubeClient v1.CoreV1Interface, logger *keptnutils.Logger) (string, string, error) {
	secretName := "dynatrace"
	// check if secret exists
	secret, err := kubeClient.Secrets("keptn").Get(secretName, metav1.GetOptions{})

	// return cluster-internal dynatrace URL if no secret has been found
	if err != nil {
		log.Println(err)
		return "", "", errors.New(fmt.Sprintf("Could not find secret '%s' in namespace keptn.", secretName))
	}

	tenant, found := secret.Data["DT_TENANT"]

	if !found {
		return "", "", errors.New(fmt.Sprintf("Credentials %s does not contain a field 'DT_TENANT'", secretName))
	}

	api_token, found := secret.Data["DT_API_TOKEN"]

	if !found {
		return "", "", errors.New(fmt.Sprintf("Credentials %s does not contain a field 'DT_API_TOKEN'", secretName))
	}

	tenantStr := string(tenant)

	if !strings.HasPrefix(tenantStr, "http://") && !strings.HasPrefix(tenantStr, "https://") {
		tenantStr = "https://" + tenantStr
	}

	return tenantStr, string(api_token), nil
}

// get project specific dynatrace credentials
func getProjectDynatraceCredentials(kubeClient v1.CoreV1Interface, logger *keptnutils.Logger, project string) (string, string, error) {
	secretName := fmt.Sprintf("dynatrace-credentials-%s", project)
	// check if secret 'dynatrace-credentials-<project> exists
	secret, err := kubeClient.Secrets("keptn").Get(secretName, metav1.GetOptions{})

	// return cluster-internal dynatrace URL if no secret has been found
	if err != nil {
		log.Println(err)
		return "", "", errors.New(fmt.Sprintf("Could not find secret '%s' in namespace keptn.", secretName))
	}

	secretValue, found := secret.Data["dynatrace-credentials"]

	if !found {
		return "", "", errors.New(fmt.Sprintf("Credentials %s does not contain a field 'dynatrace-credentials'", secretName))
	}

	/*
		data format:
		tenant: string
		apiToken: string
	*/
	dtCreds := &dynatraceCredentials{}
	err = yaml.Unmarshal(secretValue, dtCreds)

	if err != nil {
		return "", "", errors.New(fmt.Sprintf("invalid credentials format found in secret '%s'", secretName))
	}

	if dtCreds.Tenant == "" {
		return "", "", errors.New("Tenant must not be empty")
	}
	if dtCreds.APIToken == "" {
		return "", "", errors.New("APIToken must not be empty")
	}

	dynatraceURL := ""

	// ensure URL always has http or https in front
	if strings.HasPrefix(dtCreds.Tenant, "https://") || strings.HasPrefix(dtCreds.Tenant, "http://") {
		dynatraceURL = dtCreds.Tenant
	} else {
		dynatraceURL = "https://" + dtCreds.Tenant
	}

	return dynatraceURL, dtCreds.APIToken, nil
}

func sendInternalGetSLIDoneEvent(shkeptncontext string, project string,
	service string, stage string, indicatorValues []*keptnevents.SLIResult, start string, end string,
	teststrategy string, deploymentStrategy string) error {

	source, _ := url.Parse("dynatrace-sli-service")
	contentType := "application/json"

	getSLIEvent := keptnevents.InternalGetSLIDoneEventData{
		Project:            project,
		Service:            service,
		Stage:              stage,
		IndicatorValues:    indicatorValues,
		Start:              start,
		End:                end,
		TestStrategy:       teststrategy,
		DeploymentStrategy: deploymentStrategy,
	}
	event := cloudevents.Event{
		Context: cloudevents.EventContextV02{
			ID:          uuid.New().String(),
			Time:        &types.Timestamp{Time: time.Now()},
			Type:        keptnevents.InternalGetSLIDoneEventType,
			Source:      types.URLRef{URL: *source},
			ContentType: &contentType,
			Extensions:  map[string]interface{}{"shkeptncontext": shkeptncontext},
		}.AsV02(),
		Data: getSLIEvent,
	}

	return sendEvent(event)
}

func sendEvent(event cloudevents.Event) error {
	endPoint, err := getServiceEndpoint(eventbroker)
	if err != nil {
		return errors.New("Failed to retrieve endpoint of eventbroker. %s" + err.Error())
	}

	if endPoint.Host == "" {
		return errors.New("Host of eventbroker not set")
	}

	transport, err := cloudeventshttp.New(
		cloudeventshttp.WithTarget(endPoint.String()),
		cloudeventshttp.WithEncoding(cloudeventshttp.StructuredV02),
	)
	if err != nil {
		return errors.New("Failed to create transport:" + err.Error())
	}

	c, err := client.New(transport)
	if err != nil {
		return errors.New("Failed to create HTTP client:" + err.Error())
	}

	if _, err := c.Send(context.Background(), event); err != nil {
		return errors.New("Failed to send cloudevent:, " + err.Error())
	}
	return nil
}

// getServiceEndpoint gets an endpoint stored in an environment variable and sets http as default scheme
func getServiceEndpoint(service string) (url.URL, error) {
	url, err := url.Parse(os.Getenv(service))
	if err != nil {
		return *url, fmt.Errorf("Failed to retrieve value from ENVIRONMENT_VARIABLE: %s", service)
	}

	if url.Scheme == "" {
		url.Scheme = "http"
	}

	return *url, nil
}
