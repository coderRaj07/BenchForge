// Package canary decides whether a rollout step may advance.
package canary

import "fmt"

// Decide returns promote, pause, or rollback. The reason is the whole explanation.
func Decide(candidateP95, stableP95, candidateErr float64, samples, minSamples int, maxRegressionPct, maxError float64, steps []int, currentPct int) (string, string) {
	if minSamples < 1 {
		minSamples = 1
	}
	if samples < minSamples {
		return "pause", fmt.Sprintf("only %d samples, need %d", samples, minSamples)
	}
	if maxError > 0 && candidateErr > maxError {
		return "rollback", fmt.Sprintf("candidate error rate %.4f exceeds %.4f", candidateErr, maxError)
	}
	if stableP95 > 0 && candidateP95 > stableP95*(1+maxRegressionPct/100) {
		return "rollback", fmt.Sprintf("candidate p95 %.2fms exceeds stable %.2fms by more than %.1f%%", candidateP95, stableP95, maxRegressionPct)
	}
	next := currentPct
	for i, step := range steps {
		if step == currentPct && i+1 < len(steps) {
			next = steps[i+1]
			break
		}
	}
	if next == currentPct {
		return "promote", fmt.Sprintf("traffic %d%% meets the gate; this is the last step", currentPct)
	}
	return "promote", fmt.Sprintf("advance traffic from %d%% to %d%%", currentPct, next)
	// What: chooses pause, rollback, or promote for one canary step.
	// Used by: internal/canary/canary_test.go TestDecide. deploy/argo-rollouts/orders-rollout.yaml describes the same step list but does not call this function.
	// Logic: minSamples is raised to 1 when the caller passes a smaller number. samples compared with minSamples returns pause. candidateErr above maxError returns rollback when maxError is set. candidateP95 above stableP95 times (1 + maxRegressionPct/100) returns rollback. next starts as currentPct. The loop sets next to the following entry in steps when it finds currentPct. If next never changes, the reason says this is the last step. Otherwise the reason names the move from currentPct to next.
}
