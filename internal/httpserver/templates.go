package httpserver

import (
	"net/http"

	"github.com/YASSERRMD/FlowForge/internal/apierror"
	"github.com/YASSERRMD/FlowForge/internal/templates"
)

// handleListTemplates returns every built-in template with its graph for
// gallery preview. Templates are deterministic and small, so one endpoint
// serves both listing and preview.
func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	if IdentityFrom(r.Context()) == nil {
		apierror.Write(w, r, s.log, apierror.New(apierror.CodeUnauthorized, "authentication required"))
		return
	}
	apierror.JSON(w, http.StatusOK, map[string]any{"templates": templates.List()})
}
