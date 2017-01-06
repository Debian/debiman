package main

import (
	"compress/gzip"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/Debian/debiman/internal/convert"
	"github.com/Debian/debiman/internal/manpage"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// TODO(later): move this list to a package within pault.ag/debian/?
var releaseList = []string{
	"buzz",
	"rex",
	"bo",
	"hamm",
	"slink",
	"potato",
	"woody",
	"sarge",
	"etch",
	"lenny",
	"squeeze",
	"wheezy",
	"wheezy-backports",
	"jessie",
	"jessie-backports",
	"stretch",
	"stretch-backports",
	"buster",
	"buster-backports",
	"bullseye",
	"bullseye-backports",
}
var sortOrder = make(map[string]int)

func init() {
	for idx, r := range releaseList {
		sortOrder[r] = idx
	}
	sortOrder["testing"] = sortOrder["stretch"]
	sortOrder["unstable"] = len(releaseList)
}

// stapelberg came up with the following abbreviations:
var shortSections = map[string]string{
	"1": "progs",
	"2": "syscalls",
	"3": "libfuncs",
	"4": "files",
	"5": "formats",
	"6": "games",
	"7": "misc",
	"8": "sysadmin",
	"9": "kernel",
}

// taken from man(1)
var longSections = map[string]string{
	"1": "Executable programs or shell commands",
	"2": "System calls (functions provided by the kernel)",
	"3": "Library calls (functions within program libraries)",
	"4": "Special files (usually found in /dev)",
	"5": "File formats and conventions eg /etc/passwd",
	"6": "Games",
	"7": "Miscellaneous (including macro packages and conventions), e.g. man(7), groff(7)",
	"8": "System administration commands (usually only for root)",
	"9": "Kernel routines [Non standard]",
}

var manpageTmpl = template.Must(template.Must(commonTmpls.Clone()).New("manpage").
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
	Parse(manpageContent))

var manpageerrorTmpl = template.Must(template.Must(commonTmpls.Clone()).New("manpage-error").
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
	Parse(manpageerrorContent))

func convertFile(src string, resolve func(ref string) string) (string, error) {
	f, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		if err == io.EOF {
			// TODO: better representation of an empty manpage
			return "This space intentionally left blank.", nil
		}
		return "", err
	}
	defer r.Close()
	out, err := convert.ToHTML(r, resolve)
	if err != nil {
		return "", fmt.Errorf("convert(%q): %v", src, err)
	}
	return out, nil
}

// bestLanguageMatch returns the best manpage out of options (coming
// from current) based on text/language’s matching.
func bestLanguageMatch(current *manpage.Meta, options []*manpage.Meta) *manpage.Meta {
	sort.SliceStable(options, func(i, j int) bool {
		// prefer manpages from the same package
		if options[i].Package.Binarypkg != options[j].Package.Binarypkg {
			if options[i].Package.Binarypkg == current.Package.Binarypkg {
				return true
			}
		}
		// ensure that en comes first, so that language.Matcher treats it as default
		if options[i].Language == "en" && options[j].Language != "en" {
			return true
		}
		return options[i].Language < options[j].Language
	})

	tags := make([]language.Tag, len(options))
	for idx, m := range options {
		tags[idx] = m.LanguageTag
	}

	// NOTE(stapelberg): it would be even better to match on the
	// user’s Accept-Language HTTP header here, but that is
	// incompatible with the processing model of pre-generating
	// all manpages.

	// TODO(stapelberg): to fix the above, we could have
	// client-side javascript which queries the redirector and
	// improves cross-references.

	matcher := language.NewMatcher(tags)
	tag, _, _ := matcher.Match(current.LanguageTag)
	for idx, t := range tags {
		if t == tag {
			return options[idx]
		}
	}
	// unreached
	return nil
}

type byLanguage []*manpage.Meta

func (p byLanguage) Len() int {
	return len(p)
}

func (p byLanguage) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p byLanguage) Bytes(i int) []byte {
	return []byte(p[i].Language)
}

