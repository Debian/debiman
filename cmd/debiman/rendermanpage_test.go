package main

import (
	"testing"

	"github.com/Debian/debiman/internal/manpage"
)

func mustParseFromServingPath(t *testing.T, path string) *manpage.Meta {
	m, err := manpage.FromServingPath("/srv/man", path)
	if err != nil {
		t.Fatal(err)
	}
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
