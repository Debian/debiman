package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Debian/debiman/internal/bundled"
	"github.com/Debian/debiman/internal/convert"
	"github.com/Debian/debiman/internal/manpage"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

const iso8601Format = "2006-01-02T15:04:05Z"

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
	sortOrder["experimental"] = sortOrder["unstable"] + 1
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

var manpageTmpl = mustParseManpageTmpl()

func mustParseManpageTmpl() *template.Template {
	return template.Must(template.Must(commonTmpls.Clone()).New("manpage").
		Funcs(map[string]interface{}{
			"ShortSection": func(section string) string {
				return shortSections[section]
			},
			"LongSection": func(section string) string {
				return longSections[section]
			},
			"FragmentLink": func(fragment string) string {
				u := url.URL{Fragment: strings.Replace(fragment, " ", "_", -1)}
				return u.String()
			},
		}).
		Parse(bundled.Asset("manpage.tmpl")))
}

var manpageerrorTmpl = mustParseManpageerrorTmpl()

func mustParseManpageerrorTmpl() *template.Template {
	return template.Must(template.Must(commonTmpls.Clone()).New("manpage-error").
		Funcs(map[string]interface{}{
			"ShortSection": func(section string) string {
				return shortSections[section]
			},
			"LongSection": func(section string) string {
				return longSections[section]
			},
			"FragmentLink": func(fragment string) string {
				u := url.URL{Fragment: strings.Replace(fragment, " ", "_", -1)}
				return u.String()
			},
		}).
		Parse(bundled.Asset("manpageerror.tmpl")))
}

var manpagefooterextraTmpl = mustParseManpagefooterextraTmpl()

func mustParseManpagefooterextraTmpl() *template.Template {
	return template.Must(template.Must(commonTmpls.Clone()).New("manpage-footerextra").
		Funcs(map[string]interface{}{
			"Iso8601": func(t time.Time) string {
				return t.UTC().Format(iso8601Format)
			},
		}).
		Parse(bundled.Asset("manpagefooterextra.tmpl")))
}

func convertFile(converter *convert.Process, src string, resolve func(ref string) string) (doc string, toc []string, err error) {
	f, err := os.Open(src)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		if err == io.EOF {
			// TODO: better representation of an empty manpage
			return "This space intentionally left blank.", nil, nil
		}
		return "", nil, err
	}
	defer r.Close()
	out, toc, err := converter.ToHTML(r, resolve)
	if err != nil {
		return "", nil, fmt.Errorf("convert(%q): %v", src, err)
	}
	return out, toc, nil
}

type byPkgAndLanguage struct {
	opts       []*manpage.Meta
	currentpkg string
}

func (p byPkgAndLanguage) Len() int      { return len(p.opts) }
func (p byPkgAndLanguage) Swap(i, j int) { p.opts[i], p.opts[j] = p.opts[j], p.opts[i] }
func (p byPkgAndLanguage) Less(i, j int) bool {
	// prefer manpages from the same package
	if p.opts[i].Package.Binarypkg != p.opts[j].Package.Binarypkg {
		if p.opts[i].Package.Binarypkg == p.currentpkg {
			return true
		}
	}
	return p.opts[i].Language < p.opts[j].Language
}

// bestLanguageMatch returns the best manpage out of options (coming
// from current) based on text/language’s matching.
func bestLanguageMatch(current *manpage.Meta, options []*manpage.Meta) *manpage.Meta {
	sort.Stable(byPkgAndLanguage{options, current.Package.Binarypkg})

	if options[0].Language != "en" {
		for i := 1; i < len(options); i++ {
			if options[i].Language == "en" {
				options = append([]*manpage.Meta{options[i]}, options...)
				break
			}
		}
	}

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

type renderJob struct {
	dest     string
	src      string
	meta     *manpage.Meta
	versions []*manpage.Meta
	xref     map[string][]*manpage.Meta
	modTime  time.Time
	reuse    string
}

var notYetRenderedSentinel = errors.New("Not yet rendered")

type manpagePrepData struct {
	Title          string
	DebimanVersion string
	Breadcrumbs    breadcrumbs
	FooterExtra    template.HTML
	Suites         []*manpage.Meta
	Versions       []*manpage.Meta
	Sections       []*manpage.Meta
	Bins           []*manpage.Meta
	Langs          []*manpage.Meta
	HrefLangs      []*manpage.Meta
	Meta           *manpage.Meta
	TOC            []string
	Ambiguous      map[*manpage.Meta]bool
	Content        template.HTML
	Error          error
}

type bySuite []*manpage.Meta

func (p bySuite) Len() int      { return len(p) }
func (p bySuite) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p bySuite) Less(i, j int) bool {
	orderi, oki := sortOrder[p[i].Package.Suite]
	orderj, okj := sortOrder[p[j].Package.Suite]
	if !oki || !okj {
		panic(fmt.Sprintf("either %q or %q is an unknown suite. known: %+v", p[i].Package.Suite, p[j].Package.Suite, sortOrder))
	}
	return orderi < orderj
}

type byMainSection []*manpage.Meta

func (p byMainSection) Len() int           { return len(p) }
func (p byMainSection) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p byMainSection) Less(i, j int) bool { return p[i].MainSection() < p[j].MainSection() }

type byBinarypkg []*manpage.Meta

func (p byBinarypkg) Len() int           { return len(p) }
func (p byBinarypkg) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p byBinarypkg) Less(i, j int) bool { return p[i].Package.Binarypkg < p[j].Package.Binarypkg }

