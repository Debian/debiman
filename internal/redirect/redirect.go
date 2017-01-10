package redirect

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	pb "github.com/Debian/debiman/internal/proto"
	"github.com/Debian/debiman/internal/tag"
	"github.com/golang/protobuf/proto"
	"golang.org/x/text/language"
)

type IndexEntry struct {
	Suite     string // TODO: enum to save space
	Binarypkg string // TODO: sort by popcon, TODO: use a string pool
	Section   string // TODO: use a string pool
	Language  string // TODO: type: would it make sense to use language.Tag?
}

func (e IndexEntry) ServingPath(name string) string {
	return "/" + e.Suite + "/" + e.Binarypkg + "/" + name + "." + e.Section + "." + e.Language + ".html"
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

// bestLanguageMatch is like bestLanguageMatch in rendermanpage.go, but for the redirector index. TODO: can we de-duplicate the code?
func bestLanguageMatch(t []language.Tag, options []IndexEntry) IndexEntry {
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].Language == options[j].Language {
			return options[i].Binarypkg < options[j].Binarypkg
		}
		// ensure that en comes first, so that language.Matcher treats it as default
		if options[i].Language == "en" {
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

func (i Index) narrow(name, acceptLang string, template IndexEntry, entries []IndexEntry) []IndexEntry {
	t := template // for convenience
	valid := make(map[string]bool, len(entries))
	for _, e := range entries {
		valid[e.Suite+"/"+e.Binarypkg+"/"+name+"/"+e.Section+"/"+e.Language] = true
	}

	fullyQualified := func() bool {
		if t.Suite == "" || t.Binarypkg == "" || t.Section == "" || t.Language == "" {
			return false
		}
		return valid[t.Suite+"/"+t.Binarypkg+"/"+name+"/"+t.Section+"/"+t.Language]
	}

	// TODO: use pointers
	filtered := entries[:]

	filter := func(keep func(e IndexEntry) bool) {
		tmp := make([]IndexEntry, 0, len(filtered))
		for _, e := range filtered {
			if !keep(e) {
				continue
			}
			tmp = append(tmp, e)
		}
		filtered = tmp
	}

	// Narrow down as much as possible upfront. The keep callback is
	// the logical and of all the keep callbacks below:
	filter(func(e IndexEntry) bool {
		return (t.Suite == "" || e.Suite == t.Suite) &&
			(t.Section == "" || e.Section[:1] == t.Section[:1]) &&
			(t.Language == "" || e.Language == t.Language) &&
			(t.Binarypkg == "" || e.Binarypkg == t.Binarypkg)
	})

	// suite

	if t.Suite == "" {
		// Prefer redirecting to defaultSuite
		for _, e := range filtered {
			if e.Suite == defaultSuite {
				t.Suite = defaultSuite
				break
			}
		}
		// If the manpage is not contained in defaultSuite, use the
		// first suite we can find for which the manpage is available.
		if t.Suite == "" {
			for _, e := range filtered {
				t.Suite = e.Suite
				break
			}
		}
	}

	filter(func(e IndexEntry) bool { return t.Suite == "" || e.Suite == t.Suite })
	if len(filtered) == 0 {
		return nil
	}
	if fullyQualified() {
		return filtered
	}

	// section

	if t.Section == "" {
		// TODO(later): respect the section preference cookie (+test)
		sort.SliceStable(filtered, func(i, j int) bool {
			return filtered[i].Section < filtered[j].Section
		})
		t.Section = filtered[0].Section
	}

	filter(func(e IndexEntry) bool { return t.Section == "" || e.Section[:1] == t.Section[:1] })
	if len(filtered) == 0 {
		return nil
	}
	if fullyQualified() {
		return filtered
	}

	// language

	if t.Language == "" {
		tags, _, _ := language.ParseAcceptLanguage(acceptLang)
		// ignore err: tags == nil results in the default language
		best := bestLanguageMatch(tags, filtered)
		t.Language = best.Language
	}

	filter(func(e IndexEntry) bool { return t.Language == "" || e.Language == t.Language })
	if len(filtered) == 0 {
		return nil
	}
	if fullyQualified() {
		return filtered
	}

	// binarypkg

	if t.Binarypkg == "" {
		t.Binarypkg = filtered[0].Binarypkg
	}

	filter(func(e IndexEntry) bool { return t.Binarypkg == "" || e.Binarypkg == t.Binarypkg })
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func (i Index) Redirect(r *http.Request) (string, error) {
	path := r.URL.Path
	for strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".gz") {
		path = strings.TrimSuffix(path, ".html")
		path = strings.TrimSuffix(path, ".gz")
	}

	// Parens are converted into dots, so that “i3(1)” becomes
	// “i3.1.”. Trailing dots are stripped and two dots next to each
	// other are converted into one.
	path = strings.Replace(path, "(", ".", -1)
	path = strings.Replace(path, ")", ".", -1)
	path = strings.Replace(path, "..", ".", -1)
	path = strings.TrimSuffix(path, ".")

	suite, binarypkg := i.splitDir(path)
	name, section, lang := i.splitBase(path)

	log.Printf("path %q -> suite = %q, binarypkg = %q, name = %q, section = %q, lang = %q", path, suite, binarypkg, name, section, lang)

	entries, ok := i.Entries[name]
	if !ok {
		// TODO: this should result in a good 404 page.
		return "", fmt.Errorf("No such man page: name=%q", name)
	}

	acceptLang := r.Header.Get("Accept-Language")
	filtered := i.narrow(name, acceptLang, IndexEntry{
		Suite:     suite,
		Binarypkg: binarypkg,
		Section:   section,
		Language:  lang,
	}, entries)

	if len(filtered) == 0 {
		return "", fmt.Errorf("No such manpage found")
	}

	return filtered[0].ServingPath(name), nil
}

func IndexFromProto(path string) (Index, error) {
	index := Index{
		Langs:    make(map[string]bool),
		Sections: make(map[string]bool),
		Suites: map[string]bool{
			"testing":  true,
			"unstable": true,
			"sid":      true,
		},
	}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return index, err
	}
	var idx pb.Index
	if err := proto.Unmarshal(b, &idx); err != nil {
		return index, err
	}
	index.Entries = make(map[string][]IndexEntry, len(idx.Entry))
	for _, e := range idx.Entry {
		index.Entries[e.Name] = append(index.Entries[e.Name], IndexEntry{
			Suite:     e.Suite,
			Binarypkg: e.Binarypkg,
			Section:   e.Section,
			Language:  e.Language,
		})
	}
	for _, l := range idx.Language {
		index.Langs[l] = true
	}
	for _, l := range idx.Suite {
		index.Suites[l] = true
	}
	for _, l := range idx.Section {
		index.Sections[l] = true
	}

	return index, nil
}
