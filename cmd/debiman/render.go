package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Debian/debiman/internal/commontmpl"
	"github.com/Debian/debiman/internal/convert"
	"github.com/Debian/debiman/internal/manpage"
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
)

type breadcrumb struct {
	Link string
	Text string
}

var commonTmpls = commontmpl.MustParseCommonTmpls()

type renderingMode int

const (
	regularFiles renderingMode = iota
	symlinks
	packageIndex
)

// walkManContents walks over all entries in dir and, depending on mode, does:
// 1. send a renderJob for each regular file
// 2. send a renderJob for each symlink
// 3. renders a directory index
func walkManContents(ctx context.Context, renderChan chan<- renderJob, dir string, mode renderingMode, gv globalView, newestModTime time.Time) (time.Time, error) {
	// the invariant is: each file ending in .gz must have a corresponding .html.gz file
	// the .html.gz must have a modtime that is >= the modtime of the .gz file

	var manpageByName map[string]*manpage.Meta
	if mode == packageIndex {
		manpageByName = make(map[string]*manpage.Meta)
	}

	files, err := os.Open(dir)
	if err != nil {
		return newestModTime, err
	}
	defer files.Close()

	var predictedEof bool
	for {
		if predictedEof {
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
		predictedEof = len(names) < 2048

		for _, fn := range names {
			if !strings.HasSuffix(fn, ".gz") ||
				strings.HasSuffix(fn, ".html.gz") {
				continue
			}
			full := filepath.Join(dir, fn)
			if mode == packageIndex {
				m, err := manpage.FromServingPath(*servingDir, full)
				if err != nil {
					// If we run into this case, our code cannot correctly
					// interpret the result of ServingPath().
					log.Printf("BUG: cannot parse manpage from serving path %q: %v", full, err)
					continue
				}

				manpageByName[fn] = m
				continue
			}

			st, err := os.Lstat(full)
			if err != nil {
				continue
			}
			if st.ModTime().After(newestModTime) {
				newestModTime = st.ModTime()
			}

			symlink := st.Mode()&os.ModeSymlink != 0

			if mode == regularFiles && symlink ||
				mode == symlinks && !symlink {
				continue
			}

			n := strings.TrimSuffix(fn, ".gz") + ".html.gz"
			htmlst, err := os.Stat(filepath.Join(dir, n))
			if err != nil || htmlst.ModTime().Before(st.ModTime()) {
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
				select {
				case renderChan <- renderJob{
					dest:     filepath.Join(dir, n),
					src:      full,
					meta:     m,
					versions: versions,
					xref:     gv.xref,
					modTime:  st.ModTime(),
					symlink:  symlink,
				}:
				case <-ctx.Done():
					break
				}
			}
		}
	}

	if mode != packageIndex {
		return newestModTime, nil
	}

	st, err := os.Stat(filepath.Join(dir, "index.html.gz"))
	if err == nil && st.ModTime().After(newestModTime) {
		return newestModTime, nil
	}

	if len(manpageByName) == 0 {
		log.Printf("WARNING: empty directory %q, not generating package index", dir)
		return newestModTime, nil
	}

	if err := renderPkgindex(filepath.Join(dir, "index.html.gz"), manpageByName); err != nil {
		return newestModTime, err
	}

	return newestModTime, nil
}

func walkContents(ctx context.Context, renderChan chan<- renderJob, whitelist map[string]bool, gv globalView) error {
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
					if _, err := walkManContents(ctx, renderChan, dir, packageIndex, gv, newestModTime); err != nil {
						return err
					}
					return nil
				})
			}
			if err := wg.Wait(); err != nil {
				return err
			}
		}
		bins.Close()
	}
	return nil
}

func renderAll(gv globalView) error {
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
			gzipw, err := gzip.NewWriterLevel(nil, gzip.BestCompression)
			if err != nil {
				return err
			}

			for r := range renderChan {
				if err := rendermanpage(gzipw, converter, r); err != nil {
					// rendermanpage writes an error page if rendering
					// failed, any returned error is severe (e.g. file
					// system full) and should lead to termination.
					return err
				}
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
