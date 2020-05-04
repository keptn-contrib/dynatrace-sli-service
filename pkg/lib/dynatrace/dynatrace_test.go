package dynatrace

import (
	"strconv"
	"testing"
	"time"

	"github.com/keptn-contrib/dynatrace-sli-service/pkg/common"
)

func TestCreateNewDynatraceHandler(t *testing.T) {
	keptnEvent := &common.BaseKeptnEvent{}
	keptnEvent.Project = "sockshop"
	keptnEvent.Stage = "dev"
	keptnEvent.Service = "carts"
	keptnEvent.DeploymentStrategy = "direct"

	dh := NewDynatraceHandler(
		"dynatrace",
		keptnEvent,
		map[string]string{
			"Authorization": "Api-Token " + "test",
		},
		nil,
	)

	if dh.ApiURL != "dynatrace" {
		t.Errorf("dh.ApiURL=%s; want dynatrace", dh.ApiURL)
	}

	if dh.KeptnEvent.Project != "sockshop" {
		t.Errorf("dh.Project=%s; want sockshop", dh.KeptnEvent.Project)
	}

	if dh.KeptnEvent.Stage != "dev" {
		t.Errorf("dh.Stage=%s; want dev", dh.KeptnEvent.Stage)
	}

	if dh.KeptnEvent.Service != "carts" {
		t.Errorf("dh.Service=%s; want carts", dh.KeptnEvent.Service)
	}
	if dh.KeptnEvent.DeploymentStrategy != "direct" {
		t.Errorf("dh.Deployment=%s; want direct", dh.KeptnEvent.DeploymentStrategy)
	}
}

// Test that unsupported metrics return an error
func TestGetTimeseriesUnsupportedSLI(t *testing.T) {
	keptnEvent := &common.BaseKeptnEvent{}
	keptnEvent.Project = "sockshop"
	keptnEvent.Stage = "dev"
	keptnEvent.Service = "carts"
	keptnEvent.DeploymentStrategy = ""

	dh := NewDynatraceHandler(
		"dynatrace",
		keptnEvent,
		map[string]string{
			"Authorization": "Api-Token " + "test",
		},
		nil,
	)

	got, err := dh.getTimeseriesConfig("foobar")

	if got != "" {
		t.Errorf("dh.getTimeseriesConfig() returned (\"%s\"), expected(\"\")", got)
	}

	expected := "unsupported SLI metric foobar"

	if err == nil {
		t.Errorf("dh.getTimeseriesConfig() did not return an error")
	} else {
		if err.Error() != expected {
			t.Errorf("dh.getTimeseriesConfig() returned error %s, expected %s", err.Error(), expected)
		}
	}
}

func TestTimestampToString(t *testing.T) {
	dt := time.Now()

	got := timestampToString(dt)

	expected := strconv.FormatInt(dt.Unix()*1000, 10)

	if got != expected {
		t.Errorf("timestampToString() returned %s, expected %s", got, expected)
	}
}

// tests the parseUnixTimestamp with invalid params
func TestParseInvalidUnixTimestamp(t *testing.T) {
	_, err := parseUnixTimestamp("")

	if err == nil {
		t.Errorf("parseUnixTimestamp(\"\") did not return an error")
	}
}

// tests the parseUnixTimestamp with valid params
func TestParseValidUnixTimestamp(t *testing.T) {
	got, err := parseUnixTimestamp("2019-10-24T15:44:27.152330783Z")

	if err != nil {
		t.Errorf("parseUnixTimestamp(\"2019-10-24T15:44:27.152330783Z\") returned error %s", err.Error())
	}

	if got.Year() != 2019 {
		t.Errorf("parseUnixTimestamp() returned year %d, expected 2019", got.Year())
	}

	if got.Month() != 10 {
		t.Errorf("parseUnixTimestamp() returned month %d, expected 10", got.Month())
	}

	if got.Day() != 24 {
		t.Errorf("parseUnixTimestamp() returned day %d, expected 24", got.Day())
	}

	if got.Hour() != 15 {
		t.Errorf("parseUnixTimestamp() returned hour %d, expected 15", got.Hour())
	}

	if got.Minute() != 44 {
		t.Errorf("parseUnixTimestamp() returned minute %d, expected 44", got.Minute())
	}
}
