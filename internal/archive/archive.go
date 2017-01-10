package archive

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/openpgp"
	"pault.ag/go/archive"
)

type pool struct {
	ch chan bool
}

// newPool constructs a pool which can be used by up to n workers at
// the same time.
func newPool(n int) *pool {
	return &pool{
		ch: make(chan bool, n),
	}
}

func (p *pool) lock() {
	p.ch <- true
}

func (p *pool) unlock() {
	<-p.ch
}

type Getter struct {
	ConnectionsPerMirror int
	RetriesTransient     int
	Mirrors              []string
	LocalMirror          string

	once    sync.Once
	pool    *pool
	keyring openpgp.EntityList

	byHash   map[string]bool
	byHashMu sync.RWMutex
}

type transientError struct {
	error
}

func (g *Getter) byHashFor(suite string) bool {
	g.byHashMu.RLock()
	defer g.byHashMu.RUnlock()
	return g.byHash[suite]
}

func (g *Getter) maybeByHashPath(path string, sha256sum []byte) string {
	if !strings.HasPrefix(path, "dists/") {
		return path
	}

	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return path
	}
	if !g.byHashFor(parts[1]) {
		return path
	}

	return filepath.Dir(path) + "/by-hash/SHA256/" + hex.EncodeToString(sha256sum)
}

// download stores the contents of the Debian archive’s file
// identified by path in f, provided its SHA256 sum matches
// sha256sum. download returns transientError if the caller should
// retry.
func (g *Getter) download(path string, f *os.File, sha256sum []byte) error {
	byHash := g.maybeByHashPath(path, sha256sum)

	var r io.Reader
	if g.LocalMirror != "" {
		f, err := os.Open(filepath.Join(g.LocalMirror, byHash))
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	} else {
		resp, err := http.Get("http://deb.debian.org/debian/" + byHash)
		if err != nil {
			return transientError{err}
		}
		defer func() {
			ioutil.ReadAll(resp.Body)
			resp.Body.Close()
		}()
		if got, want := resp.StatusCode, http.StatusOK; got != want {
			err := fmt.Errorf("download(%q): Unexpected HTTP status code: got %d, want %d", path, got, want)
			if resp.StatusCode < 400 || resp.StatusCode >= 500 {
				return transientError{err}
			}
			return err
		}
		r = resp.Body
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	h := sha256.New()
	rd := io.Reader(io.TeeReader(r, h))
	var err error
	if strings.HasSuffix(path, ".gz") {
		rd, err = gzip.NewReader(rd)
		if err != nil {
			return err
		}
	}

	w := bufio.NewWriter(f)

	if _, err := io.Copy(w, rd); err != nil {
		return err
	}
	if strings.HasSuffix(path, ".gz") {
		if err := rd.(*gzip.Reader).Close(); err != nil {
			return err
		}
	}

	if err := w.Flush(); err != nil {
		return err
	}

	if got, want := h.Sum(nil), sha256sum; !bytes.Equal(got, want) {
		return fmt.Errorf("%q: invalid hash: got %v, want %v", path, hex.EncodeToString(got), hex.EncodeToString(want))
	}

	return nil
}

func (g *Getter) Get(path string, sha256sum []byte) (*os.File, error) {
	if err := g.init(); err != nil {
		return nil, err
	}
	g.pool.lock()
	defer g.pool.unlock()

	// TODO: how does this fail on linux < 3.11 or other OSes?
	f, err := os.OpenFile("/tmp", 0x410000|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	// TODO: fallback
	// f, err := ioutil.TempFile("", "archive-")
	// if err != nil {
	// 	return nil, err
	// }
	// // Remove the file system entry, we make do with the file descriptor from here on.
	// os.Remove(f.Name())

	for retry := 0; retry < 3; retry++ {
		err := g.download(path, f, sha256sum)
		if err == nil {
			break
		}
		if t, ok := err.(transientError); ok {
			log.Printf("transient error %v, retrying (attempt %d of %d)", t, retry, 3)
			continue
		}
		if err != nil {
			return nil, err
		}
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	return f, nil
}

func (g *Getter) init() error {
	var err error
	g.once.Do(func() {
		g.pool = newPool(g.ConnectionsPerMirror)
		err = g.loadArchiveKeyrings()
		g.byHash = make(map[string]bool)
	})
	return err
}

// loadArchiveKeyrings loads the debian-archive-keyring.gpg keyring
// shipped in the debian-archive-keyring Debian package (NOT all
// trusted keys stored in /etc/apt/trusted.gpg.d).
func (g *Getter) loadArchiveKeyrings() error {
	f, err := os.Open("/usr/share/keyrings/debian-archive-keyring.gpg")
	if err != nil {
		// TODO: add helpful error message to install the debian-archive-keyring package in case this is os.IsNotExist
		return err
	}
	defer f.Close()
	g.keyring, err = openpgp.ReadKeyRing(f)
	return err
}

func (g *Getter) GetRelease(suite string) (*archive.Release, error) {
	if err := g.init(); err != nil {
		return nil, err
	}

	// TODO: retry
	// TODO: use correct mirror

	var r io.Reader
	if g.LocalMirror != "" {
		f, err := os.Open(filepath.Join(g.LocalMirror, "dists", suite, "Release"))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	} else {
		// TODO: switch to /InRelease for $TODO-debian-version
		path := "http://ftp.ch.debian.org/debian/dists/" + suite + "/Release"
		resp, err := http.Get(path)
		if err != nil {
			return nil, err
		}

		defer func() {
			ioutil.ReadAll(resp.Body)
			resp.Body.Close()
		}()

		if got, want := resp.StatusCode, http.StatusOK; got != want {
			return nil, fmt.Errorf("GetRelease(%q): Unexpected HTTP status code: got %d, want %d", path, got, want)
		}

		r = resp.Body
	}

	release, err := archive.LoadInRelease(r, &g.keyring)
	if err != nil {
		return nil, err
	}

	g.byHashMu.Lock()
	defer g.byHashMu.Unlock()
	g.byHash[release.Codename] = release.Values["Acquire-By-Hash"] == "yes"
	g.byHash[release.Suite] = release.Values["Acquire-By-Hash"] == "yes"

	return release, err
}
