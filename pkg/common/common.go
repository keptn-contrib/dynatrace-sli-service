package common

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/ghodss/yaml"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	configutils "github.com/keptn/go-utils/pkg/configuration-service/utils"

	keptnutils "github.com/keptn/go-utils/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var RunLocal = (os.Getenv("env") == "runlocal")
var RunLocalTest = (os.Getenv("env") == "runlocaltest")

/**
 * Defines the Dynatrace Configuration File structure!
 */
const DynatraceConfigFilename = "dynatrace/dynatrace.conf.yaml"
const DynatraceConfigFilenameLOCAL = "dynatrace/_dynatrace.conf.yaml"

type DynatraceConfigFile struct {
	SpecVersion string `json:"spec_version" yaml:"spec_version"`
	DtCreds     string `json:"dtCreds",omitempty yaml:"dtCreds",omitempty`
}

type DTCredentials struct {
	Tenant    string `json:"DT_TENANT" yaml:"DT_TENANT"`
	ApiToken  string `json:"DT_API_TOKEN" yaml:"DT_API_TOKEN"`
	PaaSToken string `json:"DT_PAAS_TOKEN" yaml:"DT_PAAS_TOKEN"`
}

type BaseKeptnEvent struct {
	Context string
	Source  string
	Event   string

	Project            string
	Stage              string
	Service            string
	Deployment         string
	TestStrategy       string
	DeploymentStrategy string

	Image string
	Tag   string

	Labels map[string]string
}

