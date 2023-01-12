package auxserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/Debian/debiman/internal/commontmpl"
	"github.com/Debian/debiman/internal/manpage"
	"github.com/Debian/debiman/internal/redirect"
)

type Server struct {
	idx            redirect.Index
	idxMu          sync.RWMutex
	notFoundTmpl   *template.Template
	debimanVersion string
	sortedNames    []string
}

func NewServer(idx redirect.Index, notFoundTmpl *template.Template, debimanVersion string) *Server {
	s := &Server{
		idx:            idx,
		notFoundTmpl:   notFoundTmpl,
		debimanVersion: debimanVersion,
	}
	s.prepareSuggest()
	return s
}

// prepareSuggest sets sortedNames to a sorted slice of
// <name>.<section> strings found in idx.
func (s *Server) prepareSuggest() {
	names := make(map[string]bool)
	for name, entries := range s.idx.Entries {
		for _, entry := range entries {
			names[name+"."+entry.Section] = true
		}
	}

	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	s.sortedNames = result
}

func (s *Server) SwapIndex(idx redirect.Index) error {
	u, err := url.Parse("/i3")
	if err != nil {
		return err
	}
	redir, err := idx.Redirect(&http.Request{
		URL: u,
	})
	if err != nil {
		return fmt.Errorf("idx.Redirect: %v", err)
	}
	if !strings.HasSuffix(redir, "i3.1.en.html") {
		return fmt.Errorf("Redirect(/i3) does not lead to i3.1.en.html: got %q", redir)
	}
	s.idxMu.Lock()
	defer s.idxMu.Unlock()
	s.idx = idx
	s.prepareSuggest()
	return nil
}

func (s *Server) redirect(r *http.Request) (string, error) {
	s.idxMu.RLock()
	defer s.idxMu.RUnlock()
	return s.idx.Redirect(r)
}

func (s *Server) HandleRedirect(w http.ResponseWriter, r *http.Request) {
	redir, err := s.redirect(r)
	if err != nil {
		if nf, ok := err.(*redirect.NotFoundError); ok {
			var buf bytes.Buffer
			err = s.notFoundTmpl.Execute(&buf, struct {
				Title          string
				DebimanVersion string
				Breadcrumbs    []string // incorrect type, but empty anyway
				FooterExtra    string
				Manpage        string
				BestChoice     redirect.IndexEntry
				Meta           *manpage.Meta
				HrefLangs      []*manpage.Meta
			}{
				Title:          "Not Found",
				DebimanVersion: s.debimanVersion,
				Manpage:        nf.Manpage,
				BestChoice:     nf.BestChoice,
			})
			if err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.WriteHeader(http.StatusNotFound)
				io.Copy(w, &buf)
				return
			}
			/* fallthrough */
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// StatusTemporaryRedirect (HTTP 307) means subsequent requests
	// should use the old URI, which is what we want — the redirect
	// target will likely change in the future.
	http.Redirect(w, r, commontmpl.BaseURLPath()+redir, http.StatusTemporaryRedirect)
}

func (s *Server) HandleJump(w http.ResponseWriter, r *http.Request) {
	q := r.FormValue("q")
	if strings.TrimSpace(q) == "" {
		http.Error(w, "No q= query parameter specified", http.StatusBadRequest)
		return
	}

	r.URL.Path = "/" + strings.TrimPrefix(q, commontmpl.BaseURLPath())
	s.HandleRedirect(w, r)
}

func (s *Server) suggest(q string) []string {
	s.idxMu.RLock()
	defer s.idxMu.RUnlock()

	i := sort.Search(len(s.sortedNames), func(i int) bool {
		return s.sortedNames[i] >= q
	})

	var result []string
	for i < len(s.sortedNames) {
		if strings.HasPrefix(s.sortedNames[i], q) {
			result = append(result, s.sortedNames[i])
		} else {
			break
		}
		i++
	}
	if len(result) > 10 {
		result = result[:10]
	}
	return result
}

func (s *Server) HandleSuggest(w http.ResponseWriter, r *http.Request) {
	q := r.FormValue("q")
	if strings.TrimSpace(q) == "" {
		http.Error(w, "No q= query parameter specified", http.StatusBadRequest)
		return
	}

	r.URL.Path = "/" + q
	completions := s.suggest(q)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode([]interface{}{
		q,
		completions,
	}); err != nil {
		http.Error(w, fmt.Sprintf("encoding response: %v", err), http.StatusInternalServerError)
		return
	}
	io.Copy(w, &buf)
}
