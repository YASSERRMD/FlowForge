// Package engine runs published workflow graphs deterministically through
// transactional durable states. Phase 08 covers the sequential core: run
// initiation, durable task claiming, the worker lifecycle and execution of
// start/action/end nodes. Conditions gain semantics in phase 09, parallel
// fork/join in phase 10, retries and cancellation in phase 11.
package engine

// RunStatus is the lifecycle state of one workflow run. The values match
// the CHECK constraint on workflow_runs.status exactly; the database spells
// the terminal cancellation state "canceled".
type RunStatus string

const (
	RunQueued    RunStatus = "queued"
	RunRunning   RunStatus = "running"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
	RunCanceled  RunStatus = "canceled"
)

// StepStatus is the lifecycle state of one node attempt. The values match
// the CHECK constraint on step_attempts.status exactly.
type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepSucceeded StepStatus = "succeeded"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
	StepCanceled  StepStatus = "canceled"
)

// runTransitions is the complete set of legal run state changes. Any
// transition not listed is a programming error and must be rejected before
// it reaches the database.
var runTransitions = map[RunStatus][]RunStatus{
	RunQueued:  {RunRunning, RunCanceled, RunFailed},
	RunRunning: {RunSucceeded, RunFailed, RunCanceled},
	// Terminal states: no outgoing transitions.
	RunSucceeded: {},
	RunFailed:    {},
	RunCanceled:  {},
}

// stepTransitions is the complete set of legal step-attempt state changes.
var stepTransitions = map[StepStatus][]StepStatus{
	StepPending: {StepRunning, StepSkipped, StepCanceled},
	StepRunning: {StepSucceeded, StepFailed, StepCanceled},
	// Terminal states.
	StepSucceeded: {},
	StepFailed:    {},
	StepSkipped:   {},
	StepCanceled:  {},
}

// CanRunTransition reports whether a run may move from one state to another.
func CanRunTransition(from, to RunStatus) bool {
	for _, next := range runTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// CanStepTransition reports whether a step attempt may move from one state
// to another.
func CanStepTransition(from, to StepStatus) bool {
	for _, next := range stepTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// IsTerminalRun reports whether the run state is final.
func IsTerminalRun(s RunStatus) bool {
	return s == RunSucceeded || s == RunFailed || s == RunCanceled
}

// IsTerminalStep reports whether the step state is final.
func IsTerminalStep(s StepStatus) bool {
	return s == StepSucceeded || s == StepFailed || s == StepSkipped || s == StepCanceled
}

// IsActiveStep reports whether the step still occupies the run: the end
// node may only complete a run once no active steps remain.
func IsActiveStep(s StepStatus) bool {
	return s == StepPending || s == StepRunning
}
