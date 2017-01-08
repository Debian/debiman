package aux

import (
	"net/http"
	"strings"

	"github.com/Debian/debiman/internal/redirect"
)

type Server struct {
	idx redirect.Index
}

func NewServer(idx redirect.Index) *Server {
	return &Server{
		idx: idx,
	}
}

func (s *Server) HandleRedirect(w http.ResponseWriter, r *http.Request) {
	redir, err := s.idx.Redirect(r)
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
