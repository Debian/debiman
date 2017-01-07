package redirect

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Debian/debiman/internal/tag"
	"golang.org/x/text/language"
)

type IndexEntry struct {
	Suite     string // TODO: enum to save space
	Binarypkg string // TODO: sort by popcon, TODO: use a string pool
	Section   string // TODO: use a string pool
	Language  string // TODO: type: would it make sense to use language.Tag?
}

type Index struct {
	Entries  map[string][]IndexEntry
	Suites   map[string]bool
	Langs    map[string]bool
	Sections map[string]bool
}

// TODO(later): the default suite should be the latest stable release
const defaultSuite = "jessie"
const defaultLanguage = "en"

// bestLanguageMatch is like bestLanguageMatch in render.go, but for the redirector index. TODO: can we de-duplicate the code?
func bestLanguageMatch(t []language.Tag, options []IndexEntry) IndexEntry {
	sort.SliceStable(options, func(i, j int) bool {
		// ensure that en comes first, so that language.Matcher treats it as default
		if options[i].Language == "en" && options[j].Language != "en" {
			return true
		}
		return options[i].Language < options[j].Language
	})

	tags := make([]language.Tag, len(options))
	for idx, m := range options {
		tag, err := tag.FromLocale(m.Language)
		if err != nil {
			panic(fmt.Sprintf("Cannot get language.Tag from locale %q: %v", m.Language, err))
		}
		tags[idx] = tag
	}

	matcher := language.NewMatcher(tags)
	tag, _, _ := matcher.Match(t...)
	for idx, t := range tags {
		if t == tag {
			return options[idx]
		}
	}
	return options[0]
}

func (i Index) splitDir(path string) (suite string, binarypkg string) {
	dir := strings.TrimPrefix(filepath.Dir(path), "/")
	parts := strings.Split(dir, "/")
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		if i.Suites[parts[0]] {
			return parts[0], ""
		} else {
			return "", parts[0]
		}
	}
	return parts[0], parts[1]
}

func (i Index) splitBase(path string) (name string, section string, lang string) {
	base := filepath.Base(path)
	// the first part can contain dots, so we need to “split from the right”
	parts := strings.Split(base, ".")
	if len(parts) == 1 {
		return base, "", ""
	}

	// The last part can either be a language or a section
	consumed := 0
	if l := parts[len(parts)-1]; i.Langs[l] {
		lang = l
		consumed++
	}
	// The second to last part (if enough parts are present) can
	// be a section (because the language was already specified).
	if len(parts) > 1+consumed {
		if s := parts[len(parts)-1-consumed]; i.Sections[s] {
			section = s
			consumed++
		}
	}

	return strings.Join(parts[:len(parts)-consumed], "."),
		section,
		lang
}

func (i Index) Redirect(r *http.Request) (string, error) {
	path := r.URL.Path
	for strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".gz") {
		path = strings.TrimSuffix(path, ".html")
		path = strings.TrimSuffix(path, ".gz")
	}

	suite, binarypkg := i.splitDir(path)
	name, section, lang := i.splitBase(path)

	fullyQualified := func() bool {
		return suite != "" && binarypkg != "" && section != "" && lang != ""
	}
	concat := func() string {
		return "/" + suite + "/" + binarypkg + "/" + name + "." + section + "." + lang + ".html"
	}

	log.Printf("path %q -> suite = %q, binarypkg = %q, name = %q, section = %q, lang = %q", path, suite, binarypkg, name, section, lang)

	if fullyQualified() {
		return concat(), nil
	}

	if suite == "" {
		suite = defaultSuite
	}

	if fullyQualified() {
		return concat(), nil
	}

	entries, ok := i.Entries[name]
	if !ok {
		// TODO: this should result in a good 404 page.
		return "", fmt.Errorf("No such man page: name=%q", name)
	}

	if fullyQualified() {
		return concat(), nil
	}

	// TODO: use pointers
	filtered := make([]IndexEntry, 0, len(entries))
	for _, e := range entries {
		if binarypkg != "" && e.Binarypkg != binarypkg {
			continue
		}
		if e.Suite != suite {
			continue
		}
		if section != "" && e.Section != section {
			continue
		}
		filtered = append(filtered, e)
	}

	if len(filtered) == 0 {
		return "", fmt.Errorf("No such manpage found")
	}

	if section == "" {
		// TODO(later): respect the section preference cookie (+test)
		sort.SliceStable(filtered, func(i, j int) bool {
			return filtered[i].Section < filtered[j].Section
		})
		section = filtered[0].Section
	}

	if fullyQualified() {
		return concat(), nil
	}

	if lang == "" {
		lfiltered := make([]IndexEntry, 0, len(filtered))
		for _, f := range filtered {
			if f.Section != section {
				continue
			}
			lfiltered = append(lfiltered, f)
		}

		t, _, _ := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))
		// ignore err: t == nil results in the default language
		best := bestLanguageMatch(t, lfiltered)
		lang = best.Language
		if binarypkg == "" {
			binarypkg = best.Binarypkg
		}
	}

	if binarypkg == "" {
		binarypkg = filtered[0].Binarypkg
	}

	return concat(), nil
}
