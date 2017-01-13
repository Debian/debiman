package redirect

import "strings"

func (i *Index) splitLegacy(path string) (suite string, binarypkg string, name string, section string, lang string) {
	parts := strings.Split(path[1:], "/")
	// /man/<name>
	if len(parts) == 2 {
		return "", "", parts[1], parts[0][len("man"):], ""
	}
	// /man/<section>/<name>
	// /man/<lang>/<name>
	if len(parts) == 3 {
		if i.Langs[parts[1]] {
			return "", "", parts[2], "", parts[1]
		} else if i.Sections[parts[1]] {
			return "", "", parts[2], parts[1], ""
		}
	}
	// /man/<suite>/<section>/<name>
	if len(parts) == 4 {
		return parts[1], "", parts[3], parts[2], ""
	}
	// /man/<suite>/<lang>/<section>/<name>
	if len(parts) == 5 {
		return parts[1], "", parts[4], parts[3], parts[2]
	}
	return "", "", "", "", ""
}
