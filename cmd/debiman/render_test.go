package main

import (
	"fmt"
	"testing"
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
