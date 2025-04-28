package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckService(t *testing.T) {
	snc := Agent{}
	res := snc.RunCheck("check_service", []string{"filter='state=running'", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Regexpf(t,
		`^OK - All \d+ service\(s\) are ok.`,
		string(res.BuildPluginOutput()),
		"output matches",
	)

	res = snc.RunCheck("check_service", []string{"service=nonexistingservice"})
	assert.Equalf(t, CheckExitUnknown, res.State, "state Unknown")
	assert.Containsf(t, string(res.BuildPluginOutput()), "The specified service does not exist as an installed service", "output matches")

	// search service by display name
	res = snc.RunCheck("check_service", []string{"service=Server", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 service", "output matches")

	// search service by non case name
	res = snc.RunCheck("check_service", []string{"service=server", "show-all"})
	assert.Equalf(t, CheckExitOK, res.State, "state OK")
	assert.Containsf(t, string(res.BuildPluginOutput()), "OK - All 1 service", "output matches")
}
