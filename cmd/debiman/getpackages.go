package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/Debian/debiman/internal/archive"
	"github.com/Debian/debiman/internal/manpage"
	ptarchive "pault.ag/go/archive"
	"pault.ag/go/debian/control"
	"pault.ag/go/debian/version"
)

type pkgEntry struct {
	source    string // only used during getPackages
	suite     string
	binarypkg string
	arch      string
	filename  string
	version   version.Version
	sha256    []byte
	bytes     int64
}

// TODO(later): containsMans could be a map[string]bool, if only all
// Debian packages would ship their manpages in all
// architectures. Example of a package which is doing it wrong:
// “inventor-clients”, which only contains manpages in i386.
//
// In theory, /usr/share must contain the same files across
// architectures: the file-system hierarchy standard (FHS) specifies
// that /usr/share is reserved for architecture independent files, see
// http://www.pathname.com/fhs/pub/fhs-2.3.html#USRSHAREARCHITECTUREINDEPENDENTDATA
// TODO(later): find out which packages are affected and file bugs
func buildContainsMains(content []*contentEntry) map[string]map[string]bool {
	containsMans := make(map[string]map[string]bool)
	for _, entry := range content {
		if _, ok := containsMans[entry.binarypkg]; !ok {
			containsMans[entry.binarypkg] = make(map[string]bool)
		}
		containsMans[entry.binarypkg][entry.arch] = true
	}
	log.Printf("%d content entries, %d packages\n", len(content), len(containsMans))
	return containsMans
}

var emptyVersion version.Version
var (
	prefixPackage  = []byte("Package")
	prefixSource   = []byte("Source")
	prefixVersion  = []byte("Version")
	prefixFilename = []byte("Filename")
	prefixSize     = []byte("Size")
	prefixSHA256   = []byte("SHA256")
)

