package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/Debian/debiman/internal/manpage"
)

func TestBreadcrumbsToJSON(t *testing.T) {
	const breadcrumbsJSON = `{"@context":"http://schema.org","@type":"BreadcrumbList","itemListElement":[{"@type":"ListItem","position":1,"item":{"@type":"Thing","@id":"/contents-jessie.html","name":"jessie"}},{"@type":"ListItem","position":2,"item":{"@type":"Thing","@id":"/jessie/i3-wm/index.html","name":"i3-wm"}},{"@type":"ListItem","position":3,"item":{"@type":"Thing","@id":"","name":"i3(1)"}}]}`

	const Suite = "jessie"
	const Binarypkg = "i3-wm"
	b := breadcrumbs{
		{fmt.Sprintf("/contents-%s.html", Suite), Suite},
		{fmt.Sprintf("/%s/%s/index.html", Suite, Binarypkg), Binarypkg},
		{"", "i3(1)"},
	}
	if got, want := string(b.ToJSON()), breadcrumbsJSON; got != want {
		fmt.Printf("%s\n", got)
		t.Fatalf("unexpected breadcrumbs JSON: got %q, want %q", got, want)
	}
}

// Ensure that section names containing unsafe characters like colons
// are properly handled (and do not result in ZgotmplZ values) on pages
// like https://manpages.debian.org/trixie/foot/foot.ini.5.en.html
func TestFragmentLinkWithColon(t *testing.T) {
	var buf bytes.Buffer
	err := manpageTmpl.ExecuteTemplate(&buf, "manpage", manpagePrepData{
		Meta: &manpage.Meta{
			Name:    "test",
			Section: "1",
			Package: &manpage.PkgMeta{
				Suite:     "testing",
				Binarypkg: "test",
			},
		},
		TOC: []string{"SECTION: main"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "ZgotmplZ") {
		t.Fatal("ZgotmplZ in output")
	}
}
