package version

import (
	"fmt"
)

var (
	// Version is the semantic version (added at compile time)
	Version string = "1.0"

	Dirty  string
	Branch string
	// Revision is the git commit id (added at compile time)
	Revision  string
	Goversion string
)

func PrintVersion() {

	fmt.Printf("buildInfo Version=%v, Gover=%v, Branch=%v, Dirty=%v\n", Version, Goversion, Branch, Dirty)
}