func rendermanpageprep(converter *convert.Process, job renderJob) (*template.Template, manpagePrepData, error) {
	meta := job.meta // for convenience
	// TODO(issue): document fundamental limitation: “other languages” is imprecise: e.g. crontab(1) — are the languages for package:systemd-cron or for package:cron?
	// TODO(later): to boost confidence in detecting cross-references, can we add to testdata the entire list of man page names from debian to have a good test?
	// TODO(later): add plain-text version

	var (
		content   string
		toc       []string
		renderErr = notYetRenderedSentinel
	)
	if job.reuse != "" {
		content, toc, renderErr = reuse(job.reuse)
		if renderErr != nil {
			log.Printf("WARNING: re-using %q failed: %v", job.reuse, renderErr)
		}
	}
	if renderErr != nil {
		content, toc, renderErr = convertFile(converter, job.src, func(ref string) string {
			idx := strings.LastIndex(ref, "(")
			if idx == -1 {
				return ""
			}
			section := ref[idx+1 : len(ref)-1]
			name := ref[:idx]
			related, ok := job.xref[name]
			if !ok {
				return ""
			}
			filtered := make([]*manpage.Meta, 0, len(related))
			for _, r := range related {
				if r.MainSection() != section {
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
	}

	log.Printf("rendering %q", job.dest)

	suites := make([]*manpage.Meta, 0, len(job.versions))
	for _, v := range job.versions {
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

	sort.Stable(bySuite(suites))

	bySection := make(map[string][]*manpage.Meta)
	for _, v := range job.versions {
		if v.Package.Suite != meta.Package.Suite {
			continue
		}
		bySection[v.Section] = append(bySection[v.Section], v)
	}
	sections := make([]*manpage.Meta, 0, len(bySection))
	for _, all := range bySection {
		sections = append(sections, bestLanguageMatch(meta, all))
	}
	sort.Stable(byMainSection(sections))

	conflicting := make(map[string]bool)
	bins := make([]*manpage.Meta, 0, len(job.versions))
	for _, v := range job.versions {
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
	sort.Stable(byBinarypkg(bins))

	ambiguous := make(map[*manpage.Meta]bool)
	byLang := make(map[string][]*manpage.Meta)
	for _, v := range job.versions {
		if v.Section != meta.Section {
			continue
		}
		if v.Package.Suite != meta.Package.Suite {
			continue
		}
		if conflicting[v.Package.Binarypkg] {
			continue
		}

		byLang[v.Language] = append(byLang[v.Language], v)
	}
	langs := make([]*manpage.Meta, 0, len(byLang))
	hrefLangs := make([]*manpage.Meta, 0, len(byLang))
	for _, all := range byLang {
		for _, e := range all {
			langs = append(langs, e)
			if len(all) > 1 {
				ambiguous[e] = true
			}
			// hreflang consists only of language and region,
			// scripts are not supported.
			if !strings.Contains(e.Language, "@") {
				hrefLangs = append(hrefLangs, e)
			}
		}
	}

	// NOTE(stapelberg): since our user interface currently is in
	// English, we use english collation rules to sort the list of
	// languages.

	// TODO(stapelberg): is english collation always the same as
	// strings.Sort (at least on the list of languages)?
	collate.New(language.English).Sort(byLanguage(langs))
	collate.New(language.English).Sort(byLanguage(hrefLangs))

	t := manpageTmpl
	title := fmt.Sprintf("%s(%s) — %s — Debian %s", meta.Name, meta.Section, meta.Package.Binarypkg, meta.Package.Suite)
	shorttitle := fmt.Sprintf("%s(%s)", meta.Name, meta.Section)
	if renderErr != nil {
		t = manpageerrorTmpl
		title = "Error: " + title
	}

	var footerExtra bytes.Buffer
	if err := manpagefooterextraTmpl.Execute(&footerExtra, struct {
		SourceFile  string
		LastUpdated time.Time
		Converted   time.Time
		Meta        *manpage.Meta
	}{
		SourceFile:  filepath.Base(job.src),
		LastUpdated: job.modTime,
		Converted:   time.Now(),
		Meta:        meta,
	}); err != nil {
		return nil, manpagePrepData{}, err
	}

	return t, manpagePrepData{
		Title:          title,
		DebimanVersion: debimanVersion,
		Breadcrumbs: breadcrumbs{
			{fmt.Sprintf("/contents-%s.html", meta.Package.Suite), meta.Package.Suite},
			{fmt.Sprintf("/%s/%s/index.html", meta.Package.Suite, meta.Package.Binarypkg), meta.Package.Binarypkg},
			{"", shorttitle},
		},
		FooterExtra: template.HTML(footerExtra.String()),
		Suites:      suites,
		Versions:    job.versions,
		Sections:    sections,
		Bins:        bins,
		Langs:       langs,
		HrefLangs:   hrefLangs,
		Meta:        meta,
		TOC:         toc,
		Ambiguous:   ambiguous,
		Content:     template.HTML(content),
		Error:       renderErr,
	}, nil
}

type countingWriter int64

func (c *countingWriter) Write(p []byte) (n int, err error) {
	*c += countingWriter(len(p))
	return len(p), nil
}

func rendermanpage(gzipw *gzip.Writer, converter *convert.Process, job renderJob) (uint64, error) {
	t, data, err := rendermanpageprep(converter, job)
	if err != nil {
		return 0, err
	}

	var written countingWriter
	if err := writeAtomicallyWithGz(job.dest, gzipw, func(w io.Writer) error {
		return t.Execute(io.MultiWriter(w, &written), data)
	}); err != nil {
		return 0, err
	}

	return uint64(written), nil
}
