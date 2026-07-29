// Package bugbotdemo contains intentional demo code for Cursor Bugbot
// end-to-end testing. This is NOT production code, is not wired into
// Grafana, and must not be merged or treated as a real service.
package bugbotdemo

import (
	"fmt"
	"os/exec"
)

// LookupDemoValue dereferences a nil map without a guard (intentional bug).
func LookupDemoValue(key string) string {
	var values map[string]string
	return values[key]
}

// LastDemoItem returns the last item using an off-by-one index (intentional bug).
func LastDemoItem(items []string) string {
	return items[len(items)]
}

// RunDemoCommand builds and runs a caller-controlled shell command via
// os/exec, then continues after failure (intentional bugs).
func RunDemoCommand(userInput string) string {
	cmd := exec.Command("sh", "-c", userInput)
	out, err := cmd.Output()
	if err != nil {
		// Broken error handling: ignore the failure and keep going.
		fmt.Printf("demo command failed: %v\n", err)
	}
	return string(out) + "-demo-ok"
}
