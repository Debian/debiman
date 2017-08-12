package commontmpl

import (
	"flag"
	"html/template"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	"github.com/Debian/debiman/internal/bundled"
)

const iso8601Format = "2006-01-02T15:04:05Z"

var ambiguousLangs = map[string]bool{
	"cat": true, // català (ca, ca@valencia)
	"por": true, // português (pt, pt_BR)
	"zho": true, // 繁體中文 (zh_HK, zh_TW)
}

var (
	baseURLPath string
	baseURLOnce sync.Once
)

// BaseURLPath returns the path of the -base_url flag. E.g. “/sub” for
// “https://example.com/sub”, or “” for “https://manpages.debian.org”.
func BaseURLPath() string {
	baseURLOnce.Do(func() {
		u, err := url.Parse(flag.Lookup("base_url").Value.String())
		if err != nil {
			log.Fatalf("Invalid -base_url: %v", err)
		}
		baseURLPath = u.Path
	})
	return baseURLPath
}

func MustParseCommonTmpls() *template.Template {
	funcmap := map[string]interface{}{
		"DisplayLang": func(tag language.Tag) string {
			lang := display.Self.Name(tag)
			// Some languages are not present in the Unicode CLDR,
			// so we cannot express their name in their own
			// language. Fall back to English.
			if lang == "" {
				return display.English.Languages().Name(tag)
			}
			base, _ := tag.Base()
			if ambiguousLangs[base.ISO3()] {
				return lang + " (" + tag.String() + ")"
			}
			return lang

		},
		"EnglishLang": func(tag language.Tag) string {
			return display.English.Languages().Name(tag)
		},
		"HasSuffix": func(s, suffix string) bool {
			return strings.HasSuffix(s, suffix)
		},
		"HasPrefix": func(s, suffix string) bool {
			return strings.HasPrefix(s, suffix)
		},
		"BaseURLPath": func() string {
			return BaseURLPath()
		},
		"Now": func() string {
			return time.Now().UTC().Format(iso8601Format)
		}}

	t := template.New("root")
	t = template.Must(t.New("header").Funcs(funcmap).Parse(bundled.Asset("header.tmpl")))
	t = template.Must(t.New("footer").Funcs(funcmap).Parse(bundled.Asset("footer.tmpl")))
	t = template.Must(t.New("style").Parse(bundled.Asset("style.css")))
	return t
}
