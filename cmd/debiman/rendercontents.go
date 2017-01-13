package main

import (
	"fmt"
	"html/template"
	"io"
	"sort"

	"github.com/Debian/debiman/internal/bundled"
)

var contentsTmpl = mustParseContentsTmpl()

func mustParseContentsTmpl() *template.Template {
	return template.Must(template.Must(commonTmpls.Clone()).New("contents").Parse(bundled.Asset("contents.tmpl")))
}

func renderContents(dest, suite string, bins []string) error {
	sort.Strings(bins)

	return writeAtomically(dest, true, func(w io.Writer) error {
		return contentsTmpl.Execute(w, struct {
			Title          string
			DebimanVersion string
			Breadcrumbs    []breadcrumb
			FooterExtra    string
			Bins           []string
			Suite          string
		}{
			Title:          fmt.Sprintf("Contents of Debian %s", suite),
			DebimanVersion: debimanVersion,
			Breadcrumbs: []breadcrumb{
				{fmt.Sprintf("/contents-%s.html", suite), suite},
				{"", "Contents"},
			},
			Bins:  bins,
			Suite: suite,
		})
	})
}
