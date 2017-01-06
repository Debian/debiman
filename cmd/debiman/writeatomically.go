package main

import (
	"bufio"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

func writeAtomically(dest string, write func(w io.Writer) error) error {
	f, err := ioutil.TempFile(filepath.Dir(dest), "debiman-")
	if err != nil {
		return err
	}
	defer f.Close()
	// TODO: defer os.Remove() in case we return before the tempfile is destroyed

	// TODO(later): benchmark/support other compression algorithms. zopfli gets dos2unix from 9659B to 9274B (4% win)

	bufw := bufio.NewWriter(f)

	// NOTE(stapelberg): gzip’s decompression phase takes the same
	// time, regardless of compression level. Hence, we invest the
	// maximum CPU time once to achieve the best compression.
	gzipw, err := gzip.NewWriterLevel(bufw, gzip.BestCompression)
	if err != nil {
		return err
	}
	defer gzipw.Close()

	if err := write(gzipw); err != nil {
		return err
	}

	if err := gzipw.Close(); err != nil {
		return err
	}

	if err := bufw.Flush(); err != nil {
		return err
	}

	if err := f.Chmod(0644); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(f.Name(), dest)
}
