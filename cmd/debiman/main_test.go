package main

import (
	"flag"
	"io/ioutil"
	"testing"
)

func TestEndToEnd(t *testing.T) {
	dir, err := ioutil.TempDir("", "debiman")
	if err != nil {
		t.Fatal(err)
	}
	flag.Set("serving_dir", dir)
	flag.Set("local_mirror", "../../testdata/tinymirror")
	if err := logic(); err != nil {
		t.Fatal(err)
	}
}
