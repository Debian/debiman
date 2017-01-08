// minisrv serves a manpage repository for development purposes (not
// production!).
package main

import (
	"compress/gzip"
	"errors"
	"flag"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Debian/debiman/internal/aux"
	"github.com/Debian/debiman/internal/redirect"
)

var (
	servingDir = flag.String("serving_dir",
		"/srv/man",
		"Directory from which manpages should be served")

	listenAddr = flag.String("listen",
		"localhost:8089",
		"host:port on which to serve manpages")
)

var fileNotFound = errors.New("File not found")

func serveFile(w http.ResponseWriter, r *http.Request) error {
	compressed := false
	path := filepath.Join(*servingDir, r.URL.Path)
	if r.URL.Path == "/" {
		path = filepath.Join(path, "index.html")
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Try with .gz suffix
			compressed = true
			f, err = os.Open(path + ".gz")
			if err != nil && os.IsNotExist(err) {
				return fileNotFound
			}
		}
		if err != nil {
			return err
		}
	}
	defer f.Close()

	ctype := mime.TypeByExtension(filepath.Ext(path))
	if ctype == "" {
		ctype = "text/html"
	}
	w.Header().Set("Content-Type", ctype)

	rd := io.Reader(f)
	if compressed {
		gzipr, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		rd = gzipr
		defer gzipr.Close()
	}

	_, err = io.Copy(w, rd)
	return err
}

func main() {
	flag.Parse()

	idx, err := redirect.IndexFromProto(filepath.Join(*servingDir, "auxserver.idx"))
	if err != nil {
		log.Fatalf("Could not load auxserver index: %v", err)
	}

	server := aux.NewServer(idx)

	http.HandleFunc("/jump", server.HandleJump)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Similarly to http.ServeFile, deny requests containing .. as
		// a precaution. The server will usually be running on
		// localhost, but might be exposed to the internet for testing
		// temporarily.
		if strings.Contains(r.URL.Path, "..") {
			http.Error(w, "invalid URL path", http.StatusBadRequest)
			log.Printf("Error: invalid URL path %q", r.URL.Path)
			return
		}

		// Check if the path refers to an existing file (possibly compressed)
		err := serveFile(w, r)
		if err != nil && err != fileNotFound {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("Error: %v", err)
			return
		}
		if err == nil {
			return
		}

		server.HandleRedirect(w, r)
	})

	log.Printf("Serving manpages from %q on %q", *servingDir, *listenAddr)
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
