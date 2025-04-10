package snclient

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/pdh"
)

func init() {
	AvailableChecks["check_pdh"] = CheckEntry{"check_pdh", NewCheckPDH}
	AvailableChecks["CheckCounter"] = CheckEntry{"check_pdh", NewCheckPDH}
}

type CheckPDH struct {
	CounterPath          string
	HostName             string
	Type                 string
	Instances            bool
	ExpandIndex          bool
	EnglishFallBackNames bool
	OptionalAlias        string
}

func NewCheckPDH() CheckHandler {
	return &CheckPDH{}
}

func (c *CheckPDH) Build() *CheckData {
	return &CheckData{
		implemented:  Windows,
		name:         "check_pdh",
		description:  "Checks pdh paths and handles WildCard expansion. Also available with the alias CheckCounter",
		detailSyntax: "%(name)",
		okSyntax:     "%(status) - All %(count) counter values are ok",
		topSyntax:    "%(status) - %(problem_count)/%(count) counter (%(count)) %(problem_list)",
		emptySyntax:  "%(status) - No counter found",
		emptyState:   CheckExitUnknown,
		args: map[string]CheckArgument{
			"counter":      {value: &c.CounterPath, description: "The fully qualified Counter Name"},
			"Counter":      {value: &c.CounterPath, description: "The fully qualified Counter Name"},
			"host":         {value: &c.HostName, description: "The Name Of the Host Mashine in Network where the Counter should be searched, defults to local mashine"},
			"expand-index": {value: &c.ExpandIndex, description: "Should Indices be translated?"},
			"instances":    {value: &c.Instances, description: "Expand WildCards And Fethch all instances"},
			"type":         {value: &c.Type, description: "this can be large or float depending what you expect, default is large "},
			"english":      {value: &c.EnglishFallBackNames, description: "Using English Names Regardless of system Language requires Windows Vista or higher"},
		},
		result: &CheckResult{
			State: CheckExitOK,
		},
		attributes: []CheckAttribute{
			{name: "count ", description: "Number of items matching the filter. Common option for all checks."},
			{name: "value ", description: "The counter value (either float or int)"},
		},
		exampleDefault: `
		check_pdh "counter=foo" "warn=value > 80" "crit=value > 90"
		Everything looks good
		'foo value'=18;80;90
		`,
		exampleArgs:     `counter=\\System\\System Up Time" "warn=value > 5" "crit=value > 9999`,
		argsPassthrough: true,
	}
}

