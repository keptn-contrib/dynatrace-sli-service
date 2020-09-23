package dynatrace

import (
	"os"
	"strconv"
)

func GetFetchSloSliFromDashboardConfig() bool {
	return readEnvAsBool("FETCH_SLO_SLI_FROM_DASHBOARD")
}

func readEnvAsBool(env string) bool {
	if b, err := strconv.ParseBool(os.Getenv(env)); err == nil {
		return b
	}
	return false
}
