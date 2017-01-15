package convert

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

var heading = map[string]bool{
	"h1": true,
	"h2": true,
	"h3": true,
	"h4": true,
	"h5": true,
	"h6": true,
}

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

func findXrefs(txt string) [][]int {
	var results [][]int

	lastWordBoundary := -1
	lastOpeningParen := -1

	for i, r := range txt {
		switch {
		case 'a' <= r && r <= 'z':
		case 'A' <= r && r <= 'Z':
		case '0' <= r && r <= '9':
		case r == '-':
		case r == '.':
		case r == '_':
		case r == '(':
			lastOpeningParen = i
		case r == ')':
			if lastOpeningParen > -1 &&
				lastWordBoundary < (lastOpeningParen-1) {
				results = append(results, []int{lastWordBoundary + 1, i + 1})
			}
		default:
			lastWordBoundary = i
			lastOpeningParen = -1
		}
	}
	return results
}

func xrefMatches(txt string, resolve func(ref string) string) []ref {
	xrefm := findXrefs(txt)
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

// findUrls finds anything that roughly like a URL. Matches are
// filtered by checking whether (net/url).Parse returns an error.
func findUrls(txt string) [][]int {
	var results [][]int

	lastWordBoundary := -1
	lastColon := -1
	lastSlash := -1
	inUrl := false

	for i, r := range txt {
		switch {
		case 'a' <= r && r <= 'z':
		case 'A' <= r && r <= 'Z':
		case '0' <= r && r <= '9':
		case r == ':':
			lastColon = i
		case r == '/':
			if lastColon > -1 && lastColon == i-2 && lastSlash == i-1 && lastWordBoundary < (lastColon-1) {
				inUrl = true
			}
			lastSlash = i
		default:
			if inUrl && r != ' ' {
				continue
			}
			if inUrl && r == ' ' {
				results = append(results, []int{lastWordBoundary + 1, i})
				inUrl = false
			}

			lastWordBoundary = i
			lastSlash = -1
			lastColon = -1
		}
	}
	if inUrl {
		results = append(results, []int{lastWordBoundary + 1, len(txt)})
	}
	return results
}

func urlMatches(txt string) []ref {
	urlm := findUrls(txt)
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

func headTable(n *html.Node) bool {
	for _, a := range n.Attr {
		if a.Key == "class" && a.Val == "head" {
			return true
		}
	}
	return false
}

func replaceId(n *html.Node, id string) {
	for idx, a := range n.Attr {
		if a.Key == "id" {
			n.Attr[idx].Val = id
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{
		Key: "id",
		Val: id,
	})
}

func postprocess(resolve func(ref string) string, n *html.Node, toc *[]string) error {
	if n.Parent == nil {
		return nil
	}

	// Remove <html>, <head> and <body> tags, as we are dealing with
	// an HTML fragment that is included in an existing document, not
	// a document itself.
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

	if n.Type == html.ElementNode && heading[n.Data] {
		// Derive and set an id="" attribute for the heading
		text := plaintext(n)
		// HTML5 requires that ids must contain at least one character
		// and may not contain any spaces, see
		// http://stackoverflow.com/a/79022/712014
		id := strings.Replace(text, " ", "_", -1)
		u := url.URL{Fragment: id}
		replaceId(n, id)
		// Insert an <a> element into the heading, after the text. Via
		// CSS, this link will only be made visible while hovering.
		a := &html.Node{
			Type: html.ElementNode,
			Data: "a",
			Attr: []html.Attribute{
				{Key: "class", Val: "anchor"},
				{Key: "href", Val: u.String()},
			},
		}
		a.AppendChild(&html.Node{
			Type: html.TextNode,
			Data: "¶",
		})
		n.AppendChild(a)

		if n.Data == "h1" && toc != nil {
			*toc = append(*toc, text)
		}
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

// TODO(stapelberg): ToHTML’s output currently is used directly as
// (html/template).HTML, i.e. “known safe HTML document fragment”. We
// should be more aggressive in whitelisting the allowed tags.
//
// resolve, if non-nil, will be called to resolve a reference (like
// “rm(1)”) into a URL.
func (p *Process) ToHTML(r io.Reader, resolve func(ref string) string) (doc string, toc []string, err error) {
	stdout, stderr, err := p.mandoc(r)
	if stderr != "" {
		return "", nil, fmt.Errorf("mandoc failed: %v", stderr)
	}
	if err != nil {
		return "", nil, fmt.Errorf("running mandoc failed: %v", err)
	}

	parsed, err := html.Parse(strings.NewReader(stdout))
	if err != nil {
		return "", nil, err
	}

	err = recurse(parsed, func(n *html.Node) error { return postprocess(resolve, n, &toc) })
	if err != nil {
		return "", toc, err
	}
	var rendered bytes.Buffer
	if err := html.Render(&rendered, parsed); err != nil {
		return "", toc, err
	}
	return rendered.String(), toc, nil
}
