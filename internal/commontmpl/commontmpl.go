package commontmpl

import (
	"html/template"
	"strings"
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

func MustParseCommonTmpls() *template.Template {
	t := template.New("root")
	t = template.Must(t.New("header").Parse(bundled.Asset("header.tmpl")))
	t = template.Must(t.New("footer").
		Funcs(map[string]interface{}{
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
			"Now": func() string {
				return time.Now().UTC().Format(iso8601Format)
			}}).
		Parse(bundled.Asset("footer.tmpl")))
	t = template.Must(t.New("style").Parse(bundled.Asset("style.css")))
	return t
}
