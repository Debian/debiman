package manpage

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Debian/debiman/internal/tag"
	"golang.org/x/text/language"
	"pault.ag/go/debian/version"
)

type PkgMeta struct {
	Binarypkg string

	// Version is used by the templates when rendering.
	Version version.Version

	// Suite is the Debian suite in which this binary package was
	// found.
	Suite string
}

type Meta struct {
	// Name is e.g. “w3m”, or “systemd.service”.
	Name string

	// Package is the Debian binary package from which this manpage
	// was extracted.
	Package *PkgMeta

	// Section is the man section to which this manpage belongs,
	// e.g. 1, 3pm, …
	Section string

	// Language is the locale-like language directory
	// (i.e. language[_territory][.codeset][@modifier], with language coming
	// from ISO639 and territory coming from ISO3166) in which this
	// manpage was found.
	Language    string
	LanguageTag language.Tag
}

// FromManPath constructs a manpage, gathering details from path (relative underneath /usr/share/man).
func FromManPath(path string, p *PkgMeta) (*Meta, error) {
	// man pages are in /usr/share/man/(<lang>/|)man<section>/<name>.<section>.gz

	lang := "C"

	parts := strings.Split(path, "/")
	if len(parts) == 3 {
		// Language-specific subdirectory
		lang = parts[0]
		// Strip the codeset, if any
		if idx := strings.Index(lang, "."); idx > -1 {
			lidx := strings.LastIndex(lang, "@")
			if lidx == -1 {
				lidx = len(lang)
			}
			lang = lang[:idx] + lang[lidx:len(lang)]
		}
		parts = parts[1:]
	}

	// Both C and POSIX are to be treated as english. TODO: add reference
	if lang == "C" || lang == "POSIX" {
		lang = "en"
	}

	tag, err := tag.FromLocale(lang)
	if err != nil {
		return nil, fmt.Errorf("Cannot parse language %q: %v", lang, err)
	}

	if len(parts) != 2 {
		return nil, fmt.Errorf("Unexpected path format %q", path)
	}

	if !strings.HasSuffix(parts[1], ".gz") {
		parts[1] = parts[1] + ".gz"
	}

	section := strings.TrimPrefix(parts[0], "man")
	re := regexp.MustCompile(fmt.Sprintf(`\.%s([^.]*)\.gz$`, section))
	matches := re.FindStringSubmatch(parts[1])
	if matches == nil {
		return nil, fmt.Errorf("file name (%q) does not match regexp %v", parts[1], re)
	}
	if len(matches) > 1 {
		section = section + matches[1]
	}

	return &Meta{
		Name:        strings.TrimSuffix(parts[1], "."+section+".gz"),
		Package:     p,
		Section:     section,
		Language:    lang,
		LanguageTag: tag,
	}, nil
}

// FromServingPath constructs a manpage, gathering details from path
func FromServingPath(servingDir, path string) (*Meta, error) {
	// *servingDir/<suite>/<binarypkg>/<name>.<section>.<lang>
	relpath := strings.TrimPrefix(path, filepath.Clean(servingDir)+"/")
	pparts := strings.Split(relpath, "/")
	if len(pparts) != 3 {
		return nil, fmt.Errorf("Unexpected path format %q", relpath)
	}

	base := strings.TrimSuffix(filepath.Base(path), ".gz")
	// the first part can contain dots, so we need to “split from the right”
	// TODO: this can be implemented more efficiently
	allbparts := strings.Split(base, ".")
	if len(allbparts) < 3 {
		return nil, fmt.Errorf("Unexpected file name format %q", base)
	}
	bparts := []string{
		strings.Join(allbparts[:len(allbparts)-2], "."),
		allbparts[len(allbparts)-2],
		allbparts[len(allbparts)-1],
	}

	tag, err := tag.FromLocale(bparts[2])
	if err != nil {
		return nil, fmt.Errorf("Cannot parse language %q (in path %q): %v", bparts[2], path, err)
	}

	if len(bparts) != 3 {
		return nil, fmt.Errorf("Unexpected file name format %q", base)
	}

	return &Meta{
		Name: bparts[0],
		Package: &PkgMeta{
			Binarypkg: pparts[1],
			Suite:     pparts[0],
		},
		Section:     bparts[1],
		Language:    bparts[2],
		LanguageTag: tag,
	}, nil
}

func (m *Meta) String() string {
	return m.ServingPath()
}

func (m *Meta) ServingPath() string {
	return m.Package.Suite + "/" + m.Package.Binarypkg + "/" + m.Name + "." + m.Section + "." + m.Language
}

// RawPath returns the path to access the raw manpage equivalent of
// what is currently being served, i.e. locked to the current
// language.
func (m *Meta) RawPath() string {
	return m.Package.Suite + "/" + m.Package.Binarypkg + "/" + m.Name + "." + m.Section + "." + m.Language + ".gz"
}

func (m *Meta) PermaLink() string {
	return m.Package.Suite + "/" + m.Package.Binarypkg + "/" + m.Name + "." + m.Section
}

func (m *Meta) MainSection() string {
	return m.Section[:1]
}
