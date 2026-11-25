package engine

import "testing"

func TestRunTransitions(t *testing.T) {
	allowed := [][2]RunStatus{
		{RunQueued, RunRunning},
		{RunQueued, RunCanceled},
		{RunQueued, RunFailed},
		{RunRunning, RunSucceeded},
		{RunRunning, RunFailed},
		{RunRunning, RunCanceled},
	}
	for _, tr := range allowed {
		if !CanRunTransition(tr[0], tr[1]) {
			t.Errorf("expected %s -> %s to be allowed", tr[0], tr[1])
		}
	}

	rejected := [][2]RunStatus{
		{RunQueued, RunQueued},
		{RunQueued, RunSucceeded},
		{RunRunning, RunQueued},
		{RunRunning, RunRunning},
		{RunSucceeded, RunRunning},
		{RunSucceeded, RunFailed},
		{RunFailed, RunRunning},
		{RunCanceled, RunRunning},
		{RunCanceled, RunSucceeded},
	}
	for _, tr := range rejected {
		if CanRunTransition(tr[0], tr[1]) {
			t.Errorf("expected %s -> %s to be rejected", tr[0], tr[1])
		}
	}
}

func TestStepTransitions(t *testing.T) {
	allowed := [][2]StepStatus{
		{StepPending, StepRunning},
		{StepPending, StepSkipped},
		{StepPending, StepCanceled},
		{StepRunning, StepSucceeded},
		{StepRunning, StepFailed},
		{StepRunning, StepCanceled},
	}
	for _, tr := range allowed {
		if !CanStepTransition(tr[0], tr[1]) {
			t.Errorf("expected %s -> %s to be allowed", tr[0], tr[1])
		}
	}

	rejected := [][2]StepStatus{
		{StepPending, StepSucceeded},
		{StepPending, StepFailed},
		{StepRunning, StepPending},
		{StepRunning, StepSkipped},
		{StepSucceeded, StepRunning},
		{StepFailed, StepPending},
		{StepSkipped, StepRunning},
		{StepCanceled, StepRunning},
	}
	for _, tr := range rejected {
		if CanStepTransition(tr[0], tr[1]) {
			t.Errorf("expected %s -> %s to be rejected", tr[0], tr[1])
		}
	}
}

func TestTerminalAndActiveClassification(t *testing.T) {
	if !IsTerminalRun(RunSucceeded) || !IsTerminalRun(RunFailed) || !IsTerminalRun(RunCanceled) {
		t.Error("all three end states must be terminal")
	}
	if IsTerminalRun(RunQueued) || IsTerminalRun(RunRunning) {
		t.Error("queued/running must not be terminal")
	}
	if !IsTerminalStep(StepSucceeded) || !IsTerminalStep(StepSkipped) ||
		!IsTerminalStep(StepFailed) || !IsTerminalStep(StepCanceled) {
		t.Error("all four step end states must be terminal")
	}
	if !IsActiveStep(StepPending) || !IsActiveStep(StepRunning) {
		t.Error("pending/running must be active")
	}
	if IsActiveStep(StepSucceeded) || IsActiveStep(StepFailed) ||
		IsActiveStep(StepSkipped) || IsActiveStep(StepCanceled) {
		t.Error("terminal steps must not be active")
	}
}

// The status strings are part of the database contract; a typo here breaks
// every CHECK-constrained write.
func TestStatusesMatchDatabaseEnums(t *testing.T) {
	runs := []RunStatus{RunQueued, RunRunning, RunSucceeded, RunFailed, RunCanceled}
	wantRuns := []string{"queued", "running", "succeeded", "failed", "canceled"}
	for i, s := range runs {
		if string(s) != wantRuns[i] {
			t.Errorf("run status %d = %q, want %q", i, s, wantRuns[i])
		}
	}
	steps := []StepStatus{StepPending, StepRunning, StepSucceeded, StepFailed, StepSkipped, StepCanceled}
	wantSteps := []string{"pending", "running", "succeeded", "failed", "skipped", "canceled"}
	for i, s := range steps {
		if string(s) != wantSteps[i] {
			t.Errorf("step status %d = %q, want %q", i, s, wantSteps[i])
		}
	}
}
