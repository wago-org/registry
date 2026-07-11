package api

import (
	"net/http"
	"strings"

	"github.com/wago-org/registry-backend/internal/httpx"
	"github.com/wago-org/registry-backend/internal/model"
)

// ownedPackage resolves the package named in the URL and checks that the current
// user may manage it — its owner (directly, or as an owner/admin of the GitHub
// org that owns it), or a site admin. On any failure it writes the response and
// returns ok=false.
func (a *App) ownedPackage(w http.ResponseWriter, r *http.Request) (model.Package, bool) {
	u := a.Sessions.CurrentUser(r)
	if u == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return model.Package{}, false
	}
	p, ok := a.Store.GetPackage(r.PathValue("name"))
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "package not found")
		return model.Package{}, false
	}
	if !a.ownsPackage(u, p) {
		httpx.WriteError(w, http.StatusForbidden, "not the package owner")
		return model.Package{}, false
	}
	return p, true
}

// decorateForViewer decorates p and adds a viewer-specific canManage flag
// (org-aware), so the frontend can surface owner controls for org owners/admins
// too, not just the literal owner login.
func (a *App) decorateForViewer(p model.Package, u *model.User) map[string]any {
	id := ""
	if u != nil {
		id = u.ID
	}
	m := a.decoratePackage(p, id)
	m["canManage"] = a.ownsPackage(u, p)
	return m
}

// handleUnpublishPackage removes an entire package (owner only).
func (a *App) handleUnpublishPackage(w http.ResponseWriter, r *http.Request) {
	p, ok := a.ownedPackage(w, r)
	if !ok {
		return
	}
	if err := a.Store.DeletePackage(p.Short); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "store error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "unpublished": p.Short})
}

// publishersRequest is the body of PUT /api/packages/{name}/publishers.
type publishersRequest struct {
	Publishers []string `json:"publishers"`
}

// handleSetPublishers sets the package's allowed publishers — extra GitHub logins
// (beyond the repo's author/admins) permitted to publish. Owner / admin only.
func (a *App) handleSetPublishers(w http.ResponseWriter, r *http.Request) {
	p, ok := a.ownedPackage(w, r)
	if !ok {
		return
	}
	var req publishersRequest
	if err := decodeJSON(w, r, &req, 1<<16); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	seen := map[string]bool{}
	out := []string{}
	for _, s := range req.Publishers {
		login := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "@"))
		// Skip blanks, the owner (always allowed), and duplicates.
		if login == "" || strings.EqualFold(login, p.OwnerLogin) {
			continue
		}
		if key := strings.ToLower(login); !seen[key] {
			seen[key] = true
			out = append(out, login)
		}
	}
	p.AllowedPublishers = out
	if err := a.Store.UpsertPackage(p); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "store error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.decorateForViewer(p, a.Sessions.CurrentUser(r)))
}

// transferRequest is the body of POST /api/packages/{name}/transfer.
type transferRequest struct {
	Owner string `json:"owner"`
}

// handleTransfer reassigns a package's owner login. The caller must currently
// manage the package, and must also control the destination — their own login,
// or a GitHub org they own/admin — so ownership can't be dumped onto an org they
// don't administer.
func (a *App) handleTransfer(w http.ResponseWriter, r *http.Request) {
	p, ok := a.ownedPackage(w, r)
	if !ok {
		return
	}
	u := a.Sessions.CurrentUser(r) // non-nil once ownedPackage passed
	var req transferRequest
	if err := decodeJSON(w, r, &req, 1<<16); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	target := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(req.Owner), "@"))
	if target == "" {
		httpx.WriteError(w, http.StatusBadRequest, "owner is required")
		return
	}
	// Already the owner (case-insensitive): nothing to do.
	if strings.EqualFold(target, p.OwnerLogin) {
		httpx.WriteJSON(w, http.StatusOK, a.decorateForViewer(p, u))
		return
	}
	// The caller must control the destination: their own login, or a GitHub org
	// they own/admin (verified with their token).
	if !strings.EqualFold(target, u.Login) {
		if u.GitHubToken == "" {
			httpx.WriteError(w, http.StatusForbidden, "re-authenticate to verify your access to "+target)
			return
		}
		if !a.viewerOwnsOrg(u, target) {
			httpx.WriteError(w, http.StatusForbidden, "you must be an owner or admin of "+target+" to transfer it there")
			return
		}
	}
	p.OwnerLogin = target
	if err := a.Store.UpsertPackage(p); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "store error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.decorateForViewer(p, u))
}

// handleUnpublishVersion removes a single version. If it was the last version,
// the whole package is removed. (owner only)
func (a *App) handleUnpublishVersion(w http.ResponseWriter, r *http.Request) {
	p, ok := a.ownedPackage(w, r)
	if !ok {
		return
	}
	target := r.PathValue("version")
	kept := make([]model.Version, 0, len(p.Versions))
	found := false
	for _, v := range p.Versions {
		if v.Version == target {
			found = true
			continue
		}
		kept = append(kept, v)
	}
	if !found {
		httpx.WriteError(w, http.StatusNotFound, "version not found")
		return
	}
	if len(kept) == 0 {
		if err := a.Store.DeletePackage(p.Short); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "store error")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "unpublished": p.Short})
		return
	}
	// Re-point "latest" at the newest remaining version (last in the slice).
	for i := range kept {
		kept[i].Latest = false
	}
	kept[len(kept)-1].Latest = true
	p.Versions = kept
	if err := a.Store.UpsertPackage(p); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "store error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.decorateForViewer(p, a.Sessions.CurrentUser(r)))
}

// deprecateRequest is the body of POST /api/packages/{name}/deprecate.
type deprecateRequest struct {
	Message string `json:"message"`
	Version string `json:"version"`
	Undo    bool   `json:"undo"`
}

// handleDeprecate marks a package (or a specific version) deprecated, or undoes
// it. (owner only)
func (a *App) handleDeprecate(w http.ResponseWriter, r *http.Request) {
	p, ok := a.ownedPackage(w, r)
	if !ok {
		return
	}
	var req deprecateRequest
	if err := decodeJSON(w, r, &req, 1<<16); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if v := strings.TrimSpace(req.Version); v != "" {
		found := false
		for i := range p.Versions {
			if p.Versions[i].Version == v {
				p.Versions[i].Deprecated = !req.Undo
				found = true
				break
			}
		}
		if !found {
			httpx.WriteError(w, http.StatusNotFound, "version not found")
			return
		}
	} else if req.Undo {
		p.DeprecatedMessage = ""
	} else {
		msg := strings.TrimSpace(req.Message)
		if msg == "" {
			msg = "This package is deprecated."
		}
		p.DeprecatedMessage = msg
	}

	if err := a.Store.UpsertPackage(p); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "store error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.decorateForViewer(p, a.Sessions.CurrentUser(r)))
}
