package write

import (
	"bufio"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

func tempDir(dest string) string {
	tempdir := os.Getenv("TMPDIR")
	if tempdir == "" {
		// Convenient for development: decreases the chance that we
		// cannot move files due to /tmp being on a different file
		// system.
		tempdir = filepath.Dir(dest)
	}
	return tempdir
}

func Atomically(dest string, compress bool, write func(w io.Writer) error) (err error) {
	f, err := ioutil.TempFile(tempDir(dest), "debiman-")
	if err != nil {
		return err
	}
	defer func() {
		// Remove the tempfile if an error occurred
		if err != nil {
			os.Remove(f.Name())
		}
	}()
	defer f.Close()

	bufw := bufio.NewWriter(f)

	if compress {
		// NOTE(stapelberg): gzip’s decompression phase takes the same
		// time, regardless of compression level. Hence, we invest the
		// maximum CPU time once to achieve the best compression.
		gzipw, err := gzip.NewWriterLevel(bufw, gzip.BestCompression)
		if err != nil {
			return err
		}
		if err := write(gzipw); err != nil {
			return err
		}
		if err := gzipw.Close(); err != nil {
			return err
		}
	} else {
		if err := write(bufw); err != nil {
			return err
		}
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

func AtomicallyWithGz(dest string, gzipw *gzip.Writer, write func(w io.Writer) error) (err error) {
	f, err := ioutil.TempFile(tempDir(dest), "debiman-")
	if err != nil {
		return err
	}
	defer func() {
		// Remove the tempfile if an error occurred
		if err != nil {
			os.Remove(f.Name())
		}
	}()
	defer f.Close()

	bufw := bufio.NewWriter(f)
	gzipw.Reset(bufw)

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
