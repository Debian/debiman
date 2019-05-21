package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Debian/debiman/internal/commontmpl"
	"github.com/Debian/debiman/internal/convert"
	"github.com/Debian/debiman/internal/manpage"
	"github.com/Debian/debiman/internal/sitemap"
	"github.com/Debian/debiman/internal/write"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
)

var (
	manwalkConcurrency = flag.Int("concurrency_manwalk",
		1000, // below the default 1024 open file descriptor limit
		"Concurrency level for walking through binary package man directories (ulimit -n must be higher!)")

	renderConcurrency = flag.Int("concurrency_render",
		5,
		"Concurrency level for rendering manpages using mandoc")

	gzipLevel = flag.Int("gzip",
		9,
		"gzip compression level to use for compressing HTML versions of manpages. defaults to 9 to keep network traffic minimal, but useful to reduce for development/disaster recovery (level 1 results in a 2x speedup!)")

	baseURL = flag.String("base_url",
		"https://manpages.debian.org",
		"Base URL (without trailing slash) to the site. Used where absolute URLs are required, e.g. sitemaps.")
)

type breadcrumb struct {
	Link string
	Text string
}

type breadcrumbs []breadcrumb

func (b breadcrumbs) ToJSON() template.HTML {
	type item struct {
		Type string `json:"@type"`
		ID   string `json:"@id"`
		Name string `json:"name"`
	}
	type listItem struct {
		Type     string `json:"@type"`
		Position int    `json:"position"`
		Item     item   `json:"item"`
	}
	type breadcrumbList struct {
		Context  string     `json:"@context"`
		Type     string     `json:"@type"`
		Elements []listItem `json:"itemListElement"`
	}
	l := breadcrumbList{
		Context:  "http://schema.org",
		Type:     "BreadcrumbList",
		Elements: make([]listItem, len(b)),
	}
	for idx, br := range b {
		l.Elements[idx] = listItem{
			Type:     "ListItem",
			Position: idx + 1,
			Item: item{
				Type: "Thing",
				Id:   br.Link,
				Name: br.Text,
			},
		}
	}
	jsonb, err := json.Marshal(l)
	if err != nil {
		log.Fatal(err)
	}
	return template.HTML(jsonb)
}

var commonTmpls = commontmpl.MustParseCommonTmpls()

type renderingMode int

const (
	regularFiles renderingMode = iota
	symlinks
)

// listManpages lists all files in dir (non-recursively) and returns a map from
// filename (within dir) to *manpage.Meta.
func listManpages(dir string) (map[string]*manpage.Meta, error) {
	manpageByName := make(map[string]*manpage.Meta)

	files, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer files.Close()

	var predictedEOF bool
	for {
		if predictedEOF {
			break
		}

		names, err := files.Readdirnames(2048)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				// We avoid an additional stat syscalls for each
				// binary package directory by just optimistically
				// calling readdir and handling the ENOTDIR error.
				if sce, ok := err.(*os.SyscallError); ok && sce.Err == syscall.ENOTDIR {
					return nil, nil
				}
				return nil, err
			}
		}

		// When len(names) < 2048 the next Readdirnames() call will
		// result in io.EOF and can be skipped to reduce getdents(2)
		// syscalls by half.
		predictedEOF = len(names) < 2048

		for _, fn := range names {
			if !strings.HasSuffix(fn, ".gz") ||
				strings.HasSuffix(fn, ".html.gz") {
				continue
			}
			full := filepath.Join(dir, fn)

			m, err := manpage.FromServingPath(*servingDir, full)
			if err != nil {
				// If we run into this case, our code cannot correctly
				// interpret the result of ServingPath().
				log.Printf("BUG: cannot parse manpage from serving path %q: %v", full, err)
				continue
			}

			manpageByName[fn] = m
		}
	}
	return manpageByName, nil
}

func renderDirectoryIndex(dir string, newestModTime time.Time) error {
	st, err := os.Stat(filepath.Join(dir, "index.html.gz"))
	if !*forceRerender && err == nil && st.ModTime().After(newestModTime) {
		return nil
	}

	manpageByName, err := listManpages(dir)
	if err != nil {
		return err
	}

	if len(manpageByName) == 0 {
		log.Printf("WARNING: empty directory %q, not generating package index", dir)
		return nil
	}

	return renderPkgindex(filepath.Join(dir, "index.html.gz"), manpageByName)
}

