package dynatrace

import (
	keptnevents "github.com/keptn/go-utils/pkg/events"
	"strconv"
	"testing"
	"time"
)

// Tests parsing custom filters array returns two empty strings
func TestParseCustomFiltersEmptyResults(t *testing.T) {
	customFilters := []*keptnevents.SLIFilter{}

	got_a, got_b := parseCustomFilters(customFilters)

	// empty slice should return two empty strings
	if got_a != "" || got_b != "" {
		t.Errorf("parseCustomFilters returned (\"%s\", \"%s\"), expected (\"\", \"\")", got_a, got_b)
	}

	// using a value with a key that is not recognized should return two empty strings
	customFilters = []*keptnevents.SLIFilter{
		{Key: "something", Value: "Something else"},
	}

	got_a, got_b = parseCustomFilters(customFilters)

	if got_a != "" || got_b != "" {
		t.Errorf("parseCustomFilters returned (\"%s\", \"%s\"), expected (\"\", \"\")", got_a, got_b)
	}
}

// Tests parsing custom filters returns the dynatrace entity name
func TestParseCustomFiltersDynatraceEntityName(t *testing.T) {

	customFilters := []*keptnevents.SLIFilter{
		{Key: "dynatraceEntityName", Value: "MyService"},
	}

	got_a, got_b := parseCustomFilters(customFilters)

	if got_a != "MyService" || got_b != "" {
		t.Errorf("parseCustomFilters returned (\"%s\", \"%s\"), expected (\"MyService\", \"\")", got_a, got_b)
	}
}

// Tests parsing custom filters returns tags
func TestParseCustomFiltersTags(t *testing.T) {

	customFilters := []*keptnevents.SLIFilter{
		{Key: "tags", Value: "tag1,tag2"},
	}

	got_a, got_b := parseCustomFilters(customFilters)

	if got_a != "" || got_b != "tag1,tag2" {
		t.Errorf("parseCustomFilters returned (\"%s\", \"%s\"), expected (\"\", \"tag1,tag2\")", got_a, got_b)
	}
}

func TestCreateNewDynatraceHandler(t *testing.T) {
	dh := NewDynatraceHandler(
		"dynatrace",
		"sockshop",
		"dev",
		"carts",
		map[string]string{
			"Authorization": "Api-Token " + "test",
		},
	)

	if dh.ApiURL != "dynatrace" {
		t.Errorf("dh.ApiURL=%s; want dynatrace", dh.ApiURL)
	}

	if dh.Project != "sockshop" {
		t.Errorf("dh.Project=%s; want sockshop", dh.Project)
	}

	if dh.Stage != "dev" {
		t.Errorf("dh.Stage=%s; want dev", dh.Stage)
	}

	if dh.Service != "carts" {
		t.Errorf("dh.Service=%s; want carts", dh.Service)
	}
}

// Test that unsupported metrics return an error
func TestGetTimeseriesUnsupportedSLI(t *testing.T) {
	dh := NewDynatraceHandler(
		"dynatrace",
		"sockshop",
		"dev",
		"carts",
		map[string]string{
			"Authorization": "Api-Token " + "test",
		},
	)

	got_a, got_b, _, err := dh.getTimeseriesConfig("foobar", time.Now(), time.Now())

	if got_a != "" || got_b != "" {
		t.Errorf("dh.getTimeseriesConfig() returned (\"%s\",\"%s\"), expected(\"\",\"\")", got_a, got_b)
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

// Tests the result of getTimeseriesConfig for Throughput
func TestGetTimeseriesThroughput(t *testing.T) {
	dh := NewDynatraceHandler(
		"dynatrace",
		"sockshop",
		"dev",
		"carts",
		map[string]string{
			"Authorization": "Api-Token " + "test",
		},
	)

	timeseries, aggregation, percentile, err := dh.getTimeseriesConfig(Throughput, time.Now(), time.Now())

	if timeseries != "com.dynatrace.builtin:service.requests" {
		t.Errorf("dh.getTimeseriesConfig() returned timeseries %s, expected com.dynatrace.builtin:service.requests", timeseries)
	}

	if aggregation != "count" {
		t.Errorf("dh.getTimeseriesConfig() returned aggregation %s, expected count", aggregation)
	}

	if percentile != 0 {
		t.Errorf("dh.getTimeseriesConfig() returned percentile %d, expected 0", percentile)
	}

	if err != nil {
		t.Errorf("dh.getTimeseriesConfig() returned an error %s", err.Error())
	}
}

func TestGetTimeseriesRequestLatencyP90(t *testing.T) {
	dh := NewDynatraceHandler(
		"dynatrace",
		"sockshop",
		"dev",
		"carts",
		map[string]string{
			"Authorization": "Api-Token " + "test",
		},
	)

	timeseries, aggregation, percentile, err := dh.getTimeseriesConfig(RequestLatencyP90, time.Now(), time.Now())

	if timeseries != "com.dynatrace.builtin:service.responsetime" {
		t.Errorf("dh.getTimeseriesConfig() returned timeseries %s, expected com.dynatrace.builtin:service.responsetime", timeseries)
	}

	if aggregation != "percentile" {
		t.Errorf("dh.getTimeseriesConfig() returned aggregation %s, expected percentile", aggregation)
	}

	if percentile != 90 {
		t.Errorf("dh.getTimeseriesConfig() returned percentile %d, expected 90", percentile)
	}

	if err != nil {
		t.Errorf("dh.getTimeseriesConfig() returned an error %s", err.Error())
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
