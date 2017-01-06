package main

import (
	"fmt"
	"html/template"
	"io"
	"sort"

	"github.com/Debian/debiman/internal/manpage"
)

var pkgindexTmpl = template.Must(template.Must(commonTmpls.Clone()).New("pkgindex").Parse(pkgindexContent))

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

	return writeAtomically(dest, func(w io.Writer) error {
		return pkgindexTmpl.Execute(w, struct {
			Title         string
			Breadcrumbs   []breadcrumb
			First         *manpage.Meta
			ManpageByName map[string]*manpage.Meta
			Mans          []string
		}{
			Title: fmt.Sprintf("Manpages of %s in Debian %s", first.Package.Binarypkg, first.Package.Suite),
			Breadcrumbs: []breadcrumb{
				{fmt.Sprintf("/contents-%s.html", first.Package.Suite), first.Package.Suite},
				{fmt.Sprintf("/%s/%s/index.html", first.Package.Suite, first.Package.Binarypkg), first.Package.Binarypkg},
				{"", "Contents"},
			},
			First:         first,
			ManpageByName: manpageByName,
			Mans:          mans,
		})
	})
}
