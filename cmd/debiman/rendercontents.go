package main

import (
	"fmt"
	"html/template"
	"io"
	"sort"
)

var contentsTmpl = template.Must(template.Must(commonTmpls.Clone()).New("contents").Parse(contentsContent))

func renderContents(dest, suite string, bins []string) error {
	sort.Strings(bins)

	return writeAtomically(dest, func(w io.Writer) error {
		return contentsTmpl.Execute(w, struct {
			Title       string
			Breadcrumbs []breadcrumb
			FooterExtra string
			Bins        []string
			Suite       string
		}{
			Title: fmt.Sprintf("Contents of Debian %s", suite),
			Breadcrumbs: []breadcrumb{
				{fmt.Sprintf("/contents-%s.html", suite), suite},
				{"", "Contents"},
			},
			Bins:  bins,
			Suite: suite,
		})
	})
}
