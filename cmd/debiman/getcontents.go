package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/sync/errgroup"

	"github.com/Debian/debiman/internal/archive"
	ptarchive "pault.ag/go/archive"
	"pault.ag/go/debian/control"
)

type contentEntry struct {
	suite     string
	arch      string
	binarypkg string
	filename  string
}

var manPrefix = []byte("usr/share/man/")

func parseContentsEntry(scanner *bufio.Scanner) ([]*contentEntry, error) {
	for scanner.Scan() {
		text := scanner.Bytes()
		if !bytes.HasPrefix(text, manPrefix) {
			continue
		}

		idx := bytes.LastIndex(text, []byte{' '})
		if idx == -1 {
			continue
		}
		parts := bytes.Split(text[idx:], []byte{','})
		entries := make([]*contentEntry, 0, len(parts))
		for _, part := range parts {
			idx2 := bytes.IndexByte(part, '/')
			if idx2 == -1 {
				continue
			}
			entries = append(entries, &contentEntry{
				binarypkg: string(part[idx2+1:]),
				filename:  string(bytes.TrimSpace(text[len(manPrefix):idx])),
			})
		}
		if len(entries) > 0 {
			return entries, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return nil, io.EOF
}

func getContents(ar *archive.Getter, suite string, component string, archs []string, hashByFilename map[string]*control.SHA256FileHash) ([]*contentEntry, error) {
	files := make([]*os.File, len(archs))
	scanners := make([]*bufio.Scanner, len(archs))
	contents := make([][]*contentEntry, len(archs))
	advance := make([]bool, len(archs))
	exhausted := make([]bool, len(archs))
	var eg errgroup.Group
	for idx, arch := range archs {
		idx := idx   // copy
		arch := arch // copy
		eg.Go(func() error {
			path := component + "/Contents-" + arch + ".gz"
			fh, ok := hashByFilename[path]
			if !ok {
				return fmt.Errorf("ERROR: expected path %q not found in Release file", path)
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
			contents[idx], err = parseContentsEntry(scanners[idx])
			if err != nil {
				return err
			}
			advance[idx] = false
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
		return nil, err
	}

	var entries []*contentEntry
	for {
		for idx, move := range advance {
			if !move {
				continue
			}
			var err error
			contents[idx], err = parseContentsEntry(scanners[idx])
			if err != nil {
				if err == io.EOF {
					exhausted[idx] = true
				} else {
					return nil, err
				}
			}
		}
		// TODO: unit test for edge cases: can this loop indefinitely or can packages be skipped here?
		if done(exhausted) {
			break
		}

		// find the filename which is the least advanced in the sort order
		var lowest int
		var sum int
		for idx := range archs {
			sum += len(contents[idx])
			if exhausted[idx] {
				continue
			}
			if contents[idx][0].filename < contents[lowest][0].filename {
				lowest = idx
			}
		}

		for idx := range advance {
			advance[idx] = !exhausted[idx] && contents[lowest][0].filename == contents[idx][0].filename
		}

		binarypkgs := make(map[string]string, sum)
		for idx := range archs {
			if !advance[idx] {
				continue
			}

			for _, e := range contents[idx] {
				// first arch (amd64) wins
				if _, ok := binarypkgs[e.binarypkg]; !ok {
					binarypkgs[e.binarypkg] = archs[idx]
				}
			}
		}

		for pkg, arch := range binarypkgs {
			entries = append(entries, &contentEntry{
				binarypkg: pkg,
				arch:      arch,
				filename:  contents[lowest][0].filename,
				suite:     suite,
			})
		}
	}

	return entries, nil
}

func getAllContents(ar *archive.Getter, suite string, release *ptarchive.Release, hashByFilename map[string]*control.SHA256FileHash) ([]*contentEntry, error) {
	// We skip archAll, because there is no Contents-all file. The
	// contents of Architecture: all packages are included in the
	// architecture-specific Contents-* files.

	// TODO(later): make this code work with all components once it’s
	// confirmed that we are interested in serving more than just
	// main.
	for _, component := range []string{"main"} {
		archs := make([]string, len(release.Architectures))
		for idx, arch := range release.Architectures {
			archs[idx] = arch.String()
		}

		return getContents(ar, suite, component, archs, hashByFilename)
	}

	return nil, nil
}