func parsePackageParagraph(scanner *bufio.Scanner, arch string, containsMans map[string]map[string]bool) (pkgEntry, error) {
	var entry pkgEntry
	for scanner.Scan() {
		text := scanner.Bytes()
		if len(text) == 0 {
			entry = pkgEntry{}
			continue
		}
		idx := bytes.IndexByte(text, ':')
		if idx == -1 {
			continue
		}

		key := text[:idx]
		if bytes.Equal(key, prefixPackage) {
			entry.binarypkg = string(text[idx+2:])
		} else if bytes.Equal(key, prefixSource) {
			entry.source = string(text[idx+2:])
		} else if bytes.Equal(key, prefixVersion) {
			v, err := version.Parse(string(text[idx+2:]))
			if err != nil {
				return entry, err
			}
			entry.version = v
		} else if bytes.Equal(key, prefixFilename) {
			entry.filename = string(text[idx+2:])
		} else if bytes.Equal(key, prefixSize) {
			i, err := strconv.ParseInt(string(text[idx+2:]), 0, 64)
			if err != nil {
				return entry, err
			}
			entry.bytes = i
		} else if bytes.Equal(key, prefixSHA256) {
			h := make([]byte, hex.DecodedLen(len(text[idx+2:])))
			n, err := hex.Decode(h, text[idx+2:])
			if err != nil {
				return entry, err
			}
			entry.sha256 = h[:n]
		}

		if entry.binarypkg != "" &&
			entry.version != emptyVersion &&
			entry.filename != "" &&
			entry.bytes > 0 &&
			entry.sha256 != nil {
			if !containsMans[entry.binarypkg][arch] {
				entry = pkgEntry{}
				continue
			}
			if entry.source == "" {
				entry.source = entry.binarypkg
			}
			idx := strings.Index(entry.source, " ")
			if idx > -1 {
				entry.source = entry.source[:idx]
			}
			return entry, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return entry, err
	}
	entry = pkgEntry{}
	return entry, io.EOF
}

func less(a, b pkgEntry) bool {
	if a.source == b.source {
		return a.binarypkg < b.binarypkg
	}
	return a.source < b.source
}

func done(exhausted []bool) bool {
	for idx := range exhausted {
		if !exhausted[idx] {
			return false
		}
	}
	return true
}

func getPackages(ar *archive.Getter, suite string, component string, archs []string, hashByFilename map[string]*control.SHA256FileHash, containsMans map[string]map[string]bool) ([]*pkgEntry, map[string]*manpage.PkgMeta, error) {
	files := make([]*os.File, len(archs))
	scanners := make([]*bufio.Scanner, len(archs))
	pkgs := make([]pkgEntry, len(archs))
	advance := make([]bool, len(archs))
	exhausted := make([]bool, len(archs))
	var eg errgroup.Group
	for idx, arch := range archs {
		idx := idx   // copy
		arch := arch // copy
		eg.Go(func() error {
			// Prefer gzip over xz because gzip uncompresses faster.
			path := component + "/binary-" + arch + "/Packages.gz"
			fh, ok := hashByFilename[path]
			if !ok {
				path = component + "/binary-" + arch + "/Packages.xz"
				fh, ok = hashByFilename[path]
				if !ok {
					return fmt.Errorf("ERROR: expected path %q not found in Release file", path)
				}
			}

			h, err := hex.DecodeString(fh.Hash)
			if err != nil {
				return err
			}

			log.Printf("getting %q (hash %v)", suite+"/"+path, fh.Hash)
			r, err := ar.Get("dists/"+suite+"/"+path, h)
			if err != nil {
				return err
			}

			files[idx] = r
			scanners[idx] = bufio.NewScanner(r)
			advance[idx] = true
			return nil
		})
	}
	defer func() {
		for _, f := range files {
			if f != nil {
				f.Close()
			}
		}
	}()
	if err := eg.Wait(); err != nil {
		return nil, nil, err
	}

	byVersion := make(map[string]*pkgEntry)
	for {
		for idx, move := range advance {
			if !move {
				continue
			}
			arch := archs[idx]
			p, err := parsePackageParagraph(scanners[idx], arch, containsMans)
			if err != nil {
				if err == io.EOF {
					exhausted[idx] = true
				} else {
					return nil, nil, err
				}
			}
			p.arch = arch
			p.suite = suite
			pkgs[idx] = p
		}
		// TODO: unit test for edge cases: can this loop indefinitely or can packages be skipped here?
		if done(exhausted) {
			break
		}

		// find the package which is the least advanced in the sort order
		lowest := -1
		for idx := range archs {
			if exhausted[idx] {
				continue
			}
			if lowest == -1 || less(pkgs[idx], pkgs[lowest]) {
				lowest = idx
			}
		}

		for idx := range advance {
			advance[idx] = !exhausted[idx] && !less(pkgs[lowest], pkgs[idx])
		}

		// find the best architecture for that package
		var newest *pkgEntry
		for idx := range archs {
			if exhausted[idx] {
				continue
			}
			if less(pkgs[lowest], pkgs[idx]) {
				continue
			}
			if newest == nil || version.Compare(pkgs[idx].version, newest.version) > 0 {
				newest = &(pkgs[idx])
			}
		}

		key := suite + "/" + newest.binarypkg
		if v, ok := byVersion[key]; ok && version.Compare(v.version, newest.version) > 0 {
			continue
		}

		var best *pkgEntry
		for idx, p := range pkgs {
			if exhausted[idx] {
				continue
			}
			if less(pkgs[lowest], pkgs[idx]) {
				continue
			}
			if p.version != newest.version {
				continue
			}
			if p.arch == mostPopularArchitecture {
				best = &(pkgs[idx])
				break
			}
		}
		if best == nil {
			for idx, p := range pkgs {
				if exhausted[idx] {
					continue
				}
				if less(pkgs[lowest], pkgs[idx]) {
					continue
				}
				if p.version != newest.version {
					continue
				}
				best = &(pkgs[idx])
				break
			}
		}

		entry := *best // copy
		byVersion[key] = &entry
	}

	result := make([]*pkgEntry, 0, len(byVersion))
	latestVersion := make(map[string]*manpage.PkgMeta, len(byVersion))
	for key, p := range byVersion {
		result = append(result, p)
		latestVersion[key] = &manpage.PkgMeta{
			Binarypkg: p.binarypkg,
			Suite:     p.suite,
			Version:   p.version,
		}
	}

	return result, latestVersion, nil
}

func getAllPackages(ar *archive.Getter, suite string, release *ptarchive.Release, hashByFilename map[string]*control.SHA256FileHash, containsMans map[string]map[string]bool) ([]*pkgEntry, map[string]*manpage.PkgMeta, error) {
	var components = [...]string{"main", "contrib"}
	partsp := make([][]*pkgEntry, len(components))
	partsl := make([]map[string]*manpage.PkgMeta, len(components))
	latestVersion := make(map[string]*manpage.PkgMeta)
	var sum int
	for idx, component := range components {
		archs := make([]string, len(release.Architectures))
		for idx, arch := range release.Architectures {
			archs[idx] = arch.String()
		}
		partp, partl, err := getPackages(ar, suite, component, archs, hashByFilename, containsMans)
		if err != nil {
			return nil, nil, err
		}
		partsp[idx] = partp
		partsl[idx] = partl
		sum += len(partp)
	}

	results := make([]*pkgEntry, 0, sum)
	for idx := range partsp {
		results = append(results, partsp[idx]...)
		for key, value := range partsl[idx] {
			latestVersion[key] = value
		}
	}

	return results, latestVersion, nil
}
