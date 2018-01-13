package main

import (
	"fmt"
	"html/template"
	"io"
	"sort"

	"github.com/Debian/debiman/internal/bundled"
	"github.com/Debian/debiman/internal/manpage"
	"github.com/Debian/debiman/internal/write"
)

var pkgindexTmpl = mustParsePkgindexTmpl()
var srcpkgindexTmpl = mustParseSrcPkgindexTmpl()

func mustParsePkgindexTmpl() *template.Template {
	return template.Must(template.Must(commonTmpls.Clone()).New("pkgindex").Parse(bundled.Asset("pkgindex.tmpl")))
}

func mustParseSrcPkgindexTmpl() *template.Template {
	return template.Must(template.Must(commonTmpls.Clone()).New("srcpkgindex").Parse(bundled.Asset("srcpkgindex.tmpl")))
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

	return write.Atomically(dest, true, func(w io.Writer) error {
		return pkgindexTmpl.Execute(w, struct {
			Title          string
			DebimanVersion string
			Breadcrumbs    breadcrumbs
			FooterExtra    string
			First          *manpage.Meta
			Meta           *manpage.Meta
			ManpageByName  map[string]*manpage.Meta
			Mans           []string
			HrefLangs      []*manpage.Meta
		}{
			Title:          fmt.Sprintf("Manpages of %s in Debian %s", first.Package.Binarypkg, first.Package.Suite),
			DebimanVersion: debimanVersion,
			Breadcrumbs: breadcrumbs{
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

func renderSrcPkgindex(dest string, src string, manpageByName map[string]*manpage.Meta) error {
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

	return write.Atomically(dest, true, func(w io.Writer) error {
		return srcpkgindexTmpl.Execute(w, struct {
			Title          string
			DebimanVersion string
			Breadcrumbs    breadcrumbs
			FooterExtra    string
			First          *manpage.Meta
			Meta           *manpage.Meta
			ManpageByName  map[string]*manpage.Meta
			Mans           []string
			HrefLangs      []*manpage.Meta
			Src            string
		}{
			Title:          fmt.Sprintf("Manpages of src:%s in Debian %s", src, first.Package.Suite),
			DebimanVersion: debimanVersion,
			Breadcrumbs: breadcrumbs{
				{fmt.Sprintf("/contents-%s.html", first.Package.Suite), first.Package.Suite},
				{fmt.Sprintf("/%s/%s/index.html", first.Package.Suite, first.Package.Binarypkg), first.Package.Binarypkg},
				{"", "Contents"},
			},
			First:         first,
			Meta:          first,
			ManpageByName: manpageByName,
			Mans:          mans,
			Src:           src,
		})
	})
}
