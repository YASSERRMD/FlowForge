// Command worker runs the FlowForge workflow execution worker.
package main

import (
	"fmt"
	"os"

	"github.com/YASSERRMD/FlowForge/internal/buildinfo"
)

func main() {
	fmt.Printf("flowforge-worker %s (commit %s)\n", buildinfo.Version, buildinfo.Commit)
	fmt.Fprintln(os.Stderr, "worker bootstrap: task claiming is added in phase 08")
}
