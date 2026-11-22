// Command api runs the FlowForge HTTP and WebSocket API service.
package main

import (
	"fmt"
	"os"

	"github.com/YASSERRMD/FlowForge/internal/buildinfo"
)

func main() {
	fmt.Printf("flowforge-api %s (commit %s)\n", buildinfo.Version, buildinfo.Commit)
	fmt.Fprintln(os.Stderr, "api server bootstrap: routing is added in phase 03")
}
