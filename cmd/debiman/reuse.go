package main

import (
	"bytes"
	"compress/gzip"
	"os"

	"golang.org/x/net/html"
)

func recurse(n *html.Node, f func(c *html.Node) error) error {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if err := recurse(c, f); err != nil {
			return err
		}
		if err := f(c); err != nil {
			return err
		}
	}
	return nil
}

func mandocDiv(n *html.Node) bool {
	for _, a := range n.Attr {
		if a.Key == "class" && a.Val == "mandoc" {
			return true
		}
	}
	return false
}

func tocLink(n *html.Node) bool {
	for _, a := range n.Attr {
		if a.Key == "class" && a.Val == "toclink" {
			return true
		}
	}
	return false
}

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
	parsed, err := html.Parse(r)
	if err != nil {
		return "", nil, err
	}

	if err := r.Close(); err != nil {
		return "", nil, err
	}

	if err := recurse(parsed, func(n *html.Node) error {
		if n.Type == html.ElementNode && n.Data == "div" && mandocDiv(n) {
			var buf bytes.Buffer
			if err := html.Render(&buf, n); err != nil {
				return err
			}
			doc = buf.String()
			return nil
		}

		if n.Type == html.ElementNode && n.Data == "a" && tocLink(n) {
			toc = append(toc, n.FirstChild.Data)
		}

		return nil
	}); err != nil {
		return "", nil, err
	}

	return doc, toc, nil
}
