package main

import (
	"html/template"
	"io"
	"time"
)

const metricsTmplContent = `
# HELP packages_total The total number of Debian binary packages processed.
# TYPE packages_total gauge
packages_total {{ .Packages }}

# HELP packages_extracted Number of Debian binary packages from which manpages were extracted.
# TYPE packages_extracted gauge
packages_extracted {{ .Stats.PackagesExtracted }}

# HELP packages_deleted Number of Debian binary packages deleted because they were no longer present.
# TYPE packages_deleted gauge
packages_deleted {{ .Stats.PackagesDeleted }}

# HELP manpages_rendered Number of manpages rendered to HTML
# TYPE manpages_rendered gauge
manpages_rendered {{ .Stats.ManpagesRendered }}

# HELP manpage_bytes Total number of bytes used by manpages (by format).
# TYPE manpage_bytes gauge
manpage_bytes{format="man"} {{ .Stats.ManpageBytes }}
manpage_bytes{format="html"} {{ .Stats.HtmlBytes }}

# HELP index_bytes Total number of bytes used for the auxserver index.
# TYPE index_bytes gauge
index_bytes {{ .Stats.IndexBytes }}

# HELP runtime Wall-clock runtime in seconds.
# TYPE runtime gauge
runtime {{ .Seconds }}

# HELP last_successful_run Last successful run in seconds since the epoch.
# TYPE last_successful_run gauge
last_successful_run {{ .LastSuccessfulRun }}
`

var metricsTmpl = template.Must(template.New("metrics").Parse(metricsTmplContent))

func writeMetrics(w io.Writer, gv globalView, start time.Time) error {
	now := time.Now()
	return metricsTmpl.Execute(w, struct {
		Packages          int
		Stats             *stats
		Now               time.Time
		Seconds           int
		LastSuccessfulRun int64
	}{
		Packages:          len(gv.pkgs),
		Stats:             gv.stats,
		Now:               now,
		Seconds:           int(now.Sub(start).Seconds()),
		LastSuccessfulRun: now.Unix(),
	})
}
