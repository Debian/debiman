package manpage

import (
	"testing"

	"golang.org/x/text/language"
)

func TestManpageFromManPath(t *testing.T) {
	table := []struct {
		path            string
		pkg             PkgMeta
		wantLang        string
		wantSection     string
		wantServingPath string
	}{
		// Verify the codeset is stripped from the path.
		{
			path:            "bg.UTF-8/man6/hex-a-hop.6.gz",
			pkg:             PkgMeta{Binarypkg: "hex-a-hop", Suite: "testing"},
			wantLang:        "bg",
			wantSection:     "6",
			wantServingPath: "testing/hex-a-hop/hex-a-hop.6.bg",
		},

		// Verify the modifier is retained from the path.
		{
			path:            "ca@valencia/man1/deja-dup.1.gz",
			pkg:             PkgMeta{Binarypkg: "deja-dup", Suite: "testing"},
			wantLang:        "ca@valencia",
			wantSection:     "1",
			wantServingPath: "testing/deja-dup/deja-dup.1.ca@valencia",
		},

		// Verify the section is parsed correctly for files which do
		// not have a .gz extension.
		{
			path:            "man3/el_init.3",
			pkg:             PkgMeta{Binarypkg: "libedit-dev", Suite: "testing"},
			wantLang:        "en",
			wantSection:     "3",
			wantServingPath: "testing/libedit-dev/el_init.3.en",
		},
	}

	for _, entry := range table {
		entry := entry // capture
		t.Run(entry.path, func(t *testing.T) {
			t.Parallel()
			m, err := FromManPath(entry.path, entry.pkg)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := m.Language, entry.wantLang; got != want {
				t.Fatalf("Unexpected language: got %q, want %q", got, want)
			}
			if got, want := m.Section, entry.wantSection; got != want {
				t.Fatalf("Unexpected section: got %q, want %q", got, want)
			}
			if got, want := m.ServingPath(), entry.wantServingPath; got != want {
				t.Fatalf("Unexpected section: got %q, want %q", got, want)
			}
		})
	}
}

func TestLanguageTag(t *testing.T) {
	table := []struct {
		path    string
		pkg     PkgMeta
		wantTag language.Tag
	}{
		// Verify pt_BR can be parsed (text/language calls it pt-BR)
		{
			path:    "pt_BR/man1/deja-dup.1.gz",
			pkg:     PkgMeta{Binarypkg: "deja-dup", Suite: "testing"},
			wantTag: language.BrazilianPortuguese,
		},

		{
			path:    "pt_PT/man1/deja-dup.1.gz",
			pkg:     PkgMeta{Binarypkg: "deja-dup", Suite: "testing"},
			wantTag: language.EuropeanPortuguese,
		},

		{
			path:    "sr@latin/man1/ark.1.gz",
			pkg:     PkgMeta{Binarypkg: "kde-l10n-sr", Suite: "testing"},
			wantTag: language.SerbianLatin,
		},
	}

	for _, entry := range table {
		entry := entry // capture
		t.Run(entry.path, func(t *testing.T) {
			t.Parallel()
			m, err := FromManPath(entry.path, entry.pkg)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := m.LanguageTag, entry.wantTag; got != want {
				t.Fatalf("Unexpected language: got %q, want %q", got, want)
			}
		})
	}
}
