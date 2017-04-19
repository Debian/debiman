package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/Debian/debiman/internal/archive"
	"github.com/Debian/debiman/internal/manpage"
	"pault.ag/go/debian/control"
)

// mostPopularArchitecture is used as preferred architecture when we
// need to pick an arbitrary architecture. The rationale is that
// downloading the package for the most popular architecture has the
// least bad influence on the mirror server’s caches.
const mostPopularArchitecture = "amd64"

type stats struct {
	PackagesExtracted uint64
	PackagesDeleted   uint64
	ManpagesRendered  uint64
	ManpageBytes      uint64
	HtmlBytes         uint64
	IndexBytes        uint64
}

type link struct {
	from string
	to   string
}

type globalView struct {
	pkgs          []*pkgEntry
	suites        map[string]bool
	idxSuites     map[string]string
	contentByPath map[string][]*contentEntry
	xref          map[string][]*manpage.Meta
	// alternatives maps from Debian binary package to a slice of
	// links (from→to pairs).
	alternatives map[string][]link
	stats        *stats
	start        time.Time
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

func parseAlternativesFile(fn, prefix string) (map[string][]link, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	res := make(map[string][]link)
	dec := json.NewDecoder(r)
	// read open bracket
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	for dec.More() {
		var m struct {
			Binpackage string
			From       string
			To         string
		}
		if err := dec.Decode(&m); err != nil {
			return nil, err
		}
		log.Printf("adding from %q to %q to pkg %q", m.From, m.To, m.Binpackage)
		key := prefix + "/" + m.Binpackage
		res[key] = append(res[key], link{
			from: m.From,
			to:   m.To,
		})
	}
	return res, nil
}

func parseAlternativesDir(dir string) (map[string][]link, error) {
	if dir == "" {
		return map[string][]link{}, nil
	}
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	results := make([]map[string][]link, len(infos))
	var eg errgroup.Group
	for idx, fi := range infos {
		eg.Go(func() error {
			suite := strings.TrimSuffix(fi.Name(), ".json.gz")
			res, err := parseAlternativesFile(filepath.Join(dir, fi.Name()), suite)
			results[idx] = res
			return err
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	// Merge all subresults into one map. This is non-destructive
	// because the keys are prefixed by the Debian suite, which is
	// derived from the filename and hence unique.
	merged := make(map[string][]link)
	for idx := range infos {
		for key, val := range results[idx] {
			merged[key] = val
		}
	}
	return merged, nil
}

func markPresent(latestVersion map[string]*manpage.PkgMeta, xref map[string][]*manpage.Meta, filename string, key string) error {
	if _, ok := latestVersion[key]; !ok {
		return fmt.Errorf("Could not determine latest version")
	}
	m, err := manpage.FromManPath(strings.TrimPrefix(filename, "usr/share/man/"), latestVersion[key])
	if err != nil {
		return fmt.Errorf("Trying to interpret path %q: %v", filename, err)
	}
	// NOTE(stapelberg): this additional verification step
	// is necessary because manpages such as the French
	// manpage for qelectrotech(1) are present in multiple
	// encodings. manpageFromManPath ignores encodings, so
	// if we didn’t filter, we would end up with what
	// looks like duplicates.
	present := false
	for _, x := range xref[m.Name] {
		if x.ServingPath() == m.ServingPath() {
			present = true
			break
		}
	}
	if !present {
		xref[m.Name] = append(xref[m.Name], m)
	}
	return nil
}

func buildGlobalView(ar *archive.Getter, dists []distribution, alternativesDir string, start time.Time) (globalView, error) {
	var stats stats
	res := globalView{
		suites:        make(map[string]bool, len(dists)),
		idxSuites:     make(map[string]string, len(dists)),
		contentByPath: make(map[string][]*contentEntry),
		xref:          make(map[string][]*manpage.Meta),
		stats:         &stats,
		start:         start,
	}

	var err error
	res.alternatives, err = parseAlternativesDir(alternativesDir)
	if err != nil {
		return res, err
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
		res.idxSuites[dist.name] = suite

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
			pkgs, latestVersion, err = getAllPackages(ar, suite, release, hashByFilename, buildContainsMains(content, res.alternatives))
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
			key := c.suite + "/" + c.binarypkg
			if err := markPresent(latestVersion, res.xref, c.filename, key); err != nil {
				knownIssues[key] = append(knownIssues[key], err)
			}
		}

		for key, links := range res.alternatives {
			for _, link := range links {
				log.Printf("key=%q, link=%v, latest = %v", key, link, latestVersion[key])
				if err := markPresent(latestVersion, res.xref, strings.TrimPrefix(link.from, "/"), key); err != nil {
					knownIssues[key] = append(knownIssues[key], err)
				}
			}
		}

		for key, errors := range knownIssues {
			// TODO: write these to a known-issues file, parse bug numbers from an auxilliary file
			log.Printf("package %q has errors: %v", key, errors)
		}
	}
	return res, nil
}
