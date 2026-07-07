package store

import (
	"testing"

	"github.com/wago-org/registry-backend/internal/model"
)

// TestPebblePersistRoundTrip writes one of every record kind, closes the store,
// reopens it (forcing a full loadAll reconstruction), and verifies everything
// survived — then that DeletePackage cascades durably.
func TestPebblePersistRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenPebble(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	if err := s.UpsertUser(model.User{ID: "u1", Login: "alice", Name: "Alice"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertPackage(model.Package{Short: "redis", Name: "github.com/acme/wago-redis"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetStar("redis", "u1", true); err != nil {
		t.Fatal(err)
	}
	rev, err := s.UpsertReview("redis", "u1", 5, "great")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SetVote(rev.ID, "u2", "up"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddComment("redis", "u1", "hello", ""); err != nil {
		t.Fatal(err)
	}
	_ = s.RecordInstall("redis", "2026-07-07")
	_ = s.RecordInstall("redis", "2026-07-07")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen: everything must reload from disk.
	s2, err := OpenPebble(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if _, ok := s2.GetUser("u1"); !ok {
		t.Error("user did not persist")
	}
	if _, ok := s2.GetPackage("redis"); !ok {
		t.Error("package did not persist")
	}
	if got := s2.StarCount("redis"); got != 1 {
		t.Errorf("StarCount=%d want 1", got)
	}
	if got := len(s2.ReviewsForPackage("redis")); got != 1 {
		t.Errorf("reviews=%d want 1", got)
	}
	if up, _ := s2.VoteTally(rev.ID); up != 1 {
		t.Errorf("upvotes=%d want 1", up)
	}
	if got := len(s2.CommentsForPackage("redis")); got != 1 {
		t.Errorf("comments=%d want 1", got)
	}
	if got := s2.InstallTotal("redis"); got != 2 {
		t.Errorf("installTotal=%d want 2", got)
	}

	// Delete cascades and persists.
	if err := s2.DeletePackage("redis"); err != nil {
		t.Fatal(err)
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
	s3, err := OpenPebble(dir)
	if err != nil {
		t.Fatalf("reopen 2: %v", err)
	}
	defer s3.Close()
	if _, ok := s3.GetPackage("redis"); ok {
		t.Error("package should be gone after delete")
	}
	if s3.StarCount("redis") != 0 || len(s3.ReviewsForPackage("redis")) != 0 ||
		len(s3.CommentsForPackage("redis")) != 0 || s3.InstallTotal("redis") != 0 {
		t.Error("delete did not cascade durably")
	}
	if up, _ := s3.VoteTally(rev.ID); up != 0 {
		t.Error("review votes should be gone after package delete")
	}
}
