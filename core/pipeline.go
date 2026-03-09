package core

// PipelineResult is the outcome of running all hunts.
type PipelineResult struct {
	HuntResults []HuntResult
}

// HasErrors returns true if any hunt had errors.
func (pr *PipelineResult) HasErrors() bool {
	for _, hr := range pr.HuntResults {
		if len(hr.Errors) > 0 {
			return true
		}
	}
	return false
}

// HuntResult is the outcome of running a single hunt.
type HuntResult struct {
	HuntName  string
	Scanned   int
	Evaluated int
	Notified  int
	Errors    []StepError
}

// StepError records a failure during a pipeline step.
type StepError struct {
	Step    string // "scan", "evaluate", "notify", "brief", etc.
	Err     error
	Context string // which item/group failed
}
