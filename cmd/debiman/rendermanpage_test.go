package main

import (
	"compress/gzip"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/Debian/debiman/internal/convert"
	"github.com/Debian/debiman/internal/manpage"
)

func mustParseFromServingPath(t *testing.T, path string) *manpage.Meta {
	m, err := manpage.FromServingPath("/srv/man", path)
	if err != nil {
		t.Fatal(err)
	}
	m.Package.Sourcepkg = m.Package.Binarypkg
	return m
}

func TestBestLanguageMatch(t *testing.T) {
	table := []struct {
		current         *manpage.Meta
		options         []*manpage.Meta
		wantServingPath string
	}{
		{
			current: mustParseFromServingPath(t, "testing/cron/crontab.1.fr"),
			options: []*manpage.Meta{
				mustParseFromServingPath(t, "testing/systemd-cron/crontab.5.fr"),
				mustParseFromServingPath(t, "testing/cron/crontab.5.fr"),
				mustParseFromServingPath(t, "testing/cron/crontab.5.en"),
			},
			wantServingPath: "testing/cron/crontab.5.fr",
		},
	}

	for _, entry := range table {
		entry := entry // capture
		t.Run(entry.wantServingPath, func(t *testing.T) {
			t.Parallel()
			best := bestLanguageMatch(entry.current, entry.options)
			if got, want := best.ServingPath(), entry.wantServingPath; got != want {
				t.Fatalf("Unexpected best language match: got %q, want %q", got, want)
			}
		})
	}
}

func TestPrep(t *testing.T) {
	const manContents = `.SH foobar
baz
.SH qux
`
	f, err := ioutil.TempFile("", "debiman-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	gzipw := gzip.NewWriter(f)
	if _, err := gzipw.Write([]byte(manContents)); err != nil {
		t.Fatal(err)
	}
	if err := gzipw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	manpagesFrExtra5 := mustParseFromServingPath(t, "jessie/manpages-fr-extra/crontab.5.fr")
	manpagesFrExtra1 := mustParseFromServingPath(t, "jessie/manpages-fr-extra/crontab.1.fr")
	manpagesJa := mustParseFromServingPath(t, "jessie/manpages-ja/crontab.5.ja")
	systemdCron := mustParseFromServingPath(t, "jessie/systemd-cron/crontab.5.en")
	cron := mustParseFromServingPath(t, "jessie/cron/crontab.5.en")
	bcronRun := mustParseFromServingPath(t, "jessie/bcron-run/crontab.5.en")
	// Pretend crontab.5.en moved to manpages-fr-systemd for testing issue #27
	manpagesFrSystemd := mustParseFromServingPath(t, "testing/manpages-fr-systemd/crontab.5.fr")
	manpagesFrSystemd.Package.Sourcepkg = manpagesFrExtra5.Package.Sourcepkg

	converter, err := convert.NewProcess()
	if err != nil {
		t.Fatal(err)
	}
	defer converter.Kill()

	_, data, err := rendermanpageprep(converter, renderJob{
		dest: f.Name(),
		src:  f.Name(),
		meta: manpagesFrExtra5,
		versions: []*manpage.Meta{
			manpagesFrExtra5,
			manpagesFrExtra1,
			manpagesJa,
			systemdCron,
			cron,
			bcronRun,
			manpagesFrSystemd,
		},
		xref: map[string][]*manpage.Meta{
			"crontab": []*manpage.Meta{
				manpagesFrExtra5,
				manpagesFrExtra1,
				manpagesJa,
				systemdCron,
				cron,
				bcronRun,
				manpagesFrSystemd,
			},
		},
		modTime: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("versions", func(t *testing.T) {
		wantSuites := []*manpage.Meta{
			manpagesFrExtra5,
			manpagesFrSystemd,
		}

		if got, want := len(data.Suites), len(wantSuites); got != want {
			t.Fatalf("unexpected number of data.Suites: got %d, want %d", got, want)
		}

		for i := 0; i < len(data.Suites); i++ {
			if got, want := data.Suites[i], wantSuites[i]; got != want {
				t.Fatalf("unexpected entry in data.Suites: got %v, want %v", got, want)
			}
		}
	})

	t.Run("lang", func(t *testing.T) {
		wantLang := []*manpage.Meta{
			systemdCron,
			cron,
			bcronRun,
			manpagesFrExtra5,
			manpagesJa,
		}

		if got, want := len(data.Langs), len(wantLang); got != want {
			t.Fatalf("unexpected number of data.Langs: got %d, want %d", got, want)
		}

		for i := 0; i < len(data.Langs); i++ {
			if got, want := data.Langs[i], wantLang[i]; got != want {
				t.Fatalf("unexpected entry in data.Langs: got %v, want %v", got, want)
			}
		}
	})

	t.Run("section", func(t *testing.T) {
		wantSections := []*manpage.Meta{
			manpagesFrExtra1,
			manpagesFrExtra5,
		}

		if got, want := len(data.Sections), len(wantSections); got != want {
			t.Fatalf("unexpected number of data.Sections: got %d, want %d", got, want)
		}

		for i := 0; i < len(data.Sections); i++ {
			if got, want := data.Sections[i], wantSections[i]; got != want {
				t.Fatalf("unexpected entry in data.Sections: got %v, want %v", got, want)
			}
		}
	})

	t.Run("ambiguous", func(t *testing.T) {
		wantAmbiguous := map[*manpage.Meta]bool{
			systemdCron: true,
			cron:        true,
			bcronRun:    true,
		}

		if got, want := len(data.Ambiguous), len(wantAmbiguous); got != want {
			t.Fatalf("unexpected number of data.Ambiguous: got %d, want %d", got, want)
		}

		for want := range wantAmbiguous {
			if _, ok := data.Ambiguous[want]; !ok {
				t.Fatalf("data.Ambiguous unexpectedly does not contain key %v", want)
			}
		}
	})
}
