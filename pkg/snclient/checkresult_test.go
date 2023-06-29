package snclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckResultValueOnly(t *testing.T) {
	cr := &CheckResult{
		State:  0,
		Output: "OK - test  |  free=317MB;;;;",
	}

	expect := []*CheckMetric{{
		Name:  "free",
		Value: "317",
		Unit:  "MB",
	}}
	cr.ParsePerformanceDataFromOutput()
	assert.Equalf(t, expect, cr.Metrics, "parsed metrics")
	assert.Equalf(t, "OK - test", cr.Output, "plugin output is trimmed now")
	assert.Equalf(t, "OK - test |'free'=317MB", string(cr.BuildPluginOutput()), "plugin output")
}

func TestCheckResultWarnCritMinMax(t *testing.T) {
	cr := &CheckResult{
		State:  0,
		Output: "OK - test  |  val=5c;2;3;0;10",
	}

	zero := float64(0)
	ten := float64(10)
	twoStr := "2"
	threeStr := "3"
	expect := []*CheckMetric{{
		Name:        "val",
		Value:       "5",
		Unit:        "c",
		WarningStr:  &twoStr,
		CriticalStr: &threeStr,
		Min:         &zero,
		Max:         &ten,
	}}
	cr.ParsePerformanceDataFromOutput()
	assert.Equalf(t, expect, cr.Metrics, "parsed metrics")
	assert.Equalf(t, "OK - test", cr.Output, "plugin output is trimmed now")
	assert.Equalf(t, "OK - test |'val'=5c;2;3;0;10", string(cr.BuildPluginOutput()), "plugin output")
}

func TestCheckResultEscapedPipe(t *testing.T) {
	// escaped pipes are ignored
	output := "OK - test \\|  free=317MB;;;;"
	cr := &CheckResult{
		State:  0,
		Output: output,
	}

	expect := []*CheckMetric{}
	cr.ParsePerformanceDataFromOutput()
	assert.Equalf(t, expect, cr.Metrics, "parsed metrics")
	assert.Equalf(t, output, cr.Output, "plugin output is unchanged")
	assert.Equalf(t, output, string(cr.BuildPluginOutput()), "plugin output")
}

func TestCheckResultEscapedPipeAndUnescaped(t *testing.T) {
	// escaped pipes are ignored
	output := "OK - test \\|  free=317MB;;;; | test=9"
	cr := &CheckResult{
		State:  0,
		Output: output,
	}

	expect := []*CheckMetric{{
		Name:  "test",
		Value: "9",
		Unit:  "",
	}}
	cr.ParsePerformanceDataFromOutput()
	assert.Equalf(t, expect, cr.Metrics, "parsed metrics")
	assert.Equalf(t, "OK - test \\|  free=317MB;;;;", cr.Output, "plugin output is trimmed now")
	assert.Equalf(t, "OK - test \\|  free=317MB;;;; |'test'=9", string(cr.BuildPluginOutput()), "plugin output")
}

func TestCheckResultPerfOnly(t *testing.T) {
	cr := &CheckResult{
		State:  0,
		Output: "|  free=317MB;;;;",
	}

	expect := []*CheckMetric{{
		Name:  "free",
		Value: "317",
		Unit:  "MB",
	}}
	cr.ParsePerformanceDataFromOutput()
	assert.Equalf(t, expect, cr.Metrics, "parsed metrics")
	assert.Equalf(t, "", cr.Output, "plugin output is trimmed now")
	assert.Equalf(t, "|'free'=317MB", string(cr.BuildPluginOutput()), "plugin output")
}

func TestCheckResultMultiple(t *testing.T) {
	cr := &CheckResult{
		State:  0,
		Output: `|  free=317MB;;;; 'used bytes'=42GB;;;;  "total bytes"=11.5GB;10:20;@5:30`,
	}

	warn := "10:20"
	crit := "@5:30"
	expect := []*CheckMetric{{
		Name:  "free",
		Value: "317",
		Unit:  "MB",
	}, {
		Name:  "used bytes",
		Value: "42",
		Unit:  "GB",
	}, {
		Name:        "total bytes",
		Value:       "11.5",
		Unit:        "GB",
		WarningStr:  &warn,
		CriticalStr: &crit,
	}}
	cr.ParsePerformanceDataFromOutput()
	assert.Equalf(t, expect, cr.Metrics, "parsed metrics")
	assert.Equalf(t, "", cr.Output, "plugin output is trimmed now")
	assert.Equalf(t, `|'free'=317MB 'used bytes'=42GB 'total bytes'=11.5GB;10:20;@5:30`, string(cr.BuildPluginOutput()), "plugin output")
}