// walkManContents walks over all entries in dir and, depending on mode, does:
// 1. send a renderJob for each regular file
// 2. send a renderJob for each symlink
func walkManContents(ctx context.Context, renderChan chan<- renderJob, dir string, mode renderingMode, gv globalView, newestModTime time.Time) (time.Time, error) {
	// the invariant is: each file ending in .gz must have a corresponding .html.gz file
	// the .html.gz must have a modtime that is >= the modtime of the .gz file

	files, err := os.Open(dir)
	if err != nil {
		return newestModTime, err
	}
	defer files.Close()

	var predictedEOF bool
	for {
		if predictedEOF {
			break
		}

		names, err := files.Readdirnames(2048)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				// We avoid an additional stat syscalls for each
				// binary package directory by just optimistically
				// calling readdir and handling the ENOTDIR error.
				if sce, ok := err.(*os.SyscallError); ok && sce.Err == syscall.ENOTDIR {
					return newestModTime, nil
				}
				return newestModTime, err
			}
		}

		// When len(names) < 2048 the next Readdirnames() call will
		// result in io.EOF and can be skipped to reduce getdents(2)
		// syscalls by half.
		predictedEOF = len(names) < 2048

		for _, fn := range names {
			if !strings.HasSuffix(fn, ".gz") ||
				strings.HasSuffix(fn, ".html.gz") {
				continue
			}
			full := filepath.Join(dir, fn)

			st, err := os.Lstat(full)
			if err != nil {
				continue
			}
			if st.ModTime().After(newestModTime) {
				newestModTime = st.ModTime()
			}

			symlink := st.Mode()&os.ModeSymlink != 0

			if !symlink {
				atomic.AddUint64(&gv.stats.ManpageBytes, uint64(st.Size()))
			}

			if mode == regularFiles && symlink ||
				mode == symlinks && !symlink {
				continue
			}

			n := strings.TrimSuffix(fn, ".gz") + ".html.gz"
			htmlst, err := os.Stat(filepath.Join(dir, n))
			if err == nil {
				atomic.AddUint64(&gv.stats.HTMLBytes, uint64(htmlst.Size()))
			}
			if err != nil || *forceRerender || htmlst.ModTime().Before(st.ModTime()) {
				m, err := manpage.FromServingPath(*servingDir, full)
				if err != nil {
					// If we run into this case, our code cannot correctly
					// interpret the result of ServingPath().
					log.Printf("BUG: cannot parse manpage from serving path %q: %v", full, err)
					continue
				}

				versions := gv.xref[m.Name]
				// Replace m with its corresponding entry in versions
				// so that rendermanpage() can use pointer equality to
				// efficiently skip entries.
				for _, v := range versions {
					if v.ServingPath() == m.ServingPath() {
						m = v
						break
					}
				}

				// Render dependent manpages first to properly resume
				// in case debiman is interrupted.
				for _, v := range versions {
					if v == m || *forceRerender {
						continue
					}

					vfull := filepath.Join(*servingDir, v.RawPath())
					vfn := filepath.Join(*servingDir, v.ServingPath()+".html.gz")
					vhtmlst, err := os.Stat(vfn)
					if err == nil && vhtmlst.ModTime().After(gv.start) {
						// The variant was already re-rendered with this globalView.
						continue
					}

					vst, err := os.Stat(vfull)
					if err != nil {
						log.Printf("WARNING: stat %q: %v", vfull, err)
						continue
					}

					vreuse := ""
					if vhtmlst != nil && vhtmlst.ModTime().After(vst.ModTime()) {
						vreuse = vfn
					}

					log.Printf("%s invalidated by %s", vfn, full)

					select {
					case renderChan <- renderJob{
						dest:     vfn,
						src:      vfull,
						meta:     v,
						versions: versions,
						xref:     gv.xref,
						modTime:  vst.ModTime(),
						reuse:    vreuse,
					}:
					case <-ctx.Done():
						break
					}
				}

				var reuse string
				if symlink {
					link, err := os.Readlink(full)
					if err == nil {
						resolved := filepath.Join(dir, link)
						reuse = strings.TrimSuffix(resolved, ".gz") + ".html.gz"
					}
				}

				select {
				case renderChan <- renderJob{
					dest:     filepath.Join(dir, n),
					src:      full,
					meta:     m,
					versions: versions,
					xref:     gv.xref,
					modTime:  st.ModTime(),
					reuse:    reuse,
				}:
				case <-ctx.Done():
					break
				}
			}
		}
	}

	return newestModTime, nil
}

