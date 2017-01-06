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
		"1": true,
		"5": true,
	},

	Suites: map[string]bool{
		"testing":  true,
		"unstable": true,
		"sid":      true,

		// TODO: add a test, these are not indexed
		"experimental": true,
		"rc-buggy":     true,

		// These are loaded at runtime.
		"jessie":  true,
		"wheezy":  true,
		"stretch": true,

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
	},
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

	// TODO: suite redirects, e.g. stable → jessie
	table := []struct {
		Case int
		URL  string
		want string
	}{
		{Case: 1, URL: "i3", want: "jessie/i3-wm/i3.1.en.html"},
		{Case: 1, URL: "systemd.service", want: "jessie/systemd/systemd.service.5.en.html"},

		{Case: 2, URL: "i3.en", want: "jessie/i3-wm/i3.1.en.html"}, // default language
		{Case: 2, URL: "systemd.service.en", want: "jessie/systemd/systemd.service.5.en.html"},
		{Case: 2, URL: "i3.fr", want: "jessie/i3-wm/i3.1.fr.html"}, // non-default language

		{Case: 3, URL: "i3.1", want: "jessie/i3-wm/i3.1.en.html"}, // default section
		{Case: 3, URL: "systemd.service.5", want: "jessie/systemd/systemd.service.5.en.html"},
		{Case: 3, URL: "i3.5", want: "jessie/i3-wm/i3.5.en.html"}, // non-default section

		{Case: 4, URL: "i3.1.fr", want: "jessie/i3-wm/i3.1.fr.html"}, // default section
		{Case: 4, URL: "i3.5.fr", want: "jessie/i3-wm/i3.5.fr.html"}, // non-default section
		{Case: 4, URL: "systemd.service.5.en", want: "jessie/systemd/systemd.service.5.en.html"},

		{Case: 5, URL: "i3-wm/i3", want: "jessie/i3-wm/i3.1.en.html"},

		{Case: 6, URL: "i3-wm/i3.fr", want: "jessie/i3-wm/i3.1.fr.html"},

		{Case: 7, URL: "i3-wm/i3.1", want: "jessie/i3-wm/i3.1.en.html"}, // default section
		{Case: 7, URL: "i3-wm/i3.5", want: "jessie/i3-wm/i3.5.en.html"}, // non-default section

		{Case: 8, URL: "i3-wm/i3.1.fr", want: "jessie/i3-wm/i3.1.fr.html"}, // default section
		{Case: 8, URL: "i3-wm/i3.5.fr", want: "jessie/i3-wm/i3.5.fr.html"}, // non-default section

		{Case: 9, URL: "jessie/i3", want: "jessie/i3-wm/i3.1.en.html"},   // default suite
		{Case: 9, URL: "testing/i3", want: "testing/i3-wm/i3.1.en.html"}, // non-default suite

		{Case: 10, URL: "jessie/i3.fr", want: "jessie/i3-wm/i3.1.fr.html"},   // default suite
		{Case: 10, URL: "testing/i3.fr", want: "testing/i3-wm/i3.1.fr.html"}, // non-default suite

		{Case: 11, URL: "jessie/i3.1", want: "jessie/i3-wm/i3.1.en.html"},   // default suite, default section
		{Case: 11, URL: "testing/i3.5", want: "testing/i3-wm/i3.5.en.html"}, // non-default suite, non-default section

		{Case: 12, URL: "jessie/i3.1.fr", want: "jessie/i3-wm/i3.1.fr.html"},   // default suite, default section
		{Case: 12, URL: "testing/i3.5.fr", want: "testing/i3-wm/i3.5.fr.html"}, // non-default suite, non-default section

		{Case: 13, URL: "jessie/i3-wm/i3", want: "jessie/i3-wm/i3.1.en.html"},   // default suite
		{Case: 13, URL: "testing/i3-wm/i3", want: "testing/i3-wm/i3.1.en.html"}, // non-default suite

		{Case: 14, URL: "jessie/i3-wm/i3.fr", want: "jessie/i3-wm/i3.1.fr.html"},   // default suite
		{Case: 14, URL: "testing/i3-wm/i3.fr", want: "testing/i3-wm/i3.1.fr.html"}, // non-default suite

		{Case: 15, URL: "jessie/i3-wm/i3.1", want: "jessie/i3-wm/i3.1.en.html"},   // default suite, default section
		{Case: 15, URL: "testing/i3-wm/i3.5", want: "testing/i3-wm/i3.5.en.html"}, // non-default suite, non-default section

		{Case: 16, URL: "jessie/i3-wm/i3.1.fr", want: "jessie/i3-wm/i3.1.fr.html"},   // default suite
		{Case: 16, URL: "testing/i3-wm/i3.1.fr", want: "testing/i3-wm/i3.1.fr.html"}, // non-default suite
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
