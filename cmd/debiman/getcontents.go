package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/Debian/debiman/internal/archive"
	"golang.org/x/sync/errgroup"
	ptarchive "pault.ag/go/archive"
	"pault.ag/go/debian/control"
)

type contentEntry struct {
	suite     string
	arch      string
	binarypkg string
	filename  string
}

func getContents(ar *archive.Getter, suite string, arch string, path string, hashByFilename map[string]*control.SHA256FileHash, contentChan chan<- contentEntry) error {
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
	defer r.Close()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		// TODO: strip the usr/share/man prefix to save memory
		if !strings.HasPrefix(text, "usr/share/man/") {
			continue
		}
		idx := strings.LastIndex(text, " ")
		if idx == -1 {
			continue
		}
		parts := strings.Split(text[idx:], ",")
		for _, part := range parts {
			nparts := strings.Split(part, "/")
			if len(nparts) != 2 {
				continue
			}

			contentChan <- contentEntry{
				suite:     suite,
				arch:      arch,
				binarypkg: nparts[1],
				filename:  strings.TrimSpace(text[:idx]),
			}
		}
	}
	return scanner.Err()
}

func getAllContents(ar *archive.Getter, suite string, release *ptarchive.Release, hashByFilename map[string]*control.SHA256FileHash) ([]contentEntry, error) {
	contentChan := make(chan contentEntry, 10) // 10 is arbitrary to reduce goroutine switches
	complete := make(chan bool)
	var content []contentEntry
	go func() {
		for entry := range contentChan {
			content = append(content, entry)
		}
		complete <- true
	}()
	var wg errgroup.Group
	// We skip archAll, because there is no Contents-all file. The
	// contents of Architecture: all packages are included in the
	// architecture-specific Contents-* files.
	for _, arch := range release.Architectures {
		a := arch.String()
		for _, component := range []string{"main"} {
			path := component + "/Contents-" + a + ".gz"
			wg.Go(func() error {
				return getContents(ar, suite, a, path, hashByFilename, contentChan)
			})
		}
	}

	if err := wg.Wait(); err != nil {
		return nil, err
	}
	close(contentChan)
	<-complete
	return content, nil
}
