package bundled

import "strings"

// Asset returns either the bundled asset with the given name or the
// injected version (see the -inject_assets flag).
func Asset(basename string) string {
	// TODO: inject_assets

	return assets["assets/"+basename]
}

func AssetsFiltered(cb func(string) bool) map[string]string {
	result := make(map[string]string, len(assets))
	for fn, val := range assets {
		if !cb(strings.TrimPrefix(fn, "assets/")) {
			continue
		}
		result[fn] = val
	}
	return result
}
