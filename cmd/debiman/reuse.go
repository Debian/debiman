package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"os"
)

var mandocDivB = []byte(`<div class="mandoc">`)
var tocLinkPrefix = []byte(`  <a class="toclink"`)
var footerB = []byte(`<div id="footer">`)

func reuse(src string) (doc string, toc []string, err error) {
	f, err := os.Open(src)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	r, err := gzip.NewReader(f)
	if err != nil {
		return "", nil, err
	}
	defer r.Close()

	var (
		buf       bytes.Buffer
		inManpage bool
	)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		b := scanner.Bytes()
		if bytes.Equal(b, mandocDivB) {
			inManpage = true
		}
		if bytes.Equal(b, footerB) {
			all := buf.Bytes()
			all = bytes.TrimSuffix(all, []byte("</div>\n</div>\n"))
			return string(bytes.TrimSpace(all)), toc, nil
		}

		if inManpage {
			if _, err := buf.Write(b); err != nil {
				return "", nil, err
			}
			if _, err := buf.Write([]byte{'\n'}); err != nil {
				return "", nil, err
			}
		} else if bytes.HasPrefix(b, tocLinkPrefix) {
			entry := bytes.TrimSuffix(b, []byte("</a>"))
			off := bytes.Index(entry, []byte{'>'})
			if off > -1 {
				toc = append(toc, string(entry[off+1:]))
			}
		}
	}
	return buf.String(), nil, scanner.Err()
}
