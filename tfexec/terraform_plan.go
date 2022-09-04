package tfexec

import (
	"context"
	"fmt"
	"os"
	"strings"
)

const (
	OutputChanges      = "Changes to Outputs:"
	ChangesStartString = "Terraform will perform the following actions:"
	ChangesEndString   = "Plan: "
)

// Plan computes expected changes.
// If a state is given, use it for the input state.
func (c *terraformCLI) Plan(ctx context.Context, state *State, opts ...string) (*Plan, error) {
	args := []string{"plan"}

	if state != nil {
		if hasPrefixOptions(opts, "-state=") {
			return nil, fmt.Errorf("failed to build options. The state argument (!= nil) and the -state= option cannot be set at the same time: state=%v, opts=%v", state, opts)
		}
		tmpState, err := writeTempFile(state.Bytes())
		defer os.Remove(tmpState.Name())
		if err != nil {
			return nil, err
		}
		args = append(args, "-state="+tmpState.Name())
	}

	// To return a plan file as a return value, we always use an -out option and load it to memory.
	// if the option exists just use it else create a temporary file.
	planOut := ""
	if hasPrefixOptions(opts, "-out=") {
		planOut = getOptionValue(opts, "-out=")
	} else {
		tmpPlan, err := os.CreateTemp("", "tfplan")
		if err != nil {
			return nil, fmt.Errorf("failed to create temporary plan file: %s", err)
		}
		planOut = tmpPlan.Name()
		defer os.Remove(planOut)

		if err := tmpPlan.Close(); err != nil {
			return nil, fmt.Errorf("failed to close temporary plan file: %s", err)
		}
		args = append(args, "-out="+planOut)
	}

	args = append(args, opts...)

	out, _, err := c.Run(ctx, args...)

	// terraform plan -detailed-exitcode returns 2 if there is a diff.
	// So we intentionally ignore an error of read the plan file and returns the
	// original error of terraform plan command.
	plan, _ := os.ReadFile(planOut)

	// If ignore plan output changes is set to true and terraform plan option -detailed-exitcode, skip error code 2 and return plan with nil error
	if err != nil && (c.ignoreOutputDiffs &&
		strings.Contains(out, OutputChanges) &&
		!strings.Contains(out, ChangesStartString) &&
		!strings.Contains(out, ChangesEndString)) {
		return NewPlan(plan), nil
	}

	// If ignore plan output changes is set to true and only there are changes in outputs return plan and no error
	if c.ignoreOutputDiffs &&
		strings.Contains(out, OutputChanges) &&
		!strings.Contains(out, ChangesStartString) &&
		!strings.Contains(out, ChangesEndString) {
		return NewPlan(plan), nil
	}

	return NewPlan(plan), err
}
