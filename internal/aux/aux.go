package aux

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/Debian/debiman/internal/redirect"
)

type Server struct {
	idx   redirect.Index
	idxMu sync.RWMutex
}

func NewServer(idx redirect.Index) *Server {
	return &Server{
		idx: idx,
	}
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if redir == r.URL.Path {
		http.Error(w, "The request path already identifies a fully qualified manpage, the request should have been handled by the webserver upstream of auxserver. Your webserver might be misconfigured.", http.StatusNotFound)
		return
	}

	// StatusTemporaryRedirect (HTTP 307) means subsequent requests
	// should use the old URI, which is what we want — the redirect
	// target will likely change in the future.
	http.Redirect(w, r, redir, http.StatusTemporaryRedirect)
}

func (s *Server) HandleJump(w http.ResponseWriter, r *http.Request) {
	q := r.FormValue("q")
	if strings.TrimSpace(q) == "" {
		http.Error(w, "No q= query parameter specified", http.StatusBadRequest)
		return
	}

	r.URL.Path = "/" + q
	s.HandleRedirect(w, r)
}
