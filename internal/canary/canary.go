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
}
