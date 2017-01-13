package redirect

import (
	"net/http"
	"net/url"
	"testing"
)

var testIdx = Index{
	Langs: map[string]bool{
		"en": true,
		"fr": true,
	},

	Sections: map[string]bool{
		"1":     true,
		"3":     true,
		"3edit": true,
		"5":     true,
	},

	Suites: map[string]string{
		"testing":  "testing",
		"unstable": "unstable",
		"sid":      "sid",

		"experimental": "experimental",
		"rc-buggy":     "rc-buggy",

		// These are loaded at runtime.
		"jessie": "jessie",
		"stable": "jessie",

		"wheezy":    "wheezy",
		"oldstable": "wheezy",

		"stretch": "testing",

		// TODO: where can we get historical release names from?
	},

	Entries: map[string][]IndexEntry{
		"i3": []IndexEntry{
			{
				Suite:     "jessie",
				Binarypkg: "i3-wm",
				Section:   "1",
				Language:  "en",
			},

			{
				Suite:     "jessie",
				Binarypkg: "i3-wm",
				Section:   "5",
				Language:  "fr",
			},

			{
				Suite:     "jessie",
				Binarypkg: "i3-wm",
				Section:   "5",
				Language:  "en",
			},

			{
				Suite:     "jessie",
				Binarypkg: "i3-wm",
				Section:   "1",
				Language:  "fr",
			},

			{
				Suite:     "testing",
				Binarypkg: "i3-wm",
				Section:   "1",
				Language:  "en",
			},

			{
				Suite:     "testing",
				Binarypkg: "i3-wm",
				Section:   "1",
				Language:  "fr",
			},

			{
				Suite:     "testing",
				Binarypkg: "i3-wm",
				Section:   "5",
				Language:  "fr",
			},

			{
				Suite:     "testing",
				Binarypkg: "i3-wm",
				Section:   "5",
				Language:  "en",
			},
		},
		"systemd.service": []IndexEntry{
			{
				Suite:     "jessie",
				Binarypkg: "systemd",
				Section:   "5",
				Language:  "en",
			},
		},
		"editline": []IndexEntry{
			{
				Suite:     "jessie",
				Binarypkg: "libedit-dev",
				Section:   "3edit",
				Language:  "en",
			},
			{
				Suite:     "jessie",
				Binarypkg: "libeditline-dev",
				Section:   "3",
				Language:  "en",
			},
		},
		"javafxpackager": []IndexEntry{
			{
				Suite:     "testing",
				Binarypkg: "openjfx",
				Section:   "1",
				Language:  "en",
			},
		},
		"dup": []IndexEntry{
			{
				Suite:     "jessie",
				Binarypkg: "manpages-pl-dev",
				Section:   "2",
				Language:  "pl",
			},
			{
				Suite:     "jessie",
				Binarypkg: "manpages-dev",
				Section:   "2",
				Language:  "en",
			},
		},
	},
}

func TestNotIndexed(t *testing.T) {
	u, err := url.Parse("http://man.debian.org/experimental/i3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testIdx.Redirect(&http.Request{URL: u}); err == nil {
		t.Fatalf("Redirect for /experimental/i3 unexpectedly succeeded")
	}
}