func GetKubernetesClient() (*kubernetes.Clientset, error) {
	if RunLocal || RunLocalTest {
		return nil, nil
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

/**
 * Returns the Keptn Domain stored in the keptn-domainconfigmap
 */
func GetKeptnDomain() (string, error) {
	kubeAPI, err := GetKubernetesClient()
	if kubeAPI == nil || err != nil {
		return "", err
	}

	keptnDomainCM, errCM := kubeAPI.CoreV1().ConfigMaps("keptn").Get("keptn-domain", metav1.GetOptions{})
	if errCM != nil {
		return "", errors.New("Could not retrieve keptn-domain ConfigMap: " + errCM.Error())
	}

	keptnDomain := keptnDomainCM.Data["app_domain"]
	return keptnDomain, nil
}

//
// replaces $ placeholders with actual values
// $CONTEXT, $EVENT, $SOURCE
// $PROJECT, $STAGE, $SERVICE, $DEPLOYMENT
// $TESTSTRATEGY
// $LABEL.XXXX  -> will replace that with a label called XXXX
// $ENV.XXXX    -> will replace that with an env variable called XXXX
// $SECRET.YYYY -> will replace that with the k8s secret called YYYY
//
func ReplaceKeptnPlaceholders(input string, keptnEvent *BaseKeptnEvent) string {
	result := input

	// first we do the regular keptn values
	result = strings.Replace(result, "$CONTEXT", keptnEvent.Context, -1)
	result = strings.Replace(result, "$EVENT", keptnEvent.Event, -1)
	result = strings.Replace(result, "$SOURCE", keptnEvent.Source, -1)
	result = strings.Replace(result, "$PROJECT", keptnEvent.Project, -1)
	result = strings.Replace(result, "$STAGE", keptnEvent.Stage, -1)
	result = strings.Replace(result, "$SERVICE", keptnEvent.Service, -1)
	result = strings.Replace(result, "$DEPLOYMENT", keptnEvent.Deployment, -1)
	result = strings.Replace(result, "$TESTSTRATEGY", keptnEvent.TestStrategy, -1)

	// now we do the labels
	for key, value := range keptnEvent.Labels {
		result = strings.Replace(result, "$LABEL."+key, value, -1)
	}

	// now we do all environment variables
	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		result = strings.Replace(result, "$ENV."+pair[0], pair[1], -1)
	}

	// TODO: iterate through k8s secrets!

	return result
}

func GetConfigurationServiceURL() string {
	if os.Getenv("CONFIGURATION_SERVICE_URL") != "" {
		return os.Getenv("CONFIGURATION_SERVICE_URL")
	}
	return "configuration-service.keptn.svc.cluster.local:8080"
}

//
// Loads dynatrace.conf for the current service
//
func GetDynatraceConfig(keptnEvent *BaseKeptnEvent, logger *keptnutils.Logger) (*DynatraceConfigFile, error) {

	logger.Info("Loading dynatrace.conf.yaml")
	// if we run in a runlocal mode we are just getting the file from the local disk
	var fileContent string
	if RunLocal {
		localFileContent, err := ioutil.ReadFile(DynatraceConfigFilenameLOCAL)
		if err != nil {
			logMessage := fmt.Sprintf("No %s file found LOCALLY for service %s in stage %s in project %s", DynatraceConfigFilenameLOCAL, keptnEvent.Service, keptnEvent.Stage, keptnEvent.Project)
			logger.Info(logMessage)
			return nil, nil
		}
		logger.Info("Loaded LOCAL file " + DynatraceConfigFilenameLOCAL)
		fileContent = string(localFileContent)
	} else {
		resourceHandler := configutils.NewResourceHandler(GetConfigurationServiceURL())

		// Lets search on SERVICE-LEVEL
		keptnResourceContent, err := resourceHandler.GetServiceResource(keptnEvent.Project, keptnEvent.Stage, keptnEvent.Service, DynatraceConfigFilename)
		if err != nil || keptnResourceContent == nil || keptnResourceContent.ResourceContent == "" {
			// Lets search on STAGE-LEVEL
			keptnResourceContent, err = resourceHandler.GetStageResource(keptnEvent.Project, keptnEvent.Stage, DynatraceConfigFilename)
			if err != nil || keptnResourceContent == nil || keptnResourceContent.ResourceContent == "" {
				// Lets search on PROJECT-LEVEL
				keptnResourceContent, err = resourceHandler.GetProjectResource(keptnEvent.Project, DynatraceConfigFilename)
				if err != nil || keptnResourceContent == nil || keptnResourceContent.ResourceContent == "" {
					logger.Debug(fmt.Sprintf("No Keptn Resource found: %s/%s/%s/%s - %s", keptnEvent.Project, keptnEvent.Stage, keptnEvent.Service, DynatraceConfigFilename, err))
					return nil, err
				}

				logger.Debug("Found " + DynatraceConfigFilename + " on project level")
			} else {
				logger.Debug("Found " + DynatraceConfigFilename + " on stage level")
			}
		} else {
			logger.Debug("Found " + DynatraceConfigFilename + " on service level")
		}
		fileContent = keptnResourceContent.ResourceContent
	}

	// unmarshal the file
	dynatraceConfFile, err := parseDynatraceConfigFile([]byte(fileContent))

	if err != nil {
		logMessage := fmt.Sprintf("Couldn't parse %s file found for service %s in stage %s in project %s. Error: %s", DynatraceConfigFilename, keptnEvent.Service, keptnEvent.Stage, keptnEvent.Project, err.Error())
		logger.Error(logMessage)
		return nil, errors.New(logMessage)
	}

	logMessage := fmt.Sprintf("Loaded Config from dynatrace.conf.yaml:  %s", dynatraceConfFile)
	logger.Info(logMessage)

	return dynatraceConfFile, nil
}

func parseDynatraceConfigFile(input []byte) (*DynatraceConfigFile, error) {
	dynatraceConfFile := &DynatraceConfigFile{}
	err := yaml.Unmarshal([]byte(input), &dynatraceConfFile)

	if err != nil {
		return nil, err
	}

	return dynatraceConfFile, nil
}

/**
 * Pulls the Dynatrace Credentials from the passed secret
 */
func GetDTCredentials(dynatraceSecretName string) (*DTCredentials, error) {
	if dynatraceSecretName == "" {
		return nil, nil
	}

	dtCreds := &DTCredentials{}
	if RunLocal || RunLocalTest {
		// if we RunLocal we take it from the env-variables
		dtCreds.Tenant = os.Getenv("DT_TENANT")
		dtCreds.ApiToken = os.Getenv("DT_API_TOKEN")
		dtCreds.PaaSToken = os.Getenv("DT_PAAS_TOKEN")
	} else {
		kubeAPI, err := GetKubernetesClient()
		if err != nil {
			return nil, err
		}
		secret, err := kubeAPI.CoreV1().Secrets("keptn").Get(dynatraceSecretName, metav1.GetOptions{})

		if err != nil {
			return nil, err
		}

		if string(secret.Data["DT_TENANT"]) == "" || string(secret.Data["DT_API_TOKEN"]) == "" || string(secret.Data["DT_PAAS_TOKEN"]) == "" {
			return nil, errors.New("invalid or no Dynatrace credentials found")
		}

		dtCreds.Tenant = string(secret.Data["DT_TENANT"])
		dtCreds.ApiToken = string(secret.Data["DT_API_TOKEN"])
		dtCreds.PaaSToken = string(secret.Data["DT_PAAS_TOKEN"])
	}

	// ensure URL always has http or https in front
	if strings.HasPrefix(dtCreds.Tenant, "https://") || strings.HasPrefix(dtCreds.Tenant, "http://") {
		dtCreds.Tenant = dtCreds.Tenant
	} else {
		dtCreds.Tenant = "https://" + dtCreds.Tenant
	}

	return dtCreds, nil
}
