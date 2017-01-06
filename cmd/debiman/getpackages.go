package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"strconv"

	"github.com/Debian/debiman/internal/archive"
	"golang.org/x/sync/errgroup"
	ptarchive "pault.ag/go/archive"
	"pault.ag/go/debian/control"
	"pault.ag/go/debian/dependency"
	"pault.ag/go/debian/version"
)

type pkgEntry struct {
	suite     string
	binarypkg string
	arch      string
	filename  string
	version   version.Version
	sha256    []byte
	bytes     int64
}

func getPackages(ar *archive.Getter, suite string, arch string, path string, hashByFilename map[string]*control.SHA256FileHash, containsMans map[string]map[string]bool, pkgChan chan<- pkgEntry) error {
	fh, ok := hashByFilename[path]
	if !ok {
		return fmt.Errorf("ERROR: expected path %q not found in Release file", path)
	}

	h, err := hex.DecodeString(fh.Hash)
	if err != nil {
		return err
	}

	log.Printf("getting %q (hash %v)", path, hex.EncodeToString(h))
	r, err := ar.Get("dists/"+suite+"/"+path, h)
	if err != nil {
		return err
	}

	var pkgs []control.BinaryIndex
	decoder, err := control.NewDecoder(r, nil)
	if err != nil {
		return err
	}
	if err := decoder.Decode(&pkgs); err != nil {
		return err
	}

	// Explicitly close r to free memory ASAP.
	r.Close()

	for _, p := range pkgs {
		if !containsMans[p.Package][arch] {
			continue
		}

		i, err := strconv.ParseInt(p.Size, 0, 64)
		if err != nil {
			return err
		}
		h, err := hex.DecodeString(p.SHA256)
		if err != nil {
			return err
		}
		pkgChan <- pkgEntry{
			binarypkg: p.Package,
			version:   p.Version,
			filename:  p.Filename,
			arch:      arch,
			sha256:    h,
			bytes:     i,
			suite:     suite,
		}
	}
	return nil
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
func buildContainsMains(content []contentEntry) map[string]map[string]bool {
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

func getAllPackages(ar *archive.Getter, suite string, release *ptarchive.Release, hashByFilename map[string]*control.SHA256FileHash, containsMans map[string]map[string]bool) ([]pkgEntry, error) {
	pkgChan := make(chan pkgEntry, 10) // 10 is an arbitrary buffer size to reduce goroutine switches
	complete := make(chan bool)
	var pkgs []pkgEntry
	go func() {
		for entry := range pkgChan {
			pkgs = append(pkgs, entry)
		}
		complete <- true
	}()

	archAll, err := dependency.ParseArch("all")
	if err != nil {
		return nil, err
	}

	var wg errgroup.Group
	for _, arch := range append(release.Architectures, *archAll) {
		a := arch.String()
		for _, component := range []string{"main"} {
			path := component + "/binary-" + a + "/Packages.gz"
			wg.Go(func() error {
				return getPackages(ar, suite, a, path, hashByFilename, containsMans, pkgChan)
			})
		}
	}
	if err := wg.Wait(); err != nil {
		return nil, err
	}
	close(pkgChan)
	<-complete
	return pkgs, nil
}
