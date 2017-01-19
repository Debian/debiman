package sitemap

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"time"
)

type url struct {
	XMLName xml.Name `xml:"url"`
	Loc     string   `xml:"loc"`
	Lastmod string   `xml:"lastmod"`
}

type sitemap struct {
	XMLName xml.Name `xml:"sitemap"`
	Loc     string   `xml:"loc"`
	Lastmod string   `xml:"lastmod"`
}

const sitemapDateFormat = "2006-01-02"

func WriteTo(w io.Writer, baseUrl string, contents map[string]time.Time) error {
	if _, err := w.Write([]byte(xml.Header)); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)

	start := xml.StartElement{
		Name: xml.Name{Local: "urlset"},
		Attr: []xml.Attr{
			xml.Attr{
				Name:  xml.Name{Local: "xmlns"},
				Value: "http://www.sitemaps.org/schemas/sitemap/0.9",
			},
		}}

	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	pkgs := make([]string, 0, len(contents))
	for binarypkg := range contents {
		pkgs = append(pkgs, binarypkg)
	}
	sort.Strings(pkgs)
	for _, binarypkg := range pkgs {
		if err := enc.EncodeElement(&url{
			Loc:     fmt.Sprintf("%s/%s/index.html", baseUrl, binarypkg),
			Lastmod: contents[binarypkg].Format(sitemapDateFormat),
		}, xml.StartElement{Name: xml.Name{Local: "url"}}); err != nil {
			return err
		}
	}
	if err := enc.EncodeToken(xml.EndElement{Name: start.Name}); err != nil {
		return err
	}

	return enc.Flush()
}

func WriteIndexTo(w io.Writer, baseUrl string, contents map[string]time.Time) error {
	if _, err := w.Write([]byte(xml.Header)); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)

	start := xml.StartElement{
		Name: xml.Name{Local: "sitemapindex"},
		Attr: []xml.Attr{
			xml.Attr{
				Name:  xml.Name{Local: "xmlns"},
				Value: "http://www.sitemaps.org/schemas/sitemap/0.9",
			},
		}}

	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	pkgs := make([]string, 0, len(contents))
	for suite := range contents {
		pkgs = append(pkgs, suite)
	}
	sort.Strings(pkgs)
	for _, suite := range pkgs {
		if err := enc.EncodeElement(&sitemap{
			Loc:     fmt.Sprintf("%s/%s/sitemap.xml.gz", baseUrl, suite),
			Lastmod: contents[suite].Format(sitemapDateFormat),
		}, xml.StartElement{Name: xml.Name{Local: "sitemap"}}); err != nil {
			return err
		}
	}
	if err := enc.EncodeToken(xml.EndElement{Name: start.Name}); err != nil {
		return err
	}

	return enc.Flush()
}
