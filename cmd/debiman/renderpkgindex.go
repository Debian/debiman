package main

import (
	"fmt"
	"html/template"
	"io"
	"sort"

	"github.com/Debian/debiman/internal/bundled"
	"github.com/Debian/debiman/internal/manpage"
)

var pkgindexTmpl = mustParsePkgindexTmpl()

func mustParsePkgindexTmpl() *template.Template {
	return template.Must(template.Must(commonTmpls.Clone()).New("pkgindex").Parse(bundled.Asset("pkgindex.tmpl")))
}

func renderPkgindex(dest string, manpageByName map[string]*manpage.Meta) error {
	var first *manpage.Meta
	for _, m := range manpageByName {
		first = m
		break
	}

	mans := make([]string, 0, len(manpageByName))
	for n := range manpageByName {
		mans = append(mans, n)
	}
	sort.Strings(mans)

	return writeAtomically(dest, true, func(w io.Writer) error {
		return pkgindexTmpl.Execute(w, struct {
			Title          string
			DebimanVersion string
			Breadcrumbs    []breadcrumb
			FooterExtra    string
			First          *manpage.Meta
			Meta           *manpage.Meta
			ManpageByName  map[string]*manpage.Meta
			Mans           []string
		}{
			Title:          fmt.Sprintf("Manpages of %s in Debian %s", first.Package.Binarypkg, first.Package.Suite),
			DebimanVersion: debimanVersion,
			Breadcrumbs: []breadcrumb{
				{fmt.Sprintf("/contents-%s.html", first.Package.Suite), first.Package.Suite},
				{fmt.Sprintf("/%s/%s/index.html", first.Package.Suite, first.Package.Binarypkg), first.Package.Binarypkg},
				{"", "Contents"},
			},
			First:         first,
			Meta:          first,
			ManpageByName: manpageByName,
			Mans:          mans,
		})
	})
}