func walkContents(ctx context.Context, renderChan chan<- renderJob, whitelist map[string]bool, gv globalView) error {
	sitemaps := make(map[string]time.Time)

	suitedirs, err := ioutil.ReadDir(*servingDir)
	if err != nil {
		return err
	}
	for _, sfi := range suitedirs {
		if !sfi.IsDir() {
			continue
		}
		if !gv.suites[sfi.Name()] {
			continue
		}
		bins, err := os.Open(filepath.Join(*servingDir, sfi.Name()))
		if err != nil {
			return err
		}
		defer bins.Close()

		// 20000 is the order of magnitude of binary packages
		// (containing manpages) in any given Debian suite, so that is
		// a good value to start with.
		sitemapEntries := make(map[string]time.Time, 20000)
		var sitemapEntriesMu sync.RWMutex

		for {
			names, err := bins.Readdirnames(*manwalkConcurrency)
			if err != nil {
				if err == io.EOF {
					break
				} else {
					return err
				}
			}

			var wg errgroup.Group
			for _, bfn := range names {
				if whitelist != nil && !whitelist[bfn] {
					continue
				}

				bfn := bfn // copy
				dir := filepath.Join(*servingDir, sfi.Name(), bfn)
				wg.Go(func() error {
					// Iterating through the same directory in all
					// modes increases the chance for the dirents to
					// still be cached. This is important for machines
					// like manziarly.debian.org, which do not have
					// enough RAM to keep all dirents cached over the
					// runtime of this code path.

					var newestModTime time.Time
					var err error
					// Render all regular files first
					newestModTime, err = walkManContents(ctx, renderChan, dir, regularFiles, gv, newestModTime)
					if err != nil {
						return err
					}

					// then render all symlinks, re-using the rendered fragments
					newestModTime, err = walkManContents(ctx, renderChan, dir, symlinks, gv, newestModTime)
					if err != nil {
						return err
					}

					// and finally render the package index files which need to
					// consider both regular files and symlinks.
					if err := renderDirectoryIndex(dir, newestModTime); err != nil {
						return err
					}

					if !newestModTime.IsZero() {
						sitemapEntriesMu.Lock()
						defer sitemapEntriesMu.Unlock()
						sitemapEntries[bfn] = newestModTime
					}

					return nil
				})
			}
			if err := wg.Wait(); err != nil {
				return err
			}
		}
		bins.Close()

		sitemapPath := filepath.Join(*servingDir, sfi.Name(), "sitemap.xml.gz")
		if err := write.Atomically(sitemapPath, true, func(w io.Writer) error {
			return sitemap.WriteTo(w, *baseURL+"/"+sfi.Name(), sitemapEntries)
		}); err != nil {
			return err
		}
		st, err := os.Stat(sitemapPath)
		if err == nil {
			sitemaps[sfi.Name()] = st.ModTime()
		}
	}
	return write.Atomically(filepath.Join(*servingDir, "sitemapindex.xml.gz"), true, func(w io.Writer) error {
		return sitemap.WriteIndexTo(w, *baseURL, sitemaps)
	})
}

