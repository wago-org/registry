package api

import (
	"testing"
	"time"

	"github.com/wago-org/registry-backend/internal/model"
)

// TestOwnsPackage covers the ownership branches that gate every manage endpoint
// (set publishers, deprecate, transfer, unpublish). The GitHub-call branch is
// exercised by seeding the repo-admin cache, so no network is needed.
func TestOwnsPackage(t *testing.T) {
	app := &App{}
	orgPkg := model.Package{
		Short:      "workers",
		OwnerLogin: "wago-org",
		Repository: "https://github.com/wago-org/workers",
	}

	if app.ownsPackage(nil, orgPkg) {
		t.Fatal("nil user must not own")
	}
	if !app.ownsPackage(&model.User{Login: "someone", Admin: true}, orgPkg) {
		t.Fatal("site admin must own any package")
	}
	if !app.ownsPackage(&model.User{Login: "wago-org"}, orgPkg) {
		t.Fatal("direct owner login must own")
	}
	if !app.ownsPackage(&model.User{Login: "WAGO-ORG"}, orgPkg) {
		t.Fatal("owner match must be case-insensitive")
	}
	if app.ownsPackage(&model.User{Login: "stranger"}, orgPkg) {
		t.Fatal("non-owner without a token must not own")
	}
	if app.ownsPackage(&model.User{Login: "x"}, model.Package{Short: "p"}) {
		t.Fatal("a package with no owner/repo must not be owned by a non-admin")
	}

	// Repo-admin path: an admin of the package's source repo may manage it even
	// though their login isn't the owner login. Seed the cache to stand in for the
	// GitHub RepoAccess call.
	admin := &model.User{Login: "jairussw", GitHubToken: "tok"}
	app.orgRoles.m = map[string]orgRoleEntry{
		"jairussw@wago-org/workers": {owner: true, expires: time.Now().Add(time.Hour)},
	}
	if !app.ownsPackage(admin, orgPkg) {
		t.Fatal("a GitHub admin of the source repo must own the package")
	}

	// A repo member without admin (cached false) must not manage it.
	app.orgRoles.m["jairussw@wago-org/workers"] = orgRoleEntry{owner: false, expires: time.Now().Add(time.Hour)}
	if app.ownsPackage(admin, orgPkg) {
		t.Fatal("a non-admin repo member must not own the package")
	}
}
