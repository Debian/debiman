package main

import (
	"compress/gzip"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Debian/debiman/internal/convert"
	"github.com/Debian/debiman/internal/manpage"
)

func TestReuse(t *testing.T) {
	const manContents = `.SH foobar
baz
.SH qux
`
	f, err := ioutil.TempFile("", "debiman-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	gzipw := gzip.NewWriter(f)
	if _, err := gzipw.Write([]byte(manContents)); err != nil {
		t.Fatal(err)
	}
	if err := gzipw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	meta := &manpage.Meta{
		Name:     "test",
		Section:  "1",
		Language: "en",
		Package: &manpage.PkgMeta{
			Binarypkg: "test",
			Suite:     "jessie",
		},
	}

	if err := rendermanpage(renderJob{
		dest:     f.Name(),
		src:      f.Name(),
		meta:     meta,
		versions: []*manpage.Meta{meta},
		xref: map[string][]*manpage.Meta{
			"test": []*manpage.Meta{meta},
		},
		modTime: time.Now(),
		symlink: false,
	}); err != nil {
		t.Fatal(err)
	}

	converter, err := convert.NewProcess()
	if err != nil {
		t.Fatal(err)
	}
	docWant, tocWant, err := converter.ToHTML(strings.NewReader(manContents), nil)
	if err != nil {
		t.Fatal(err)
	}

	docGot, tocGot, err := reuse(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	docGot = strings.TrimSpace(docGot)
	docWant = strings.TrimSpace(docWant)

	if docGot != docWant {
		t.Fatalf("Unexpected HTML fragment: got %q, want %q", docGot, docWant)
	}

	if got, want := len(tocGot), len(tocWant); got != want {
		t.Fatalf("Unexpected table of contents length: got %d, want %d", got, want)
	}
	for n := 0; n < len(tocGot); n++ {
		if got, want := tocGot[n], tocWant[n]; got != want {
			t.Fatalf("Unexpected table of contents element %d: got %q, want %q", n, got, want)
		}
	}
}
