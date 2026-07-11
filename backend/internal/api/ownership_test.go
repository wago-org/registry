package api

import (
	"testing"
	"time"

	"github.com/wago-org/registry-backend/internal/model"
)

// TestOwnsPackage covers the ownership branches that gate every manage endpoint
// (set publishers, deprecate, transfer, unpublish). The GitHub-call branch is
// exercised by seeding the orgRoles cache, so no network is needed.
func TestOwnsPackage(t *testing.T) {
	app := &App{}
	orgPkg := model.Package{Short: "workers", OwnerLogin: "wago-org"}

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
		t.Fatal("a package with no owner must not be owned by a non-admin")
	}

	// Org-owner path: a member of wago-org whose cached GitHub role is owner/admin
	// may manage the org's package even though their login isn't "wago-org".
	member := &model.User{Login: "jairussw", GitHubToken: "tok"}
	app.orgRoles.m = map[string]orgRoleEntry{
		"jairussw@wago-org": {owner: true, expires: time.Now().Add(time.Hour)},
	}
	if !app.ownsPackage(member, orgPkg) {
		t.Fatal("a GitHub owner/admin of the org must own the org's package")
	}

	// A member who is not an org owner (cached false) must not manage it.
	app.orgRoles.m["jairussw@wago-org"] = orgRoleEntry{owner: false, expires: time.Now().Add(time.Hour)}
	if app.ownsPackage(member, orgPkg) {
		t.Fatal("a non-admin org member must not own the org's package")
	}
}
