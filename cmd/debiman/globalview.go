package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/Debian/debiman/internal/archive"
	"github.com/Debian/debiman/internal/manpage"
	"pault.ag/go/debian/control"
)

// mostPopularArchitecture is used as preferred architecture when we
// need to pick an arbitrary architecture. The rationale is that
// downloading the package for the most popular architecture has the
// least bad influence on the mirror server’s caches.
const mostPopularArchitecture = "amd64"

type globalView struct {
	pkgs          []*pkgEntry
	suites        map[string]bool
	idxSuites     map[string]string
	contentByPath map[string][]*contentEntry
	xref          map[string][]*manpage.Meta
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

			for _, c := range content {
				res.contentByPath[c.filename] = append(res.contentByPath[c.filename], c)
			}

			var latestVersion map[string]*manpage.PkgMeta
			{
				// Collect package download work units
				var pkgs []*pkgEntry
				var err error
				pkgs, latestVersion, err = getAllPackages(ar, suite, release, hashByFilename, buildContainsMains(content))
				if err != nil {
					return res, err
				}

				log.Printf("Adding %d packages from suite %q", len(pkgs), suite)
				res.pkgs = append(res.pkgs, pkgs...)
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
