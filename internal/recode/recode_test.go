package recode

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"os"
	"testing"
	"unicode/utf8"
)

func readGzipped(fn string) ([]byte, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}

	c, err := ioutil.ReadAll(gz)
	if err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return c, nil
}

func TestRecode(t *testing.T) {
	table := []struct {
		src      string
		dest     string
		language string
	}{
		{
			src: "../../testdata/kterm.1.ja.gz",
			// dest was generated using:
			// gunzip -c testdata/kterm.1.ja.gz| iconv -f EUC-JP -t UTF-8 | gzip -9 -c > testdata/kterm.1.ja.UTF-8.gz
			dest:     "../../testdata/kterm.1.ja.UTF-8.gz",
			language: "ja",
		},
	}
	for _, entry := range table {
		entry := entry // capture
		t.Run(entry.src, func(t *testing.T) {
			t.Parallel()

			srcb, err := readGzipped(entry.src)
			if err != nil {
				t.Fatal(err)
			}

			destb, err := readGzipped(entry.dest)
			if err != nil {
				t.Fatal(err)
			}

			if utf8.Valid(srcb) {
				t.Fatalf("source file %q unexpectedly valid UTF-8", entry.src)
			}

			r := Reader(bytes.NewReader(srcb), entry.language)
			recoded, err := ioutil.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}

			if got, want := bytes.Compare(recoded, destb), 0; got != want {
				t.Fatalf("recoded source file unexpectedly different from golden UTF-8 file: got %d, want %d", got, want)
			}
		})
	}
}
