package main

import (
	"bytes"
	"fmt"
	"text/tabwriter"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func versionString() string {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 1, ' ', 0)
	fmt.Fprintf(w, "Version:\t%s\n", version)
	fmt.Fprintf(w, "Commit:\t%s\n", commit)
	fmt.Fprintf(w, "Built:\t%s\n", buildTime)
	_ = w.Flush()
	return buf.String()
}
