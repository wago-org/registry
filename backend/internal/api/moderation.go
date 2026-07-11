package api

import (
	"strings"
	"sync"
	"time"

	"github.com/wago-org/registry-backend/internal/model"
)

// orgRoleTTL bounds how long a viewer's GitHub org role is trusted before we
// re-check it. Org ownership changes rarely, so caching for a few minutes keeps
// the comments endpoint from hitting GitHub on every page load.
const orgRoleTTL = 10 * time.Minute

type orgRoleEntry struct {
	owner   bool
	expires time.Time
}

// orgRoleCache memoizes "is viewer X an owner of org Y" lookups.
type orgRoleCache struct {
	mu sync.Mutex
	m  map[string]orgRoleEntry
}

// ownsPackage reports whether u may manage p — set publishers, deprecate,
// transfer, unpublish, moderate its discussion. True for the package's direct
// owner login, a site admin, or anyone with admin access to the package's source
// repository (org owners and repo admins). Repo-admin is the reliable signal: it
// needs no read:org scope (unlike GitHub org-membership), works with the login
// token we already hold, and is exactly what publishing checks.
func (a *App) ownsPackage(u *model.User, p model.Package) bool {
	if u == nil {
		return false
	}
	// Site admins manage everything.
	if u.Admin {
		return true
	}
	if p.OwnerLogin != "" && strings.EqualFold(u.Login, p.OwnerLogin) {
		return true
	}
	// Beyond the direct owner, admin on the package's source repo qualifies — an
	// org owner (or a repo admin) controls the source, hence the package.
	if u.GitHubToken == "" {
		return false
	}
	owner, repo, ok := parseGitHubRepo(p.Repository)
	if !ok {
		return false
	}
	return a.repoAdmin(u, owner, repo)
}

// canModeratePackage reports whether viewer may moderate p's discussion (e.g.
// hide comments). Ownership and moderation rights coincide, so this defers to
// ownsPackage.
func (a *App) canModeratePackage(viewer *model.User, p model.Package) bool {
	return a.ownsPackage(viewer, p)
}

// repoAdmin reports (and caches) whether u has admin access to owner/repo on
// GitHub — the signal that they control the package's source, hence the package.
// Cached for orgRoleTTL so a logged-in user viewing packages doesn't trigger a
// GitHub call per view.
func (a *App) repoAdmin(u *model.User, owner, repo string) bool {
	if u.GitHubToken == "" {
		return false
	}
	key := strings.ToLower(u.Login) + "@" + strings.ToLower(owner) + "/" + strings.ToLower(repo)
	now := time.Now()

	a.orgRoles.mu.Lock()
	if e, ok := a.orgRoles.m[key]; ok && now.Before(e.expires) {
		a.orgRoles.mu.Unlock()
		return e.owner
	}
	a.orgRoles.mu.Unlock()

	perm, _, err := a.GitHub.RepoAccess(u.GitHubToken, owner, repo)
	admin := err == nil && perm == "admin"

	a.orgRoles.mu.Lock()
	if a.orgRoles.m == nil {
		a.orgRoles.m = map[string]orgRoleEntry{}
	}
	a.orgRoles.m[key] = orgRoleEntry{owner: admin, expires: now.Add(orgRoleTTL)}
	a.orgRoles.mu.Unlock()
	return admin
}