func rendermanpage(dest, src string, meta *manpage.Meta, versions []*manpage.Meta, xref map[string][]*manpage.Meta) error {
	// TODO(issue): document fundamental limitation: “other languages” is imprecise: e.g. crontab(1) — are the languages for package:systemd-cron or for package:cron?
	// TODO(later): to boost confidence in detecting cross-references, can we add to testdata the entire list of man page names from debian to have a good test?
	// TODO(later): add plain-text version
	content, renderErr := convertFile(src, func(ref string) string {
		idx := strings.LastIndex(ref, "(")
		if idx == -1 {
			return ""
		}
		section := ref[idx+1 : len(ref)-1]
		name := ref[:idx]
		related, ok := xref[name]
		if !ok {
			return ""
		}
		filtered := make([]*manpage.Meta, 0, len(related))
		for _, r := range related {
			if r.Section != section {
				continue
			}
			if r.Package.Suite != meta.Package.Suite {
				continue
			}
			filtered = append(filtered, r)
		}
		if len(filtered) == 0 {
			return ""
		}
		return "/" + bestLanguageMatch(meta, filtered).ServingPath() + ".html"
	})
	log.Printf("rendering %q", dest)

	suites := make([]*manpage.Meta, 0, len(versions))
	for _, v := range versions {
		if v.Package.Binarypkg != meta.Package.Binarypkg {
			continue
		}
		if v.Section != meta.Section {
			continue
		}
		// TODO(later): allow switching to a different suite even if
		// switching requires a language-change. we should indicate
		// this in the UI.
		if v.Language != meta.Language {
			continue
		}
		suites = append(suites, v)
	}

	sort.SliceStable(suites, func(i, j int) bool {
		orderi, oki := sortOrder[suites[i].Package.Suite]
		orderj, okj := sortOrder[suites[j].Package.Suite]
		if !oki || !okj {
			panic(fmt.Sprintf("either %q or %q is an unknown suite. known: %+v", suites[i].Package.Suite, suites[j].Package.Suite, sortOrder))
		}
		return orderi < orderj
	})

	bySection := make(map[string][]*manpage.Meta)
	for _, v := range versions {
		if v.Package.Suite != meta.Package.Suite {
			continue
		}
		bySection[v.Section] = append(bySection[v.Section], v)
	}
	sections := make([]*manpage.Meta, 0, len(bySection))
	for _, all := range bySection {
		sections = append(sections, bestLanguageMatch(meta, all))
	}
	sort.SliceStable(sections, func(i, j int) bool {
		return sections[i].Section < sections[j].Section
	})

	conflicting := make(map[string]bool)
	bins := make([]*manpage.Meta, 0, len(versions))
	for _, v := range versions {
		if v.Section != meta.Section {
			continue
		}

		if v.Package.Suite != meta.Package.Suite {
			continue
		}

		// We require a strict match for the language when determining
		// conflicting packages, because otherwise the packages might
		// be augmenting, not conflicting: crontab(1) is present in
		// cron, but its translations are shipped e.g. in
		// manpages-fr-extra.
		if v.Language != meta.Language {
			continue
		}

		if v.Package.Binarypkg != meta.Package.Binarypkg {
			conflicting[v.Package.Binarypkg] = true
		}
		bins = append(bins, v)
	}
	sort.SliceStable(bins, func(i, j int) bool {
		return bins[i].Package.Binarypkg < bins[j].Package.Binarypkg
	})

	langs := make([]*manpage.Meta, 0, len(versions))
	for _, v := range versions {
		if v.Section != meta.Section {
			continue
		}
		if v.Package.Suite != meta.Package.Suite {
			continue
		}
		if conflicting[v.Package.Binarypkg] {
			continue
		}
		langs = append(langs, v)
	}

	// NOTE(stapelberg): since our user interface currently is in
	// English, we use english collation rules to sort the list of
	// languages.

	// TODO(stapelberg): is english collation always the same as
	// strings.Sort (at least on the list of languages)?
	collate.New(language.English).Sort(byLanguage(langs))

	t := manpageTmpl
	title := fmt.Sprintf("%s(%s)", meta.Name, meta.Section)
	if renderErr != nil {
		t = manpageerrorTmpl
		title = "Error: " + title
	}

	return writeAtomically(dest, func(w io.Writer) error {
		return t.Execute(w, struct {
			Title       string
			Breadcrumbs []breadcrumb
			Suites      []*manpage.Meta
			Versions    []*manpage.Meta
			Sections    []*manpage.Meta
			Bins        []*manpage.Meta
			Langs       []*manpage.Meta
			Meta        *manpage.Meta
			Content     template.HTML
			Error       error
		}{
			Title: title,
			Breadcrumbs: []breadcrumb{
				{fmt.Sprintf("/contents-%s.html", meta.Package.Suite), meta.Package.Suite},
				{fmt.Sprintf("/%s/%s/index.html", meta.Package.Suite, meta.Package.Binarypkg), meta.Package.Binarypkg},
				{"", title},
			},
			Suites:   suites,
			Versions: versions,
			Sections: sections,
			Bins:     bins,
			Langs:    langs,
			Meta:     meta,
			Content:  template.HTML(content),
			Error:    renderErr,
		})
	})
}
