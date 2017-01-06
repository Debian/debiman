package tag

import (
	"fmt"
	"strings"

	"golang.org/x/text/language"
)

// see https://wiki.openoffice.org/wiki/LocaleMapping#Best_mapping
// TODO: auto-create a mapping from “property value alias” to “code” in ISO-15924: http://www.unicode.org/iso15924/iso15924-codes.html. file a bug with x/text/language to include such a mapping?
// TODO(later): cover all codes listed in e.g. https://www.debian.org/international/l10n/po/?
var modifierToBCP47 = map[string]string{
	"euro":           "", // obsolete
	"cjknarrow":      "", // ?
	"valencia":       "-valencia",
	"latin":          "-Latn",
	"Latn":           "-Latn",
	"cyrillic":       "-Cyrl",
	"Cyrl":           "-Cyrl",
	"ijekavian":      "-ijekavsk",
	"ijekavianlatin": "-Latn-ijekavsk",
}

func FromLocale(l string) (language.Tag, error) {
	// Strip the codeset, if any
	if idx := strings.Index(l, "."); idx > -1 {
		lidx := strings.LastIndex(l, "@")
		if lidx == -1 {
			lidx = len(l)
		}
		l = l[:idx] + l[lidx:len(l)]
	}

	// Map the modifier to BCP-47 variant, if possible
	if idx := strings.Index(l, "@"); idx > -1 {
		modifier := l[idx+1:]
		mapping, ok := modifierToBCP47[modifier]
		if !ok {
			return language.Tag{}, fmt.Errorf("Unknown locale modifier %q, please report a bug", modifier)
		}
		l = l[:idx] + mapping
	}

	return language.Parse(l)
}
