package auxserver

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/Debian/debiman/internal/redirect"
)

var i3OnlyIdx = redirect.Index{
	Entries: map[string][]redirect.IndexEntry{
		"i3": []redirect.IndexEntry{
			{
				Name:      "i3",
				Suite:     "jessie",
				Binarypkg: "i3-wm",
				Section:   "1",
				Language:  "en",
			},
		},
	},
	Suites: map[string]string{
		"jessie": "jessie",
	},
	Langs: map[string]bool{
		"en": true,
	},
	Sections: map[string]bool{
		"1": true,
	},
}

func mustRedirectI3(t *testing.T, s *Server) {
	u, err := url.Parse("/i3")
	if err != nil {
		t.Fatal(err)
	}
	redir, err := s.redirect(&http.Request{URL: u})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := redir, "/jessie/i3-wm/i3.1.en.html"; got != want {
		t.Fatalf("Unexpected redirect for i3: got %q, want %q", got, want)
	}
}

func TestIndexSwapSucceed(t *testing.T) {
	t.Parallel()

	u, err := url.Parse("/w3m")
	if err != nil {
		t.Fatal(err)
	}

	s := NewServer(i3OnlyIdx, nil, "")
	mustRedirectI3(t, s)

	redir, err := s.redirect(&http.Request{URL: u})
	if err == nil {
		t.Fatal("redirect(/w3m) unexpectedly succeeded")
	}

	updatedIdx := redirect.Index{
		Entries: map[string][]redirect.IndexEntry{
			"i3": []redirect.IndexEntry{
				{
					Name:      "i3",
					Suite:     "jessie",
					Binarypkg: "i3-wm",
					Section:   "1",
					Language:  "en",
				},
			},

			"w3m": []redirect.IndexEntry{
				{
					Name:      "w3m",
					Suite:     "jessie",
					Binarypkg: "w3m",
					Section:   "1",
					Language:  "en",
				},
			},
		},
		Suites: map[string]string{
			"jessie": "jessie",
		},
		Langs: map[string]bool{
			"en": true,
		},
		Sections: map[string]bool{
			"1": true,
		},
	}

	if err := s.SwapIndex(updatedIdx); err != nil {
		t.Fatal(err)
	}

	mustRedirectI3(t, s)

	redir, err = s.redirect(&http.Request{URL: u})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := redir, "/jessie/w3m/w3m.1.en.html"; got != want {
		t.Fatalf("Unexpected redirect for w3m: got %q, want %q", got, want)
	}
}

func TestIndexSwapFail(t *testing.T) {
	t.Parallel()

	emptyIdx := redirect.Index{}

	s := NewServer(i3OnlyIdx, nil, "")
	mustRedirectI3(t, s)

	if err := s.SwapIndex(emptyIdx); err == nil {
		t.Fatal("SwapIndex(emptyIdx) unexpectedly succeeded")
	}

	mustRedirectI3(t, s)
}

func TestSuggest(t *testing.T) {
	s := NewServer(i3OnlyIdx, nil, "")
	for _, entry := range []struct {
		query string
		want  []string
	}{
		{
			query: "i",
			want:  []string{"i3.1"},
		},
		{
			query: "a",
			want:  nil,
		},
	} {
		if got, want := s.suggest(entry.query), entry.want; !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected result: got %v, want %v", got, want)
		}
	}
}

func BenchmarkSuggest(b *testing.B) {
	// TODO: load representative index
	s := NewServer(i3OnlyIdx, nil, "")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// TODO: run sub benchmarks for a few search terms
		s.suggest("i")
	}
}
