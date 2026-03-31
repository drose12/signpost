package db

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("testpassword")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	// Verify it's a valid bcrypt hash
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("testpassword")); err != nil {
		t.Errorf("bcrypt hash verification failed: %v", err)
	}

	// Verify wrong password fails
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("wrong")); err == nil {
		t.Error("expected bcrypt verification to fail for wrong password")
	}
}

func TestCreateSMTPUser(t *testing.T) {
	db := testDB(t)

	hash, _ := HashPassword("testpass123")
	user, err := db.CreateSMTPUser("user@drcs.ca", hash)
	if err != nil {
		t.Fatalf("CreateSMTPUser: %v", err)
	}
	if user.Username != "user@drcs.ca" {
		t.Errorf("expected username 'user@drcs.ca', got %q", user.Username)
	}
	if !user.Active {
		t.Error("expected user to be active")
	}
	if user.ID == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestCreateDuplicateSMTPUser(t *testing.T) {
	db := testDB(t)

	hash, _ := HashPassword("testpass123")
	_, err := db.CreateSMTPUser("user@drcs.ca", hash)
	if err != nil {
		t.Fatalf("first CreateSMTPUser: %v", err)
	}

	_, err = db.CreateSMTPUser("user@drcs.ca", hash)
	if err == nil {
		t.Error("expected error creating duplicate SMTP user")
	}
}

func TestListSMTPUsers(t *testing.T) {
	db := testDB(t)

	// Empty initially
	users, err := db.ListSMTPUsers()
	if err != nil {
		t.Fatalf("ListSMTPUsers: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}

	// Create two users
	hash1, _ := HashPassword("testpass123")
	hash2, _ := HashPassword("otherpass123")
	db.CreateSMTPUser("alice@drcs.ca", hash1)
	db.CreateSMTPUser("bob@drcs.ca", hash2)

	users, err = db.ListSMTPUsers()
	if err != nil {
		t.Fatalf("ListSMTPUsers after create: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

func TestGetSMTPUser(t *testing.T) {
	db := testDB(t)

	hash, _ := HashPassword("testpass123")
	created, _ := db.CreateSMTPUser("user@drcs.ca", hash)

	got, err := db.GetSMTPUser(created.ID)
	if err != nil {
		t.Fatalf("GetSMTPUser: %v", err)
	}
	if got == nil {
		t.Fatal("expected user, got nil")
	}
	if got.Username != "user@drcs.ca" {
		t.Errorf("expected username 'user@drcs.ca', got %q", got.Username)
	}

	// Get nonexistent
	got, err = db.GetSMTPUser(999)
	if err != nil {
		t.Fatalf("GetSMTPUser nonexistent: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent user")
	}
}

func TestDeleteSMTPUser(t *testing.T) {
	db := testDB(t)

	hash, _ := HashPassword("testpass123")
	user, _ := db.CreateSMTPUser("user@drcs.ca", hash)

	err := db.DeleteSMTPUser(user.ID)
	if err != nil {
		t.Fatalf("DeleteSMTPUser: %v", err)
	}

	got, _ := db.GetSMTPUser(user.ID)
	if got != nil {
		t.Error("expected user to be deleted")
	}

	// Delete nonexistent
	err = db.DeleteSMTPUser(999)
	if err == nil {
		t.Error("expected error deleting nonexistent user")
	}
}

func TestUpdateSMTPUserPassword(t *testing.T) {
	db := testDB(t)

	hash, _ := HashPassword("oldpassword1")
	user, _ := db.CreateSMTPUser("user@drcs.ca", hash)

	newHash, _ := HashPassword("newpassword1")
	err := db.UpdateSMTPUserPassword(user.ID, newHash)
	if err != nil {
		t.Fatalf("UpdateSMTPUserPassword: %v", err)
	}

	// Verify new hash
	got, _ := db.GetSMTPUser(user.ID)
	if err := bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("newpassword1")); err != nil {
		t.Error("new password hash verification failed")
	}

	// Update nonexistent
	err = db.UpdateSMTPUserPassword(999, newHash)
	if err == nil {
		t.Error("expected error updating nonexistent user")
	}
}

func TestCountSMTPUsers(t *testing.T) {
	db := testDB(t)

	count, err := db.CountSMTPUsers()
	if err != nil {
		t.Fatalf("CountSMTPUsers: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	hash, _ := HashPassword("testpass123")
	db.CreateSMTPUser("user@drcs.ca", hash)

	count, _ = db.CountSMTPUsers()
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

func TestPortEnableSettings(t *testing.T) {
	db := testDB(t)

	// Verify migration 3 added the settings
	val, err := db.GetSetting("smtp_enabled")
	if err != nil {
		t.Fatalf("GetSetting smtp_enabled: %v", err)
	}
	if val != "true" {
		t.Errorf("expected smtp_enabled='true', got %q", val)
	}

	val, err = db.GetSetting("submission_enabled")
	if err != nil {
		t.Fatalf("GetSetting submission_enabled: %v", err)
	}
	if val != "false" {
		t.Errorf("expected submission_enabled='false', got %q", val)
	}
}
