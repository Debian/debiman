package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteManpage(t *testing.T) {
	table := []struct {
		src           string
		manpage       string
		want          string
		wantRefs      []string
		pkg           pkgEntry
		contentByPath map[string][]contentEntry
	}{
		{
			src:           "/usr/share/man/man1/noref.1",
			manpage:       "no ref in here\n",
			want:          "no ref in here\n",
			wantRefs:      nil,
			pkg:           pkgEntry{},
			contentByPath: make(map[string][]contentEntry),
		},

		{
			src:           "/usr/share/man/man1/unresolved.1",
			manpage:       ".so notfound.1\n",
			want:          "",
			wantRefs:      nil,
			pkg:           pkgEntry{},
			contentByPath: make(map[string][]contentEntry),
		},

		{
			src:      "/usr/share/man/man1/samepkg.1",
			manpage:  ".so man1/samepkg.1\n",
			want:     ".so jessie/bash/samepkg.1.en.gz\n",
			wantRefs: nil,
			pkg: pkgEntry{
				binarypkg: "bash",
				suite:     "jessie",
			},
			contentByPath: map[string][]contentEntry{
				"/usr/share/man/man1/samepkg.1.gz": []contentEntry{
					{
						binarypkg: "bash",
						suite:     "jessie",
					},
				},
			},
		},

		{
			src:     "/usr/share/man/man1/samepkgaux.1",
			manpage: ".so man1/samepkgaux.inc\n",
			want:    ".so jessie/bash/aux/usr/share/man/man1/samepkgaux.inc.gz\n",
			wantRefs: []string{
				"/usr/share/man/man1/samepkgaux.inc.gz",
			},
			pkg: pkgEntry{
				binarypkg: "bash",
				suite:     "jessie",
			},
			contentByPath: map[string][]contentEntry{
				"/usr/share/man/man1/samepkgaux.inc.gz": []contentEntry{
					{
						binarypkg: "bash",
						suite:     "jessie",
					},
				},
			},
		},

		{
			src:     "/usr/share/man/man1/samedir.1",
			manpage: ".so samedir.inc\n",
			want:    ".so jessie/bash/aux/usr/share/man/man1/samedir.inc.gz\n",
			wantRefs: []string{
				"/usr/share/man/man1/samedir.inc.gz",
			},
			pkg: pkgEntry{
				binarypkg: "bash",
				suite:     "jessie",
			},
			contentByPath: map[string][]contentEntry{
				"/usr/share/man/man1/samedir.inc.gz": []contentEntry{
					{
						binarypkg: "bash",
						suite:     "jessie",
					},
				},
			},
		},

		// example for an absolute path: isdnutils-base/isdnctrl.8.en.gz uses .so /usr/share/man/man8/.isdnctrl_conf.8
		{
			src:      "/usr/share/man/man1/absolute.1",
			manpage:  ".so /usr/share/man/man8/absolute.8\n",
			want:     ".so jessie/extra/absolute.8.en.gz\n",
			wantRefs: nil,
			pkg: pkgEntry{
				binarypkg: "bash",
				suite:     "jessie",
			},
			contentByPath: map[string][]contentEntry{
				"/usr/share/man/man8/absolute.8.gz": []contentEntry{
					{
						binarypkg: "extra",
						suite:     "jessie",
					},
				},
			},
		},

		{
			src:      "/usr/share/man/man1/absolutenotfound.1",
			manpage:  ".so /usr/share/man/man8/absolute.8\n",
			want:     "",
			wantRefs: nil,
			pkg: pkgEntry{
				binarypkg: "bash",
				suite:     "jessie",
			},
			contentByPath: map[string][]contentEntry{},
		},

		{
			src:      "/usr/share/man/fr/man7/bash-builtins.7",
			manpage:  ".so man1/bash.1\n",
			want:     ".so jessie/manpages-fr-extra/bash.1.fr.gz\n",
			wantRefs: nil,
			pkg: pkgEntry{
				binarypkg: "manpages-fr-extra",
				suite:     "jessie",
			},
			contentByPath: map[string][]contentEntry{
				"/usr/share/man/man1/bash.1.gz": []contentEntry{
					{
						binarypkg: "bash",
						suite:     "jessie",
					},
				},
				"/usr/share/man/fr/man1/bash.1.gz": []contentEntry{
					{
						binarypkg: "manpages-fr-extra",
						suite:     "jessie",
					},
				},
			},
		},
	}
	for _, entry := range table {
		entry := entry // capture
		t.Run(entry.src, func(t *testing.T) {
			t.Parallel()

			r := strings.NewReader(entry.manpage)
			var buf bytes.Buffer
			refs, err := soElim(entry.src, r, &buf, entry.pkg, entry.contentByPath)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := buf.String(), entry.want; got != want {
				t.Fatalf("Unexpected soElim() result: got %q, want %q", got, want)
			}
			if got, want := len(refs), len(entry.wantRefs); got != want {
				t.Fatalf("Unexpected number of soElim() ref results: got %d, want %d", got, want)
			}
			for i := 0; i < len(refs); i++ {
				if got, want := refs[i], entry.wantRefs[i]; got != want {
					t.Fatalf("soElim() ref differs in entry %d: got %q, want %q", i, got, want)
				}
			}
		})
	}
}
