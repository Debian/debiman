package main

import (
	"flag"

	"os"
	"testing"
)

func TestEndToEnd(t *testing.T) {
	dir, err := os.MkdirTemp("", "debiman")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	flag.Set("serving_dir", dir)
	flag.Set("local_mirror", "../../testdata/tinymirror")
	flag.Set("keyring", "../../testdata/tinymirror/keyring.gpg")
	if err := logic(); err != nil {
		t.Fatal(err)
	}
}
