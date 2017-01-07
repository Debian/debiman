package convert

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func diff(got, want string) ([]byte, error) {
	gf, err := ioutil.TempFile("", "debiman-")
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(gf, got); err != nil {
		return nil, err
	}
	if err := gf.Close(); err != nil {
		return nil, err
	}
	defer os.Remove(gf.Name())

	wf, err := ioutil.TempFile("", "debiman-")
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(wf, want); err != nil {
		return nil, err
	}
	if err := wf.Close(); err != nil {
		return nil, err
	}
	defer os.Remove(wf.Name())

	cmd := exec.Command("diff", "-u", wf.Name(), gf.Name())
	out, err := cmd.Output()
	if out == nil {
		return nil, err
	} else {
		return out, nil
	}
}

func TestToHTML(t *testing.T) {
	refs := map[string]string{
		"i3lock(1)":          "testing/i3lock/i3lock.1.C",
		"i3-msg(1)":          "testing/i3-wm/i3-msg.1.C",
		"systemd.service(5)": "testing/systemd/systemd.service.5.C",
	}
	docs := []string{"i3lock", "refs"}
	for _, d := range docs {
		d := d // copy
		t.Run(d, func(t *testing.T) {
			t.Parallel()
			f, err := os.Open("../../testdata/" + d + ".1")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			got, _, err := ToHTML(f, func(ref string) string {
				return refs[ref]
			})
			if err != nil {
				t.Fatal(err)
			}
			b, err := ioutil.ReadFile("../../testdata/" + d + ".html")
			if err != nil {
				t.Fatal(err)
			}
			want := string(b)

			// Ignore leading/trailing whitespace to make the tests more
			// forgiving to differences in text editor setup.
			got = strings.TrimSpace(got)
			want = strings.TrimSpace(want)
			if got != want {
				d, err := diff(got, want)
				if err == nil {
					t.Fatalf("unexpected conversion result: (diff from want → got):\n%s\n", string(d))
				} else {
					t.Fatalf("unexpected conversion result: got %q, want %q (diff error: %v)", got, want, err)
				}
			}
		})
	}
}

func cmpElems(input *html.Node, got []*html.Node, want []*html.Node) error {
	gotn := &html.Node{
		Type: html.ElementNode,
		Data: "div"}
	for _, r := range got {
		gotn.AppendChild(r)
	}
	if len(got) == 0 {
		if input.Parent != nil {
			input.Parent.RemoveChild(input)
		}
		gotn.AppendChild(input)
	}

	var gotb bytes.Buffer
	if err := html.Render(&gotb, gotn); err != nil {
		log.Fatal(err)
	}

	wantn := &html.Node{
		Type: html.ElementNode,
		Data: "div"}

	for _, r := range want {
		// cmpElems might be called multiple times with the same want pointer
		if r.Parent != nil {
			r.Parent.RemoveChild(r)
		}
		wantn.AppendChild(r)
	}

	var wantb bytes.Buffer
	if err := html.Render(&wantb, wantn); err != nil {
		log.Fatal(err)
	}

	if got, want := gotb.String(), wantb.String(); got != want {
		return fmt.Errorf("got %q, want %q", got, want)
	}
	return nil
}

func TestXref(t *testing.T) {
	input := &html.Node{
		Type: html.TextNode,
		Data: "more details can be found in systemd.service(5), systemd.exec(5) and others",
	}

	if err := cmpElems(input, xref(input.Data, func(ref string) string { return "" }), []*html.Node{input}); err != nil {
		t.Fatalf("Unexpected xref() HTML result: %v", err)
	}

	a1 := &html.Node{
		Type: html.ElementNode,
		Data: "a",
		Attr: []html.Attribute{
			{Key: "href", Val: "systemd.service(5)"},
		},
	}
	a1.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: "systemd.service(5)",
	})

	a2 := &html.Node{
		Type: html.ElementNode,
		Data: "a",
		Attr: []html.Attribute{
			{Key: "href", Val: "systemd.exec(5)"},
		},
	}
	a2.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: "systemd.exec(5)",
	})

	want := []*html.Node{
		&html.Node{
			Type: html.TextNode,
			Data: "more details can be found in ",
		},
		a1,
		&html.Node{
			Type: html.TextNode,
			Data: ", ",
		},
		a2,
		&html.Node{
			Type: html.TextNode,
			Data: " and others",
		},
	}
	got := xref(input.Data, func(ref string) string { return ref })
	if err := cmpElems(input, got, want); err != nil {
		t.Fatalf("Unexpected xref() HTML result: %v", err)
	}

	want = []*html.Node{
		&html.Node{
			Type: html.TextNode,
			Data: "more details can be found in systemd.service(5), ",
		},
		a2,
		&html.Node{
			Type: html.TextNode,
			Data: " and others",
		},
	}
	got = xref(input.Data, func(ref string) string {
		if ref == "systemd.exec(5)" {
			return ref
		} else {
			return ""
		}
	})
	if err := cmpElems(input, got, want); err != nil {
		t.Fatalf("Unexpected xref() HTML result: %v", err)
	}

	want = []*html.Node{
		&html.Node{
			Type: html.TextNode,
			Data: "more details can be found in ",
		},
		a1,
		&html.Node{
			Type: html.TextNode,
			Data: ", systemd.exec(5) and others",
		},
	}
	got = xref(input.Data, func(ref string) string {
		if ref == "systemd.service(5)" {
			return ref
		} else {
			return ""
		}
	})
	if err := cmpElems(input, got, want); err != nil {
		t.Fatalf("Unexpected xref() HTML result: %v", err)
	}
}

