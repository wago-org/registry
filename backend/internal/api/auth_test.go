package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wago-org/registry-backend/internal/auth"
)

func TestMeAcceptsBrowserSession(t *testing.T) {
	app := newSessionApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(app.Sessions.WriteSessionCookie(auth.SessionState{
		Accounts: []string{"1", "2"},
		Active:   "2",
	}))
	res := httptest.NewRecorder()

	app.NewRouter().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", res.Code, res.Body.String())
	}
	if got := decodeLogin(t, res)["login"]; got != "bob" {
		t.Fatalf("login = %v, want bob", got)
	}
}

func TestMeAcceptsRegistryBearerToken(t *testing.T) {
	app := newSessionApp(t)
	token, _, err := app.Store.CreateToken("1", "test-cli")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()

	app.NewRouter().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", res.Code, res.Body.String())
	}
	body := decodeLogin(t, res)
	if got := body["login"]; got != "alice" {
		t.Fatalf("login = %v, want alice", got)
	}
	accounts, ok := body["accounts"].([]any)
	if !ok || len(accounts) != 1 {
		t.Fatalf("accounts = %#v, want the bearer identity only", body["accounts"])
	}
}

func TestMeRejectsInvalidBearerToken(t *testing.T) {
	app := newSessionApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	res := httptest.NewRecorder()

	app.NewRouter().ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body = %s", res.Code, res.Body.String())
	}
}

func TestRegistryDeviceAuthorizationRoutesAreRemoved(t *testing.T) {
	app := newSessionApp(t)
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/device"},
		{http.MethodPost, "/api/device/code"},
		{http.MethodPost, "/api/device/token"},
		{http.MethodPost, "/api/device/approve"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		res := httptest.NewRecorder()

		app.NewRouter().ServeHTTP(res, req)

		if res.Code != http.StatusNotFound {
			t.Errorf("%s %s status = %d, want 404", tc.method, tc.path, res.Code)
		}
	}
}