func TestUnderspecified(t *testing.T) {

	// man.debian.net/<obsolete-suite>/… → 404, mit manpage-übersicht

	// URLs match the following expression:
	// man.debian.net/(<suite>/)(<binarypkg/>)<name>(.<section>(.<lang>))

	// The following truth table outlines all possibilities we need to cover:
	//              suite  binarypkg section language
	// 01 contains                                     http://man.debian.org/i3
	// 02 contains                             t       http://man.debian.org/i3.fr
	// 03 contains                     t               http://man.debian.org/i3.1
	// 04 contains                     t       t       http://man.debian.org/i3.1.fr

	// 05 contains             t                       http://man.debian.org/i3-wm/i3
	// 06 contains             t               t       http://man.debian.org/i3-wm/i3.fr
	// 07 contains             t       t               http://man.debian.org/i3-wm/i3.1
	// 08 contains             t       t       t       http://man.debian.org/i3-wm/i3.1.fr

	// 09 contains   t                                 http://man.debian.org/testing/i3
	// 10 contains   t                         t       http://man.debian.org/testing/i3.fr
	// 11 contains   t                 t               http://man.debian.org/testing/i3.1
	// 12 contains   t                 t       t       http://man.debian.org/testing/i3.1.fr

	// 13 contains   t         t                       http://man.debian.org/testing/i3-wm/i3
	// 14 contains   t         t               t       http://man.debian.org/testing/i3-wm/i3.fr
	// 15 contains   t         t       t               http://man.debian.org/testing/i3-wm/i3.1
	// 16 contains   t         t       t       t       http://man.debian.org/testing/i3-wm/i3.1.fr

	table := []struct {
		Case int
		URL  string
		want string
	}{
		{Case: 1, URL: "i3", want: "jessie/i3-wm/i3.1.en.html"},
		{Case: 1, URL: "systemd.service", want: "jessie/systemd/systemd.service.5.en.html"},
		{Case: 1, URL: "javafxpackager", want: "testing/openjfx/javafxpackager.1.en.html"}, // not available in jessie

		{Case: 2, URL: "i3.en", want: "jessie/i3-wm/i3.1.en.html"}, // default language
		{Case: 2, URL: "systemd.service.en", want: "jessie/systemd/systemd.service.5.en.html"},
		{Case: 2, URL: "i3.fr", want: "jessie/i3-wm/i3.1.fr.html"}, // non-default language

		{Case: 3, URL: "i3.1", want: "jessie/i3-wm/i3.1.en.html"},  // default section
		{Case: 3, URL: "i3(1)", want: "jessie/i3-wm/i3.1.en.html"}, // default section
		{Case: 3, URL: "systemd.service.5", want: "jessie/systemd/systemd.service.5.en.html"},
		{Case: 3, URL: "systemd.service(5)", want: "jessie/systemd/systemd.service.5.en.html"},
		{Case: 3, URL: "i3.5", want: "jessie/i3-wm/i3.5.en.html"},                           // non-default section
		{Case: 3, URL: "editline.3", want: "jessie/libeditline-dev/editline.3.en.html"},     // section with subsection
		{Case: 3, URL: "editline.3edit", want: "jessie/libedit-dev/editline.3edit.en.html"}, // section with subsection

		{Case: 4, URL: "i3.1.fr", want: "jessie/i3-wm/i3.1.fr.html"},  // default section
		{Case: 4, URL: "i3.5.fr", want: "jessie/i3-wm/i3.5.fr.html"},  // non-default section
		{Case: 4, URL: "i3(5).fr", want: "jessie/i3-wm/i3.5.fr.html"}, // non-default section
		{Case: 4, URL: "systemd.service.5.en", want: "jessie/systemd/systemd.service.5.en.html"},
		{Case: 4, URL: "editline.3.en", want: "jessie/libeditline-dev/editline.3.en.html"}, // section with subsection

		{Case: 5, URL: "i3-wm/i3", want: "jessie/i3-wm/i3.1.en.html"},

		{Case: 6, URL: "i3-wm/i3.fr", want: "jessie/i3-wm/i3.1.fr.html"},

		{Case: 7, URL: "i3-wm/i3.1", want: "jessie/i3-wm/i3.1.en.html"},                             // default section
		{Case: 7, URL: "i3-wm/i3.5", want: "jessie/i3-wm/i3.5.en.html"},                             // non-default section
		{Case: 7, URL: "i3-wm/i3(5)", want: "jessie/i3-wm/i3.5.en.html"},                            // non-default section
		{Case: 7, URL: "libedit-dev/editline.3", want: "jessie/libedit-dev/editline.3edit.en.html"}, // section with subsection

		{Case: 8, URL: "i3-wm/i3.1.fr", want: "jessie/i3-wm/i3.1.fr.html"},                             // default section
		{Case: 8, URL: "i3-wm/i3.5.fr", want: "jessie/i3-wm/i3.5.fr.html"},                             // non-default section
		{Case: 8, URL: "i3-wm/i3(5).fr", want: "jessie/i3-wm/i3.5.fr.html"},                            // non-default section
		{Case: 8, URL: "i3-wm/i3(5)fr", want: "jessie/i3-wm/i3.5.fr.html"},                             // non-default section
		{Case: 8, URL: "libedit-dev/editline.3.en", want: "jessie/libedit-dev/editline.3edit.en.html"}, // section with subsection

		{Case: 9, URL: "jessie/i3", want: "jessie/i3-wm/i3.1.en.html"},   // default suite
		{Case: 9, URL: "testing/i3", want: "testing/i3-wm/i3.1.en.html"}, // non-default suite
		{Case: 9, URL: "stable/i3", want: "jessie/i3-wm/i3.1.en.html"},   // suite alias

		{Case: 10, URL: "jessie/i3.fr", want: "jessie/i3-wm/i3.1.fr.html"},   // default suite
		{Case: 10, URL: "testing/i3.fr", want: "testing/i3-wm/i3.1.fr.html"}, // non-default suite

		{Case: 11, URL: "jessie/i3.1", want: "jessie/i3-wm/i3.1.en.html"},                                   // default suite, default section
		{Case: 11, URL: "testing/i3.5", want: "testing/i3-wm/i3.5.en.html"},                                 // non-default suite, non-default section
		{Case: 11, URL: "jessie/libedit-dev/editline.3", want: "jessie/libedit-dev/editline.3edit.en.html"}, // section with subsection

		{Case: 12, URL: "jessie/i3.1.fr", want: "jessie/i3-wm/i3.1.fr.html"},                       // default suite, default section
		{Case: 12, URL: "testing/i3.5.fr", want: "testing/i3-wm/i3.5.fr.html"},                     // non-default suite, non-default section
		{Case: 12, URL: "jessie/editline.3.en", want: "jessie/libeditline-dev/editline.3.en.html"}, // section with subsection

		{Case: 13, URL: "jessie/i3-wm/i3", want: "jessie/i3-wm/i3.1.en.html"},   // default suite
		{Case: 13, URL: "testing/i3-wm/i3", want: "testing/i3-wm/i3.1.en.html"}, // non-default suite

		{Case: 14, URL: "jessie/i3-wm/i3.fr", want: "jessie/i3-wm/i3.1.fr.html"},   // default suite
		{Case: 14, URL: "testing/i3-wm/i3.fr", want: "testing/i3-wm/i3.1.fr.html"}, // non-default suite

		{Case: 15, URL: "jessie/i3-wm/i3.1", want: "jessie/i3-wm/i3.1.en.html"},                             // default suite, default section
		{Case: 15, URL: "testing/i3-wm/i3.5", want: "testing/i3-wm/i3.5.en.html"},                           // non-default suite, non-default section
		{Case: 15, URL: "jessie/libedit-dev/editline.3", want: "jessie/libedit-dev/editline.3edit.en.html"}, // section with subsection

		{Case: 16, URL: "jessie/i3-wm/i3.1.fr", want: "jessie/i3-wm/i3.1.fr.html"},                             // default suite
		{Case: 16, URL: "testing/i3-wm/i3.1.fr", want: "testing/i3-wm/i3.1.fr.html"},                           // non-default suite
		{Case: 16, URL: "jessie/libedit-dev/editline.3.en", want: "jessie/libedit-dev/editline.3edit.en.html"}, // section with subsection
	}
	for _, entry := range table {
		entry := entry // capture
		t.Run(entry.URL, func(t *testing.T) {
			t.Parallel()

			u, err := url.Parse("http://man.debian.org/" + entry.URL)
			if err != nil {
				t.Fatal(err)
			}
			req := &http.Request{
				URL: u,
			}
			got, err := testIdx.Redirect(req)
			if err != nil {
				t.Fatal(err)
			}
			want := "/" + entry.want
			if got != want {
				t.Fatalf("Unexpected redirect: got %q, want %q", got, want)
			}
		})
	}
}