func TestHref(t *testing.T) {
	input := &html.Node{
		Type: html.TextNode,
		Data: "more details can be found in systemd.service(5), http://debian.org/# and others",
	}

	a1 := &html.Node{
		Type: html.ElementNode,
		Data: "a",
		Attr: []html.Attribute{
			{Key: "href", Val: "systemd.service(5)"},
		},
	}
	a1.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: "systemd.service(5)",
	})

	a2 := &html.Node{
		Type: html.ElementNode,
		Data: "a",
		Attr: []html.Attribute{
			{Key: "href", Val: "http://debian.org/"},
		},
	}
	a2.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: "http://debian.org/#",
	})

	want := []*html.Node{
		&html.Node{
			Type: html.TextNode,
			Data: "more details can be found in ",
		},
		a1,
		&html.Node{
			Type: html.TextNode,
			Data: ", ",
		},
		a2,
		&html.Node{
			Type: html.TextNode,
			Data: " and others",
		},
	}
	got := xref(input.Data, func(ref string) string { return ref })
	if err := cmpElems(input, got, want); err != nil {
		t.Fatalf("Unexpected xref() HTML result: %v", err)
	}
}

func TestXrefHref(t *testing.T) {
	input := &html.Node{
		Type: html.TextNode,
		Data: "more details can be found in http://debian.org/systemd.service(5)",
	}

	a1 := &html.Node{
		Type: html.ElementNode,
		Data: "a",
		Attr: []html.Attribute{
			{Key: "href", Val: "http://debian.org/systemd.service(5)"},
		},
	}
	a1.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: "http://debian.org/systemd.service(5)",
	})

	want := []*html.Node{
		&html.Node{
			Type: html.TextNode,
			Data: "more details can be found in ",
		},
		a1,
	}
	got := xref(input.Data, func(ref string) string { return ref })
	if err := cmpElems(input, got, want); err != nil {
		t.Fatalf("Unexpected xref() HTML result: %v", err)
	}
}

func formattedXrefInput() *html.Node {
	input := &html.Node{
		Type: html.ElementNode,
		Data: "p",
	}
	input.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: "more details can be found in ",
	})
	b := &html.Node{
		Type: html.ElementNode,
		Data: "b"}
	b.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: "systemd.service"})
	input.AppendChild(b)
	input.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: "(5) and ",
	})
	i := &html.Node{
		Type: html.ElementNode,
		Data: "i"}
	i.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: "systemd.exec"})
	input.AppendChild(i)
	input.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: "(5)",
	})
	return input
}

func TestFormattedXref(t *testing.T) {
	input := formattedXrefInput()

	a1 := &html.Node{
		Type: html.ElementNode,
		Data: "a",
		Attr: []html.Attribute{
			{Key: "href", Val: "systemd.service(5)"},
		},
	}
	a1.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: "systemd.service(5)",
	})

	a2 := &html.Node{
		Type: html.ElementNode,
		Data: "a",
		Attr: []html.Attribute{
			{Key: "href", Val: "systemd.exec(5)"},
		},
	}
	a2.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: "systemd.exec(5)",
	})

	p := &html.Node{
		Type: html.ElementNode,
		Data: "p",
	}
	p.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: "more details can be found in ",
	})
	p.AppendChild(a1)
	p.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: " and ",
	})
	p.AppendChild(a2)
	want := []*html.Node{p}

	got := formattedXrefInput()
	if err := recurse(got, func(n *html.Node) error { return postprocess(func(ref string) string { return ref }, n, nil) }); err != nil {
		t.Fatal(err)
	}
	if err := cmpElems(input, []*html.Node{got}, want); err != nil {
		t.Fatalf("Unexpected xref() HTML result: %v", err)
	}
}