// Check implements CheckHandler.
func (c *CheckPDH) Check(_ context.Context, _ *Agent, check *CheckData, args []Argument) (*CheckResult, error) {
	// If the counterpath is empty we need to parse the argument ourself for the optional alias case counter:alias=...
	if c.CounterPath == "" {
		err := c.parseCheckSpecificArgs(args)
		if err != nil {
			return nil, err
		}
	}
	var possiblePaths []string
	var hQuery pdh.PDH_HQUERY
	// Open Query  - Data Source = 0 => Real Time Datasource
	ret := pdh.PdhOpenQuery(uintptr(0), uintptr(0), &hQuery)
	defer pdh.PdhCloseQuery(hQuery)

	if ret != pdh.ERROR_SUCCESS {
		return nil, fmt.Errorf("could not open query, something is wrong with the countername")
	}

	tmpPath := c.CounterPath
	if c.EnglishFallBackNames {
		var hCounter pdh.PDH_HCOUNTER
		ret = pdh.PdhAddEnglishCounter(hQuery, tmpPath, 0, &hCounter)
		if ret != pdh.ERROR_SUCCESS {
			return nil, fmt.Errorf("cannot use provided counter path as english fallback path, api response: %d", ret)
		}
		tpm, err := pdh.PdhGetCounterInfo(hCounter, false)
		if err != nil {
			return nil, fmt.Errorf("cannot use provided counter path as english fallback path, error: %s", err.Error())
		}
		tmpPath = tpm
	}

	// If HostName is set it needs to be part of the counter path
	if c.HostName != "" {
		tmpPath = `\\` + c.HostName + `\` + c.CounterPath
	}

	// Find Indices and replace with Performance Name
	r := regexp.MustCompile(`\d+`)
	matches := r.FindAllString(c.CounterPath, -1)
	for _, match := range matches {
		index, err := strconv.Atoi(strings.ReplaceAll(match, `\`, ""))
		if err != nil {
			return nil, fmt.Errorf("could not convert index. error was %s", err.Error())
		}
		res, path := pdh.PdhLookupPerfNameByIndex(uint32(index)) //nolint:gosec // Index is small and needs  to be uint32 for system call
		if res != pdh.ERROR_SUCCESS {
			return nil, fmt.Errorf("could not find given index: %d response code: %d", index, res)
		}
		tmpPath = strings.Replace(tmpPath, match, path, 1)
	}

	// Expand Counter Path That Ends with WildCard *
	if c.Instances && strings.HasSuffix(tmpPath, "*") {
		res, paths := pdh.PdhExpandCounterPath("", tmpPath, 0)
		if res != pdh.ERROR_SUCCESS {
			return nil, fmt.Errorf("something went wrong when expanding the counter path api call returned %d", res)
		}
		possiblePaths = append(possiblePaths, paths...)
	} else {
		possiblePaths = append(possiblePaths, tmpPath)
	}

	counters, err := c.addAllPathToCounter(hQuery, possiblePaths)
	if err != nil {
		return nil, fmt.Errorf("could not add all counter path to query, error: %s", err.Error())
	}

	// Collect Values For All Counters and save values in check.listData
	err = c.collectValuesForAllCounters(hQuery, counters, check)
	if err != nil {
		return nil, fmt.Errorf("could not get values for all counter path, error: %s", err.Error())
	}

	return check.Finalize()
}

func (c *CheckPDH) parseCheckSpecificArgs(args []Argument) error {
	carg := args[0]
	parts := strings.Split(carg.key, ":")
	if len(parts) < 2 {
		return fmt.Errorf("No counter defined")
	}
	counterKey := parts[0]
	alias := parts[1]

	if !strings.EqualFold(counterKey, "counter") {
		return fmt.Errorf("Something went wrong with you syntax")
	}
	c.OptionalAlias = alias
	c.CounterPath = carg.value

	return nil
}

func (c *CheckPDH) collectValuesForAllCounters(hQuery pdh.PDH_HQUERY, counters map[string]pdh.PDH_HCOUNTER, check *CheckData) error {
	for counterPath, hCounter := range counters {
		var resArr [1]pdh.PDH_FMT_COUNTERVALUE_ITEM_LARGE // Need at least one nil pointer

		largeArr, ret := collectLargeValuesArray(hCounter, hQuery, resArr)
		if ret != pdh.ERROR_SUCCESS && ret != pdh.PDH_MORE_DATA && ret != pdh.PDH_NO_MORE_DATA {
			return fmt.Errorf("could not collect formatted value %v", ret)
		}
		entry := map[string]string{}
		for _, fmtValue := range largeArr {
			var name string
			if c.OptionalAlias != "" {
				name = c.OptionalAlias
			} else {
				name = strings.Replace(counterPath, "*", utf16PtrToString(fmtValue.SzName), 1)
			}
			entry["name"] = name
			entry["value"] = fmt.Sprintf("%d", fmtValue.FmtValue.LargeValue)
			if check.showAll {
				check.result.Metrics = append(check.result.Metrics,
					&CheckMetric{
						Name:          name,
						ThresholdName: "value",
						Value:         fmtValue.FmtValue.LargeValue,
						Warning:       check.warnThreshold,
						Critical:      check.critThreshold,
						Min:           &Zero,
					})
			}
			if check.MatchMapCondition(check.filter, entry, true) {
				check.listData = append(check.listData, entry)
			}
		}
	}

	return nil
}
