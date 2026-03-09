package powder

import (
	"github.com/seanmeyer/opportunity-hunter/core"
)

// GroupForNotify groups evaluations by storm group for threaded notifications.
func (h *PowderHunt) GroupForNotify(evals []core.Evaluation) []core.NotifyGroup {
	groups := make(map[string][]core.Evaluation)
	for _, eval := range evals {
		// Group by group_key which maps to storm group.
		groups[eval.GroupKey] = append(groups[eval.GroupKey], eval)
	}

	var result []core.NotifyGroup
	for key, groupEvals := range groups {
		result = append(result, core.NotifyGroup{
			Key:         key,
			Evaluations: groupEvals,
		})
	}
	return result
}
