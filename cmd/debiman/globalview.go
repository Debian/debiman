package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/Debian/debiman/internal/archive"
	"github.com/Debian/debiman/internal/manpage"
	"pault.ag/go/debian/control"
	"pault.ag/go/debian/version"
)

// mostPopularArchitecture is used as preferred architecture when we
// need to pick an arbitrary architecture. The rationale is that
// downloading the package for the most popular architecture has the
// least bad influence on the mirror server’s caches.
const mostPopularArchitecture = "amd64"

type globalView struct {
	pkgs          []pkgEntry
	suites        map[string]bool
	idxSuites     map[string]string
	contentByPath map[string][]*contentEntry
	xref          map[string][]*manpage.Meta
}

func dedupContent(content []*contentEntry) []*contentEntry {
	byArch := make(map[string][]*contentEntry, len(content))
	for _, c := range content {
		key := c.suite + "/" + c.binarypkg + "/" + c.filename
		byArch[key] = append(byArch[key], c)
	}

	dedup := make([]*contentEntry, 0, len(byArch))
	for _, variants := range byArch {
		var best *contentEntry
		for _, v := range variants {
			if v.arch == mostPopularArchitecture {
				best = v
				break
			}
		}
		if best == nil {
			best = variants[0]
		}

		dedup = append(dedup, best)
	}
	return dedup
}

func dedupPackages(pkgs []pkgEntry) (dedup []pkgEntry, latestVersion map[string]*manpage.PkgMeta) {
	latestVersion = make(map[string]*manpage.PkgMeta)
	log.Printf("%d package entries before architecture de-duplication", len(pkgs))
	bestArch := make(map[string][]*pkgEntry, len(pkgs))
	for idx, p := range pkgs {
		key := p.suite + "/" + p.binarypkg
		bestArch[key] = append(bestArch[key], &(pkgs[idx]))
	}
	dedup = make([]pkgEntry, 0, len(bestArch))
	for key, variants := range bestArch {
		var newest version.Version
		for _, v := range variants {
			if newest.Version == "" || version.Compare(v.version, newest) > 0 {
				newest = v.version
			}
		}
		latestVersion[key] = &manpage.PkgMeta{
			Binarypkg: variants[0].binarypkg,
			Suite:     variants[0].suite,
			Version:   newest,
		}
		var best *pkgEntry
		for _, v := range variants {
			if v.version != newest {
				continue
			}
			if v.arch == mostPopularArchitecture {
				best = v
				break
			}
		}
		if best == nil {
			for _, v := range variants {
				if v.version != newest {
					continue
				}
				best = variants[0]
				break
			}
		}
		dedup = append(dedup, *best)
	}
	log.Printf("%d packages\n", len(dedup))
	return dedup, latestVersion
}

type distributionIdentifier int

const (
	fromCodename = iota
	fromSuite
)

type distribution struct {
	name       string
	identifier distributionIdentifier
}

// distributions returns a list of all distributions (either codenames
// [e.g. wheezy, jessie] or suites [e.g. testing, unstable]) from the
// -sync_codenames and -sync_suites flags.
func distributions(codenames []string, suites []string) []distribution {
	distributions := make([]distribution, 0, len(codenames)+len(suites))
	for _, e := range codenames {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		distributions = append(distributions, distribution{
			name:       e,
			identifier: fromCodename})
	}
	for _, e := range suites {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		distributions = append(distributions, distribution{
			name:       e,
			identifier: fromSuite})
	}
	return distributions
}

func buildGlobalView(ar *archive.Getter, dists []distribution) (globalView, error) {
	res := globalView{
		suites:        make(map[string]bool, len(dists)),
		idxSuites:     make(map[string]string, len(dists)),
		contentByPath: make(map[string][]*contentEntry),
		xref:          make(map[string][]*manpage.Meta),
	}

	for _, dist := range dists {
		release, err := ar.GetRelease(dist.name)
		if err != nil {
			return res, err
		}

		var suite string
		if dist.identifier == fromCodename {
			suite = release.Codename
		} else {
			suite = release.Suite
		}

		res.suites[suite] = true
		res.idxSuites[release.Suite] = suite
		res.idxSuites[release.Codename] = suite

		{
			hashByFilename := make(map[string]*control.SHA256FileHash, len(release.SHA256))
			for idx, fh := range release.SHA256 {
				// fh.Filename contains e.g. “non-free/source/Sources”
				hashByFilename[fh.Filename] = &(release.SHA256[idx])
			}

			content, err := getAllContents(ar, suite, release, hashByFilename)
			if err != nil {
				return res, err
			}
			content = dedupContent(content)

			for _, c := range content {
				res.contentByPath[c.filename] = append(res.contentByPath[c.filename], c)
			}

			latestVersion := make(map[string]*manpage.PkgMeta)
			{
				// Collect package download work units
				pkgs, err := getAllPackages(ar, suite, release, hashByFilename, buildContainsMains(content))
				if err != nil {
					return res, err
				}

				pkgs, latestVersion = dedupPackages(pkgs)
				for _, d := range pkgs {
					res.pkgs = append(res.pkgs, d)
				}
			}

			knownIssues := make(map[string][]error)

			// Build a global view of all the manpages (required for cross-referencing).
			// TODO(issue): edge case: packages which got renamed between releases
			for _, c := range content {
				if _, ok := latestVersion[c.suite+"/"+c.binarypkg]; !ok {
					key := c.suite + "/" + c.binarypkg
					knownIssues[key] = append(knownIssues[key],
						fmt.Errorf("Could not determine latest version"))
					continue
				}
				m, err := manpage.FromManPath(strings.TrimPrefix(c.filename, "usr/share/man/"), latestVersion[c.suite+"/"+c.binarypkg])
				if err != nil {
					key := c.suite + "/" + c.binarypkg
					knownIssues[key] = append(knownIssues[key],
						fmt.Errorf("Trying to interpret path %q: ", c.filename, err))
					continue
				}
				// NOTE(stapelberg): this additional verification step
				// is necessary because manpages such as the French
				// manpage for qelectrotech(1) are present in multiple
				// encodings. manpageFromManPath ignores encodings, so
				// if we didn’t filter, we would end up with what
				// looks like duplicates.
				present := false
				for _, x := range res.xref[m.Name] {
					if x.ServingPath() == m.ServingPath() {
						present = true
						break
					}
				}
				if !present {
					res.xref[m.Name] = append(res.xref[m.Name], m)
				}
			}

			for key, errors := range knownIssues {
				// TODO: write these to a known-issues file, parse bug numbers from an auxilliary file
				log.Printf("package %q has errors: %v", key, errors)
			}
		}
	}
	return res, nil
}
