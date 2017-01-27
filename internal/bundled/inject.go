package bundled

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Inject overwrites bundled assets with versions from dir. Not all
// assets must be overwritten at once, i.e. just supplying a modified
// header.tmpl is perfectly fine.
func Inject(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	entries, err := f.Readdir(-1)
	if err != nil {
		return err
	}
	for _, fi := range entries {
		if !fi.Mode().IsRegular() {
			continue
		}
		fn := fi.Name()
		path := filepath.Join(dir, fn)
		b, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		if a, ok := assets["assets/"+fn]; !ok {
			log.Printf("Warning: injected asset %q does not overwrite any bundled asset (left-over file?)", fn)
		} else {
			log.Printf("Overwriting bundled asset %q (len %d) with %q (len %d)", fn, len(a), path, len(b))
		}
		assets["assets/"+fn] = string(b)
	}
	return nil
}

// Asset returns either the bundled asset with the given name or the
// injected version (see the -inject_assets flag).
func Asset(basename string) string {
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