func TestLegacyManpagesDebianOrgRedirects(t *testing.T) {
	// The following truth table outlines all possibilities we need to cover:
	// (numbers kept, binarypkg unsupported by legacy manpages.debian.org)
	//              suite  --------- section language
	// 01 contains                                     http://man.debian.org/i3
	// 02 contains                             t       http://man.debian.org/i3.fr
	// 03 contains                     t               http://man.debian.org/i3.1
	// 04 contains                     t       t       http://man.debian.org/i3.1.fr

	// 09 contains   t                                 http://man.debian.org/testing/i3
	// 10 contains   t                         t       http://man.debian.org/testing/i3.fr
	// 11 contains   t                 t               http://man.debian.org/testing/i3.1
	// 12 contains   t                 t       t       http://man.debian.org/testing/i3.1.fr
	table := []struct {
		Case int
		URL  string
		want string
	}{
		{Case: 1, URL: "man/i3", want: "jessie/i3-wm/i3.1.en.html"},

		{Case: 2, URL: "man/fr/i3", want: "jessie/i3-wm/i3.1.fr.html"},

		{Case: 3, URL: "man/1/i3", want: "jessie/i3-wm/i3.1.en.html"},
		{Case: 3, URL: "man1/i3", want: "jessie/i3-wm/i3.1.en.html"},
		{Case: 3, URL: "man5/i3", want: "jessie/i3-wm/i3.5.en.html"},
		{Case: 3, URL: "1/i3", want: "jessie/i3-wm/i3.1.en.html"},
		{Case: 3, URL: "5/i3", want: "jessie/i3-wm/i3.5.en.html"},

		{Case: 4, URL: "fr/man1/i3", want: "jessie/i3-wm/i3.1.fr.html"}, // default section
		{Case: 4, URL: "fr/man5/i3", want: "jessie/i3-wm/i3.5.fr.html"}, // non-default section

		{Case: 9, URL: "jessie/i3", want: "jessie/i3-wm/i3.1.en.html"},   // default suite
		{Case: 9, URL: "testing/i3", want: "testing/i3-wm/i3.1.en.html"}, // non-default suite

		{Case: 10, URL: "jessie/i3.fr", want: "jessie/i3-wm/i3.1.fr.html"},   // default suite
		{Case: 10, URL: "testing/i3.fr", want: "testing/i3-wm/i3.1.fr.html"}, // non-default suite

		{Case: 11, URL: "man/testing/5/i3", want: "testing/i3-wm/i3.5.en.html"},

		{Case: 12, URL: "man/testing/fr/5/i3", want: "testing/i3-wm/i3.5.fr.html"},
	}
	for _, entry := range table {
		entry := entry // capture
		t.Run(entry.URL, func(t *testing.T) {
			t.Parallel()

			u, err := url.Parse("http://man.debian.org/" + entry.URL)
			if err != nil {
				t.Fatal(err)
			}
			req := &http.Request{
				URL: u,
			}
			got, err := testIdx.Redirect(req)
			if err != nil {
				t.Fatal(err)
			}
			want := "/" + entry.want
			if got != want {
				t.Fatalf("Unexpected redirect: got %q, want %q", got, want)
			}
		})
	}
}

func TestAcceptLanguage(t *testing.T) {
	table := []struct {
		URL  string
		want string
		lang string
	}{
		{
			URL:  "i3",
			want: "jessie/i3-wm/i3.1.fr.html",
			lang: "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5",
		},
		{
			URL:  "dup",
			want: "jessie/manpages-dev/dup.2.en.html",
			lang: "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5",
		},
	}
	for _, entry := range table {
		entry := entry // capture
		t.Run(entry.URL, func(t *testing.T) {
			t.Parallel()

			u, err := url.Parse("http://man.debian.org/" + entry.URL)
			if err != nil {
				t.Fatal(err)
			}
			req := &http.Request{
				URL: u,
				Header: http.Header{
					"Accept-Language": []string{entry.lang},
				},
			}
			got, err := testIdx.Redirect(req)
			if err != nil {
				t.Fatal(err)
			}
			want := "/" + entry.want
			if got != want {
				t.Fatalf("Unexpected redirect: got %q, want %q", got, want)
			}
		})
	}
}

// // TODO: no longer supported releases result in an error page with a link to the oldest stable version
// {
// 	URL:  "http://man.debian.org/lenny/i3",
// 	want: "http://man.debian.org/wheezy/i3-wm/i3.1.en.html",
// },
