package main

import (
	"compress/gzip"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

var contentsTmpl = template.Must(template.New("contents").
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
	Parse(contentsContent))

func renderContents(dest, suite string, bins []string) error {
	sort.Strings(bins)

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

	if err := contentsTmpl.Execute(w, struct {
		Bins  []string
		Suite string
	}{
		Bins:  bins,
		Suite: suite,
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
