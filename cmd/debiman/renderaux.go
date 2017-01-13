package main

import (
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Debian/debiman/internal/bundled"
)

var indexTmpl = template.Must(template.Must(commonTmpls.Clone()).New("index").Parse(bundled.Asset("index.tmpl")))
var faqTmpl = template.Must(template.Must(commonTmpls.Clone()).New("faq").Parse(bundled.Asset("faq.tmpl")))

type bySuiteStr []string

func (p bySuiteStr) Len() int      { return len(p) }
func (p bySuiteStr) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p bySuiteStr) Less(i, j int) bool {
	orderi, oki := sortOrder[p[i]]
	orderj, okj := sortOrder[p[j]]
	if !oki || !okj {
		panic(fmt.Sprintf("either %q or %q is an unknown suite. known: %+v", p[i], p[j], sortOrder))
	}
	return orderi < orderj
}

func renderAux(destDir string, gv globalView) error {
	suites := make([]string, 0, len(gv.suites))
	for suite := range gv.suites {
		suites = append(suites, suite)
	}
	sort.Stable(bySuiteStr(suites))

	if err := writeAtomically(filepath.Join(destDir, "index.html.gz"), true, func(w io.Writer) error {
		return indexTmpl.Execute(w, struct {
			Title          string
			DebimanVersion string
			Breadcrumbs    []breadcrumb
			FooterExtra    string
			Suites         []string
		}{
			Title:          "index",
			Suites:         suites,
			DebimanVersion: debimanVersion,
		})
	}); err != nil {
		return err
	}

	if err := writeAtomically(filepath.Join(destDir, "faq.html.gz"), true, func(w io.Writer) error {
		return faqTmpl.Execute(w, struct {
			Title          string
			DebimanVersion string
			Breadcrumbs    []breadcrumb
			FooterExtra    string
		}{
			Title:          "FAQ",
			DebimanVersion: debimanVersion,
		})
	}); err != nil {
		return err
	}

	for name, content := range bundled.AssetsFiltered(func(fn string) bool {
		return !strings.HasSuffix(fn, ".tmpl") && !strings.HasSuffix(fn, ".css")
	}) {
		if err := writeAtomically(filepath.Join(destDir, filepath.Base(name)+".gz"), true, func(w io.Writer) error {
			_, err := io.WriteString(w, content)
			return err
		}); err != nil {
			return err
		}
	}

	return nil
}
