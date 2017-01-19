package sitemap

import (
	"bytes"
	"testing"
	"time"
)

func TestSitemap(t *testing.T) {
	const want = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://manpages.debian.org/jessie/pdns-recursor/index.html</loc><lastmod>2017-01-19</lastmod></url></urlset>`

	var gotb bytes.Buffer
	if err := WriteTo(&gotb, "https://manpages.debian.org/jessie", map[string]time.Time{
		"pdns-recursor": time.Unix(1484816329, 0),
	}); err != nil {
		t.Fatal(err)
	}

	if got := gotb.String(); got != want {
		t.Fatalf("unexpected sitemap contents: got %q, want %q", got, want)
	}
}

func TestSitemapIndex(t *testing.T) {
	const want = `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><sitemap><loc>https://manpages.debian.org/jessie/sitemap.xml.gz</loc><lastmod>2017-01-19</lastmod></sitemap></sitemapindex>`

	var gotb bytes.Buffer
	if err := WriteIndexTo(&gotb, "https://manpages.debian.org", map[string]time.Time{
		"jessie": time.Unix(1484816329, 0),
	}); err != nil {
		t.Fatal(err)
	}

	if got := gotb.String(); got != want {
		t.Fatalf("unexpected sitemap contents: got %q, want %q", got, want)
	}
}