func writeSourceIndex(gv globalView, newestForSource map[string]time.Time) error {
	// Partition by suite for reduced memory usage and better locality of file
	// system access
	for suite := range gv.suites {
		binariesBySource := make(map[string][]string)
		for _, p := range gv.pkgs {
			if p.suite != suite {
				continue
			}
			binariesBySource[p.source] = append(binariesBySource[p.source], p.binarypkg)
		}

		for src, binaries := range binariesBySource {
			srcDir := filepath.Join(*servingDir, suite, "src:"+src)
			// skip if current index file is more recent than newestForSource
			st, err := os.Stat(filepath.Join(srcDir, "index.html.gz"))
			if !*forceRerender && err == nil && st.ModTime().After(newestForSource[src]) {
				continue
			}

			// Aggregate manpages of all binary packages for this source package
			manpages := make(map[string]*manpage.Meta)
			for _, binary := range binaries {
				m, err := listManpages(filepath.Join(*servingDir, suite, binary))
				if err != nil {
					if os.IsNotExist(err) {
						continue // The package might not contain any manpages.
					}
					return err
				}
				for k, v := range m {
					manpages[k] = v
				}
			}
			if len(manpages) == 0 {
				continue // The entire source package does not contain any manpages.
			}

			if err := os.MkdirAll(srcDir, 0755); err != nil {
				return err
			}
			if err := renderSrcPkgindex(filepath.Join(srcDir, "index.html.gz"), src, manpages); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeSourcesWithManpages(gv globalView) error {
	for suite := range gv.suites {
		hasManpages := make(map[string]bool)
		for _, p := range gv.pkgs {
			if p.suite != suite {
				continue
			}
			hasManpages[p.source] = true
		}
		sourcesWithManpages := make([]string, 0, len(hasManpages))
		for source := range hasManpages {
			sourcesWithManpages = append(sourcesWithManpages, source)
		}
		sort.Strings(sourcesWithManpages)
		dest := filepath.Join(*servingDir, suite, "sourcesWithManpages.txt.gz")
		if err := write.Atomically(dest, true, func(w io.Writer) error {
			for _, source := range sourcesWithManpages {
				if _, err := fmt.Fprintln(w, source); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func renderAll(gv globalView) error {
	log.Printf("Preparing inverted maps")
	sourceByBinary := make(map[string]string, len(gv.pkgs))
	newestForSource := make(map[string]time.Time)
	for _, p := range gv.pkgs {
		sourceByBinary[p.suite+"/"+p.binarypkg] = p.source
		newestForSource[p.source] = time.Time{}
	}
	log.Printf("%d sourceByBinary entries, %d newestForSource entries", len(sourceByBinary), len(newestForSource))

	eg, ctx := errgroup.WithContext(context.Background())
	renderChan := make(chan renderJob)
	for i := 0; i < *renderConcurrency; i++ {
		eg.Go(func() error {
			converter, err := convert.NewProcess()
			if err != nil {
				return err
			}
			defer converter.Kill()

			// NOTE(stapelberg): gzip’s decompression phase takes the same
			// time, regardless of compression level. Hence, we invest the
			// maximum CPU time once to achieve the best compression.
			gzipw, err := gzip.NewWriterLevel(nil, *gzipLevel)
			if err != nil {
				return err
			}

			for r := range renderChan {
				n, err := rendermanpage(gzipw, converter, r)
				if err != nil {
					// rendermanpage writes an error page if rendering
					// failed, any returned error is severe (e.g. file
					// system full) and should lead to termination.
					return err
				}

				atomic.AddUint64(&gv.stats.HTMLBytes, n)
				atomic.AddUint64(&gv.stats.ManpagesRendered, 1)
			}
			return nil
		})
	}

	var whitelist map[string]bool
	if *onlyRender != "" {
		whitelist = make(map[string]bool)
		log.Printf("Restricting rendering to the following binary packages:")
		for _, e := range strings.Split(strings.TrimSpace(*onlyRender), ",") {
			whitelist[e] = true
			log.Printf("  %q", e)
		}
		log.Printf("(total: %d whitelist entries)", len(whitelist))
	}

	if err := walkContents(ctx, renderChan, whitelist, gv); err != nil {
		return err
	}

	close(renderChan)
	if err := eg.Wait(); err != nil {
		return err
	}

	if err := writeSourceIndex(gv, newestForSource); err != nil {
		return fmt.Errorf("writing source index: %v", err)
	}

	if err := writeSourcesWithManpages(gv); err != nil {
		return fmt.Errorf("writing sourcesWithManpages: %v", err)
	}

	suitedirs, err := ioutil.ReadDir(*servingDir)
	if err != nil {
		return err
	}
	for _, sfi := range suitedirs {
		if !sfi.IsDir() {
			continue
		}
		if !gv.suites[sfi.Name()] {
			continue
		}
		bins, err := os.Open(filepath.Join(*servingDir, sfi.Name()))
		if err != nil {
			return err
		}
		defer bins.Close()

		names, err := bins.Readdirnames(-1)
		if err != nil {
			return err
		}

		if err := renderContents(filepath.Join(*servingDir, fmt.Sprintf("contents-%s.html.gz", sfi.Name())), sfi.Name(), names); err != nil {
			return err
		}

		bins.Close()
	}

	return nil
}
