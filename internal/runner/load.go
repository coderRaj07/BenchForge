package runner

import "fmt"

// ClassifyLoad decides whether the load generator itself failed to offer the
// requested schedule. Attempts dropped because every worker was busy are
// backpressure from the system under test, not generator saturation.
func ClassifyLoad(offered, completed, lateTicks, ticks int) (bool, string) {
	if ticks == 0 || offered == 0 {
		return false, ""
	}
	lag := float64(lateTicks) / float64(ticks)
	achieved := float64(completed) / float64(offered)
	if lag > 0.2 && achieved < 0.9 {
		return true, fmt.Sprintf("load generator lagged on %.0f%% of schedule ticks and completed %.0f%% of offered attempts", lag*100, achieved*100)
	}
	return false, ""
	// What: decides whether the open-loop scheduler itself fell behind.
	// Used by: internal/runner/query.go RunQueries after an open-loop window, and internal/runner/load_test.go.
	// Logic: ticks or offered of 0 returns false because there is no schedule to judge. lag is lateTicks/ticks. achieved is completed/offered. Both must be bad (lag over 20 percent and achieved under 90 percent) before the function returns true. Busy workers that drop attempts with a low lag are not saturation.
}
