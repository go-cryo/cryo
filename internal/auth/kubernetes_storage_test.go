package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	basicauth "github.com/mxcd/go-basicauth"
	"k8s.io/client-go/kubernetes/fake"
)

func TestKubernetesStorage_CRUD(t *testing.T) {
	cs := fake.NewSimpleClientset()
	store := NewKubernetesStorage(cs, "cryo", "cryo-admin-credentials")

	username := "admin"
	id := uuid.New()
	user := &basicauth.User{
		ID:           id,
		Username:     &username,
		PasswordHash: "hash-1",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if _, err := store.GetUserByUsername("admin"); err != basicauth.ErrUserNotFound {
		t.Fatalf("before create: expected ErrUserNotFound, got %v", err)
	}

	if err := store.CreateUser(user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := store.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if got.ID != id || got.PasswordHash != "hash-1" {
		t.Fatalf("unexpected user: %+v", got)
	}

	byID, err := store.GetUserByID(id)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if byID.Username == nil || *byID.Username != "admin" {
		t.Fatalf("GetUserByID returned unexpected username")
	}

	if _, err := store.GetUserByUsername("someone-else"); err != basicauth.ErrUserNotFound {
		t.Fatalf("wrong username: expected ErrUserNotFound, got %v", err)
	}

	user.PasswordHash = "hash-2"
	user.UpdatedAt = time.Now()
	if err := store.UpdateUser(user); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if got, _ = store.GetUserByUsername("admin"); got.PasswordHash != "hash-2" {
		t.Fatalf("password hash not updated: %q", got.PasswordHash)
	}

	if err := store.DeleteUser(id); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := store.GetUserByUsername("admin"); err != basicauth.ErrUserNotFound {
		t.Fatalf("after delete: expected ErrUserNotFound, got %v", err)
	}
}
