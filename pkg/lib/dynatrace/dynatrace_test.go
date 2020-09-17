package dynatrace

import (
	"reflect"
	"strconv"
	"testing"
	"time"

	_ "github.com/keptn/go-utils/pkg/lib"
	keptn "github.com/keptn/go-utils/pkg/lib"

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

	got := common.TimestampToString(dt)

	expected := strconv.FormatInt(dt.Unix()*1000, 10)

	if got != expected {
		t.Errorf("timestampToString() returned %s, expected %s", got, expected)
	}
}

// tests the parseUnixTimestamp with invalid params
func TestParseInvalidUnixTimestamp(t *testing.T) {
	_, err := common.ParseUnixTimestamp("")

	if err == nil {
		t.Errorf("parseUnixTimestamp(\"\") did not return an error")
	}
}

// tests the parseUnixTimestamp with valid params
func TestParseValidUnixTimestamp(t *testing.T) {
	got, err := common.ParseUnixTimestamp("2019-10-24T15:44:27.152330783Z")

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

func TestParsePassAndWarningFromString(t *testing.T) {
	type args struct {
		customName string
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 []*keptn.SLOCriteria
		want2 []*keptn.SLOCriteria
		want3 int
		want4 bool
	}{
		{
			name: "simple test",
			args: args{
				customName: "Some description;sli=teststep_rt;pass=<500ms,<+10%;warning=<1000ms,<+20%;weight=1;key=true",
			},
			want:  "teststep_rt",
			want1: []*keptn.SLOCriteria{&keptn.SLOCriteria{Criteria: []string{"<500ms", "<+10%"}}},
			want2: []*keptn.SLOCriteria{&keptn.SLOCriteria{Criteria: []string{"<1000ms", "<+20%"}}},
			want3: 1,
			want4: true,
		},
		{
			name: "test with = in pass/warn expression",
			args: args{
				customName: "Host Disk Queue Length (max);sli=host_disk_queue;pass=<=0;warning=<1;key=false",
			},
			want:  "host_disk_queue",
			want1: []*keptn.SLOCriteria{&keptn.SLOCriteria{Criteria: []string{"<=0"}}},
			want2: []*keptn.SLOCriteria{&keptn.SLOCriteria{Criteria: []string{"<1"}}},
			want3: 1,
			want4: false,
		},
		{
			name: "test weight",
			args: args{
				customName: "Host CPU %;sli=host_cpu;pass=<20;warning=<50;key=false;weight=2",
			},
			want:  "host_cpu",
			want1: []*keptn.SLOCriteria{&keptn.SLOCriteria{Criteria: []string{"<20"}}},
			want2: []*keptn.SLOCriteria{&keptn.SLOCriteria{Criteria: []string{"<50"}}},
			want3: 2,
			want4: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, got3, got4 := ParsePassAndWarningFromString(tt.args.customName, []string{}, []string{})
			if got != tt.want {
				t.Errorf("ParsePassAndWarningFromString() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("ParsePassAndWarningFromString() got1 = %v, want %v", got1, tt.want1)
			}
			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("ParsePassAndWarningFromString() got2 = %v, want %v", got2, tt.want2)
			}
			if !reflect.DeepEqual(got3, tt.want3) {
				t.Errorf("ParsePassAndWarningFromString() got2 = %v, want %v", got3, tt.want3)
			}
			if !reflect.DeepEqual(got4, tt.want4) {
				t.Errorf("ParsePassAndWarningFromString() got2 = %v, want %v", got4, tt.want4)
			}
		})
	}
}
