package convert

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/net/html"
)

func recurse(n *html.Node, f func(c *html.Node) error) error {
	c := n.FirstChild
	for c != nil {
		next := c.NextSibling
		if err := recurse(c, f); err != nil {
			return err
		}
		if err := f(c); err != nil {
			return err
		}
		c = next
	}
	return nil
}

var (
	xrefRe = regexp.MustCompile(`\b[A-Za-z0-9_.-]+\([^)]+\)`)
	// urlRe is a regular expression which matches anything that looks
	// roughly like a URL. Matches are filtered by checking whether
	// (net/url).Parse returns an error.
	urlRe = regexp.MustCompile(`[A-Za-z0-9]+://[^ ]+`)
)

type ref struct {
	pos  []int
	dest string
}

type byStart []ref

func (p byStart) Len() int {
	return len(p)
}
func (p byStart) Less(i, j int) bool {
	return p[i].pos[0] < p[j].pos[0]
}
func (p byStart) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func xrefMatches(txt string, resolve func(ref string) string) []ref {
	xrefm := xrefRe.FindAllStringIndex(txt, -1)
	matches := make([]ref, 0, len(xrefm))
	for _, r := range xrefm {
		url := resolve(txt[r[0]:r[1]])
		if url == "" {
			continue
		}
		matches = append(matches, ref{
			pos:  r,
			dest: url})
	}
	return matches
}

func urlMatches(txt string) []ref {
	urlm := urlRe.FindAllStringIndex(txt, -1)
	matches := make([]ref, 0, len(urlm))
	for _, r := range urlm {
		match := txt[r[0]:r[1]]
		u, err := url.Parse(match)
		if err != nil {
			continue
		}
		matches = append(matches, ref{
			pos:  r,
			dest: u.String()})
	}
	return matches
}

func xref(txt string, resolve func(ref string) string) []*html.Node {
	urlm := urlMatches(txt)
	// all xref matches (unfiltered)
	xrefa := xrefMatches(txt, resolve)
	// filter out xrefs which
	xrefm := make([]ref, 0, len(xrefa))
	for _, x := range xrefa {
		// TODO: better algorithm
		var found bool
		for _, u := range urlm {
			if x.pos[0] >= u.pos[0] && x.pos[1] <= u.pos[1] {
				found = true
				break
			}
		}
		if found {
			continue
		}
		xrefm = append(xrefm, x)
	}

	matches := append(xrefm, urlm...)
	if len(matches) == 0 {
		return nil
	}
	sort.Sort(byStart(matches))

	var res []*html.Node
	var last int
	for _, m := range matches {
		match := txt[m.pos[0]:m.pos[1]]

		res = append(res, &html.Node{
			Type: html.TextNode,
			Data: txt[last:m.pos[0]],
		})
		a := &html.Node{
			Type: html.ElementNode,
			Data: "a",
			Attr: []html.Attribute{
				{Key: "href", Val: m.dest},
			},
		}
		a.AppendChild(&html.Node{
			Type: html.TextNode,
			Data: match,
		})
		res = append(res, a)
		last = m.pos[1]
	}
	res = append(res, &html.Node{
		Type: html.TextNode,
		Data: txt[last:],
	})

	return res
}

func plaintext(n *html.Node) string {
	var result string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result = result + plaintext(c)
	}
	if n.Type == html.TextNode {
		return result + n.Data
	}
	return result
}

func postprocess(resolve func(ref string) string, n *html.Node) error {
	if n.Parent == nil {
		return nil
	}
	if n.Type == html.ElementNode &&
		(n.Data == "html" ||
			n.Data == "head" ||
			n.Data == "body") {
		c := n.FirstChild
		for c != nil {
			next := c.NextSibling
			n.RemoveChild(c)
			n.Parent.InsertBefore(c, n)
			c = next
		}
		n.Parent.RemoveChild(n)
		return nil
	}
	if resolve == nil {
		return nil
	}
	// resolve cross references
	if n.Type == html.TextNode {
		replacements := xref(n.Data, resolve)
		for _, r := range replacements {
			n.Parent.InsertBefore(r, n)
		}
		if replacements != nil {
			n.Parent.RemoveChild(n)
			return nil
		}
	}
	if n.Type == html.TextNode &&
		strings.HasPrefix(n.Data, "(") &&
		strings.Index(n.Data, ")") > -1 &&
		n.PrevSibling != nil {
		replacements := xref(plaintext(n.PrevSibling)+n.Data, resolve)
		if replacements != nil {
			n.Parent.RemoveChild(n.PrevSibling)
			for _, r := range replacements {
				n.Parent.InsertBefore(r, n)
			}
			n.Parent.RemoveChild(n)
		}
		return nil
	}

	return nil

}

var (
	unixConn     *net.UnixConn
	unixConnOnce sync.Once
)

func mandoc(r io.Reader) (string, error) {
	unixConnOnce.Do(func() {
		// TODO: configurable, error handling, parallelism
		var err error
		unixConn, err = net.DialUnix("unix", nil, &net.UnixAddr{
			Name: "/tmp/foo.sock",
			Net:  "unix"})
		if err != nil {
			panic(err)
		}
	})
	manr, manw, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer manr.Close()
	defer manw.Close()
	outr, outw, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer outr.Close()
	defer outw.Close()
	scm := syscall.UnixRights(int(manr.Fd()), int(outw.Fd()))
	if _, _, err := unixConn.WriteMsgUnix(nil, scm, nil); err != nil {
		return "", err
	}
	// Now that the remote process has them, close our duplicate file
	// descriptors to make EOF signaling work.
	manr.Close()
	outw.Close()

	if _, err := io.Copy(manw, r); err != nil {
		return "", nil
	}
	if err := manw.Close(); err != nil {
		return "", nil
	}

	b, err := ioutil.ReadAll(outr)
	if len(b) == 0 {
		return "", fmt.Errorf("mandoc returned an empty document")
	}
	return string(b), err
}

// TODO(stapelberg): ToHTML’s output currently is used directly as
// (html/template).HTML, i.e. “known safe HTML document fragment”. We
// should be more aggressive in whitelisting the allowed tags.
//
// resolve, if non-nil, will be called to resolve a reference (like
// “rm(1)”) into a URL.
func ToHTML(r io.Reader, resolve func(ref string) string) (string, error) {
	// TODO: add table of contents
	// TODO: add paragraph signs next to each header for permalinks

	out, err := mandoc(r)
	if err != nil {
		return "", err
	}
	// var out bytes.Buffer
	// cmd := exec.Command("mandoc", "-Ofragment", "-Thtml")
	// cmd.Stdin = r
	// cmd.Stdout = &out
	// cmd.Stderr = os.Stderr
	// if err := cmd.Run(); err != nil {
	// 	return "", err
	// }

	doc, err := html.Parse(strings.NewReader(out))
	if err != nil {
		return "", err
	}

	err = recurse(doc, func(n *html.Node) error { return postprocess(resolve, n) })
	if err != nil {
		return "", err
	}
	var rendered bytes.Buffer
	if err := html.Render(&rendered, doc); err != nil {
		return "", err
	}
	return rendered.String(), nil
}
