package main

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/Debian/debiman/internal/bundled"
	"github.com/Debian/debiman/internal/manpage"
)

var contentsTmpl = mustParseContentsTmpl()

func mustParseContentsTmpl() *template.Template {
	return template.Must(template.Must(commonTmpls.Clone()).New("contents").Parse(bundled.Asset("contents.tmpl")))
}

func renderContents(dest, suite string, bins []string) error {
	sort.Strings(bins)

	if err := writeAtomically(dest, true, func(w io.Writer) error {
		return contentsTmpl.Execute(w, struct {
			Title          string
			DebimanVersion string
			Breadcrumbs    breadcrumbs
			FooterExtra    string
			Bins           []string
			Suite          string
			Meta           *manpage.Meta
		}{
			Title:          fmt.Sprintf("Contents of Debian %s", suite),
			DebimanVersion: debimanVersion,
			Breadcrumbs: breadcrumbs{
				{fmt.Sprintf("/contents-%s.html", suite), suite},
				{"", "Contents"},
			},
			Bins:  bins,
			Suite: suite,
		})
	}); err != nil {
		return err
	}

	destPath := filepath.Join(*servingDir, suite, "index.html.gz")
	link := fmt.Sprintf("../contents-%s.html.gz", suite)
	if err := os.Symlink(link, destPath); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}
