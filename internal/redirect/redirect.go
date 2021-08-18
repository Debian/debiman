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
	Name      string // TODO: string pool
	Suite     string // TODO: enum to save space
	Binarypkg string // TODO: sort by popcon, TODO: use a string pool
	Section   string // TODO: use a string pool
	Language  string // TODO: type: would it make sense to use language.Tag?
}

func (e IndexEntry) ServingPath(suffix string) string {
	return "/" + e.Suite + "/" + e.Binarypkg + "/" + e.Name + "." + e.Section + "." + e.Language + suffix
}

type Index struct {
	Entries  map[string][]IndexEntry
	Suites   map[string]string
	Langs    map[string]bool
	Sections map[string]bool
}

// TODO(later): the default suite should be the latest stable release
const defaultSuite = "bullseye"
const defaultLanguage = "en"

// bestLanguageMatch is like bestLanguageMatch in rendermanpage.go, but for the redirector index. TODO: can we de-duplicate the code?
func bestLanguageMatch(t []language.Tag, options []IndexEntry) IndexEntry {
	// ensure that en comes first, so that language.Matcher treats it as default
	if options[0].Language != "en" {
		for i := 1; i < len(options); i++ {
			if options[i].Language == "en" {
				options = append([]IndexEntry{options[i]}, options...)
				break
			}
		}
	}

	if t == nil {
		return options[0]
	}

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

func (i Index) split(path string) (suite string, binarypkg string, name string, section string, lang string) {
	dir := strings.TrimPrefix(filepath.Dir(path), "/")
	base := strings.TrimSpace(filepath.Base(path))
	base = strings.Replace(base, " ", ".", -1)
	parts := strings.Split(dir, "/")
	if len(parts) > 0 {
		if len(parts) == 1 {
			if _, ok := i.Suites[parts[0]]; ok {
				suite = parts[0]
			} else if i.Sections[parts[0]] {
				// legacy manpages.debian.org
				section = parts[0]
			} else {
				if i.Sections[base] {
					// man.freebsd.org
					section = base
					base = parts[0]
				} else {
					binarypkg = parts[0]
				}
			}
		} else if len(parts) == 2 && strings.HasPrefix(parts[1], "man") && i.Sections[strings.TrimPrefix(parts[1], "man")] {
			// legacy manpages.debian.org
			lang = parts[0]
			section = strings.TrimPrefix(parts[1], "man")
		} else if len(parts) == 2 {
			suite = parts[0]
			binarypkg = parts[1]
		}
	}

	// the first part can contain dots, so we need to “split from the right”
	parts = strings.Split(base, ".")
	if len(parts) == 1 {
		return suite, binarypkg, base, section, lang
	}

	// The last part can either be a language or a section
	consumed := 0
	if l := parts[len(parts)-1]; i.Langs[l] {
		lang = l
		consumed++
	} else if l := parts[len(parts)-1]; i.Sections[l] {
		section = l
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

	return suite,
		binarypkg,
		strings.Join(parts[:len(parts)-consumed], "."),
		section,
		lang
}

type byMainSection []IndexEntry

func (p byMainSection) Len() int      { return len(p) }
func (p byMainSection) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p byMainSection) Less(i, j int) bool {
	// Compare main sections first
	mi := p[i].Section[:1]
	mj := p[j].Section[:1]
	if mi < mj {
		return true
	}
	if mi > mj {
		return false
	}
	return len(p[i].Section) > len(p[j].Section)
}

// Default taken from man(1):
var mansect = searchOrder(strings.Split("1 n l 8 3 2 3posix 3pm 3perl 3am 5 4 9 6 7", " "))

func searchOrder(sections []string) map[string]int {
	order := make(map[string]int)
	for idx, section := range sections {
		order[section] = idx
	}
	return order
}

type bySection []IndexEntry

func (p bySection) Len() int      { return len(p) }
func (p bySection) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p bySection) Less(i, j int) bool {
	oI, okI := mansect[p[i].Section]
	oJ, okJ := mansect[p[j].Section]
	if okI && okJ { // both sections are in mansect
		return oI < oJ
	}
	if !okI && okJ {
		return false // sort all mansect sections before custom sections
	}
	if okI && !okJ {
		return true // sort all mansect sections before custom sections
	}
	return p[i].Section < p[j].Section // neither are in mansect
}

