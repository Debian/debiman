package main

import (
	"compress/gzip"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/Debian/debiman/internal/manpage"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

var pkgindexTmpl = template.Must(template.New("contents").
	Funcs(map[string]interface{}{
		"DisplayLang": func(tag language.Tag) string {
			return display.Self.Name(tag)
		},
		"ShortSection": func(section string) string {
			return shortSections[section]
		},
		"LongSection": func(section string) string {
			return longSections[section]
		},
	}).
	Parse(pkgindexContent))

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

	f, err := ioutil.TempFile(filepath.Dir(dest), "debiman-")
	if err != nil {
		return err
	}
	defer f.Close()

	// TODO(later): benchmark/support other compression algorithms. zopfli gets dos2unix from 9659B to 9274B (4% win)

	// NOTE(stapelberg): gzip’s decompression phase takes the same
	// time, regardless of compression level. Hence, we invest the
	// maximum CPU time once to achieve the best compression.
	w, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		return err
	}
	defer w.Close()

	if err := pkgindexTmpl.Execute(w, struct {
		First         *manpage.Meta
		ManpageByName map[string]*manpage.Meta
		Mans          []string
	}{
		First:         first,
		ManpageByName: manpageByName,
		Mans:          mans,
	}); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}

	if err := f.Chmod(0644); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(f.Name(), dest)
}