func (i Index) Narrow(acceptLang string, template, ref IndexEntry, entries []IndexEntry) []IndexEntry {
	t := template // for convenience

	fullyQualified := func() bool {
		if t.Suite == "" || t.Binarypkg == "" || t.Section == "" || t.Language == "" {
			return false
		}

		// Verify validity
		for _, e := range entries {
			if t.Suite == e.Suite &&
				t.Binarypkg == e.Binarypkg &&
				t.Section == e.Section &&
				t.Language == e.Language {
				return true
			}
		}
		return false
	}

	filtered := make([]IndexEntry, len(entries))
	copy(filtered, entries)

	filter := func(keep func(e IndexEntry) bool) {
		tmp := filtered[:0]
		for _, e := range filtered {
			if !keep(e) {
				continue
			}
			tmp = append(tmp, e)
		}
		filtered = tmp
	}

	if t.Language != "" {
		// Verify the specified language is a valid choice
		var found bool
		for _, e := range filtered {
			if e.Language == t.Language {
				found = true
				break
			}
		}
		if !found {
			t.Language = ""
		}
	}

	if t.Suite != "" {
		// Verify the specified suite is a valid choice
		var found bool
		for _, e := range filtered {
			if e.Suite == t.Suite {
				found = true
				break
			}
		}
		if !found {
			t.Suite = ""
		}
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
		// Prefer redirecting to the suite from the referrer
		for _, e := range filtered {
			if e.Suite == ref.Suite {
				t.Suite = ref.Suite
				break
			}
		}
		// Default to defaultSuite
		if t.Suite == "" {
			for _, e := range filtered {
				if e.Suite == defaultSuite {
					t.Suite = defaultSuite
					break
				}
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

	if len(t.Section) > 1 {
		// A subsection was specified. Sort by section, but prefer
		// subsections so that they get matched first (e.g. “3” will
		// come after “3edit”).
		sort.Stable(byMainSection(filtered))
	} else {
		// No subsection was specified. Sort by section so that
		// subsections are matched later (e.g. “3edit” will come after
		// “3”).
		sort.Stable(bySection(filtered))
	}

	if t.Section == "" {
		// TODO(later): respect the section preference cookie (+test)
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

type NotFoundError struct {
	Manpage    string
	BestChoice IndexEntry
}

func (e *NotFoundError) Error() string {
	return "No such man page"
}

func (i Index) Redirect(r *http.Request) (string, error) {
	path := r.URL.Path

	if strings.HasSuffix(path, "/") ||
		strings.HasSuffix(path, "/index.html") ||
		strings.HasPrefix(path, "/contents-") {
		return "", &NotFoundError{}
	}

	suffix := ".html"
	// If a raw manpage was requested, redirect to raw, not HTML
	if strings.HasSuffix(path, ".gz") && !strings.HasSuffix(path, ".html.gz") {
		suffix = ".gz"
	}
	for strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".gz") {
		path = strings.TrimSuffix(path, ".gz")
		path = strings.TrimSuffix(path, ".html")
	}

	// Parens are converted into dots, so that “i3(1)” becomes
	// “i3.1.”. Trailing dots are stripped and two dots next to each
	// other are converted into one.
	path = strings.Replace(path, "(", ".", -1)
	path = strings.Replace(path, ")", ".", -1)
	path = strings.Replace(path, "..", ".", -1)
	path = strings.TrimSuffix(path, ".")

	var suite, binarypkg, name, section, lang string
	if strings.HasPrefix(path, "/man") && strings.Index(path[1:], "/") > -1 {
		suite, binarypkg, name, section, lang = i.splitLegacy(path)
	} else {
		suite, binarypkg, name, section, lang = i.split(path)
	}
	if rewrite, ok := i.Suites[suite]; ok {
		suite = rewrite
	}
	if section == "0" {
		// legacy manpages.debian.org
		section = ""
	}

	log.Printf("path %q -> suite = %q, binarypkg = %q, name = %q, section = %q, lang = %q", path, suite, binarypkg, name, section, lang)

	lname := strings.ToLower(name)
	entries, ok := i.Entries[lname]
	if !ok {
		// Fall back to joining (originally) whitespace-separated
		// parts by dashes and underscores, like man(1).
		entries, ok = i.Entries[strings.Replace(lname, ".", "-", -1)]
		if !ok {
			entries, ok = i.Entries[strings.Replace(lname, ".", "_", -1)]
			if !ok {
				return "", &NotFoundError{Manpage: name}
			}
		}
	}

	acceptLang := r.Header.Get("Accept-Language")
	ref := IndexEntry{
		Suite:     r.FormValue("suite"),
		Binarypkg: r.FormValue("binarypkg"),
		Section:   r.FormValue("section"),
		Language:  r.FormValue("language"),
	}
	filtered := i.Narrow(acceptLang, IndexEntry{
		Suite:     suite,
		Binarypkg: binarypkg,
		Section:   section,
		Language:  lang,
	}, ref, entries)

	if len(filtered) == 0 {
		// Present the user with another choice for this manpage.
		var best IndexEntry
		if name != "index" && name != "favicon" {
			best = i.Narrow(acceptLang, IndexEntry{}, ref, entries)[0]
		}
		return "", &NotFoundError{
			Manpage:    name,
			BestChoice: best}
	}

	return filtered[0].ServingPath(suffix), nil
}

func IndexFromProto(path string) (Index, error) {
	index := Index{
		Langs:    make(map[string]bool),
		Sections: make(map[string]bool),
		Suites:   make(map[string]string),
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
		name := strings.ToLower(e.Name)
		index.Entries[name] = append(index.Entries[name], IndexEntry{
			Name:      e.Name,
			Suite:     e.Suite,
			Binarypkg: e.Binarypkg,
			Section:   e.Section,
			Language:  e.Language,
		})
	}
	for _, l := range idx.Language {
		index.Langs[l] = true
	}
	index.Suites = idx.Suite
	for _, l := range idx.Section {
		index.Sections[l] = true
	}
	index.Sections["0"] = true

	return index, nil
}
