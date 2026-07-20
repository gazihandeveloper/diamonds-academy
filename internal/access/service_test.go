package access

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database with the access_codes table
// matching the production schema from 0011_access_codes.up.sql.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS access_codes (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			code       TEXT    NOT NULL UNIQUE,
			is_active  INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_access_codes_active ON access_codes(is_active);
		CREATE INDEX IF NOT EXISTS idx_access_codes_code ON access_codes(code);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

// insertCode is a helper to insert a code row directly, used for edge-case setup.
func insertCode(t *testing.T, db *sql.DB, code string, active bool, createdAt, expiresAt time.Time) int64 {
	t.Helper()
	activeInt := 0
	if active {
		activeInt = 1
	}
	res, err := db.Exec(
		`INSERT INTO access_codes (code, is_active, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		code, activeInt, createdAt.UTC(), expiresAt.UTC(),
	)
	if err != nil {
		t.Fatalf("insert code %q: %v", code, err)
	}
	id, _ := res.LastInsertId()
	return id
}

// ─── SetCustomCode ───────────────────────────────────────────────────────────

func TestSetCustomCode_Happy(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	code, err := svc.SetCustomCode(ctx, "MANUEL-KOD")
	if err != nil {
		t.Fatalf("SetCustomCode() error = %v", err)
	}
	if code.Code != "MANUEL-KOD" {
		t.Errorf("code = %q, want %q", code.Code, "MANUEL-KOD")
	}
	if !code.IsActive {
		t.Error("new code should be active")
	}
	want := time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
	if !code.ExpiresAt.Equal(want) {
		t.Errorf("expires_at = %v, want %v", code.ExpiresAt, want)
	}
}

func TestSetCustomCode_TooShort(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	_, err := svc.SetCustomCode(ctx, "ABC")
	if err == nil {
		t.Error("expected error for code shorter than 4 chars")
	}
}

func TestSetCustomCode_MinLength(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	code, err := svc.SetCustomCode(ctx, "ABCD")
	if err != nil {
		t.Fatalf("SetCustomCode with 4-char code error = %v", err)
	}
	if !code.IsActive {
		t.Error("4-char code should be active")
	}
}

func TestSetCustomCode_DeactivatesPrevious(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	c1, err := svc.SetCustomCode(ctx, "ESKI-KOD")
	if err != nil {
		t.Fatalf("first SetCustomCode: %v", err)
	}

	c2, err := svc.SetCustomCode(ctx, "YENI-KOD")
	if err != nil {
		t.Fatalf("second SetCustomCode: %v", err)
	}

	// First code should now be inactive.
	codes, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, c := range codes {
		if c.ID == c1.ID && c.IsActive {
			t.Error("old code should be deactivated after creating new one")
		}
		if c.ID == c2.ID && !c.IsActive {
			t.Error("new code should be active")
		}
	}
}

func TestSetCustomCode_DuplicateCode(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	_, err := svc.SetCustomCode(ctx, "SAME-CODE")
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	// Second insert with same code should fail (UNIQUE constraint).
	_, err = svc.SetCustomCode(ctx, "SAME-CODE")
	if err == nil {
		t.Error("expected error for duplicate code")
	}
}

func TestSetCustomCode_VeryLong(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	longCode := strings.Repeat("X", 200)
	code, err := svc.SetCustomCode(ctx, longCode)
	if err != nil {
		t.Fatalf("SetCustomCode long code error = %v", err)
	}
	if code.Code != longCode {
		t.Errorf("code mismatch for long code")
	}
}

// ─── Validate ───────────────────────────────────────────────────────────────

func TestValidate_Happy(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	generated, err := svc.SetCustomCode(ctx, "VALIDATE-ME")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := svc.Validate(ctx, generated.Code)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got.Code != generated.Code {
		t.Errorf("code = %q, want %q", got.Code, generated.Code)
	}
	if got.ID != generated.ID {
		t.Errorf("id = %d, want %d", got.ID, generated.ID)
	}
	if !got.IsActive {
		t.Error("validated code should be active")
	}
}

func TestValidate_InvalidCode(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	_, err := svc.Validate(ctx, "DOESNOTEXIST")
	if err != ErrInvalidCode {
		t.Errorf("error = %v, want %v", err, ErrInvalidCode)
	}
}

func TestValidate_EmptyCode(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	_, err := svc.Validate(ctx, "")
	if err != ErrInvalidCode {
		t.Errorf("error = %v, want %v", err, ErrInvalidCode)
	}
}

func TestValidate_DeactivatedCode(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	generated, err := svc.SetCustomCode(ctx, "GOING-DOWN")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := svc.Deactivate(ctx, generated.ID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	_, err = svc.Validate(ctx, generated.Code)
	if err != ErrInvalidCode {
		t.Errorf("error = %v, want %v", err, ErrInvalidCode)
	}
}

func TestValidate_ExpiredCode(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	// Insert a code that expired 3 months ago — expiry is no longer checked.
	insertCode(t, db, "EXPIRED-KEY", true,
		time.Now().UTC().AddDate(0, -6, 0), // created 6 months ago
		time.Now().UTC().AddDate(0, -3, 0), // expired 3 months ago
	)

	got, err := svc.Validate(ctx, "EXPIRED-KEY")
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil (expiry no longer checked)", err)
	}
	if got.Code != "EXPIRED-KEY" {
		t.Errorf("code = %q, want %q", got.Code, "EXPIRED-KEY")
	}
}

func TestValidate_ExpiredButActive(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	// Active flag is 1, but expiry is in the past — expiry is no longer checked, so it should succeed.
	insertCode(t, db, "ACTIVE-EXPIRED", true,
		time.Now().UTC().AddDate(0, -12, 0),
		time.Now().UTC().AddDate(0, -1, 0),
	)

	got, err := svc.Validate(ctx, "ACTIVE-EXPIRED")
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil (expiry no longer checked)", err)
	}
	if got.Code != "ACTIVE-EXPIRED" {
		t.Errorf("code = %q, want %q", got.Code, "ACTIVE-EXPIRED")
	}
}

func TestValidate_BoundaryExpiry(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	// Expires 1 second in the future — should still be valid.
	insertCode(t, db, "EDGE-FUTURE", true,
		time.Now().UTC().AddDate(0, -3, 0),
		time.Now().UTC().Add(1*time.Second),
	)

	got, err := svc.Validate(ctx, "EDGE-FUTURE")
	if err != nil {
		t.Fatalf("code expiring in 1s should be valid: %v", err)
	}
	if got.Code != "EDGE-FUTURE" {
		t.Errorf("code = %q, want %q", got.Code, "EDGE-FUTURE")
	}
}

func TestValidate_SQLInjection(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	// Classic SQL injection attempt.
	_, err := svc.Validate(ctx, "'; DROP TABLE access_codes; --")
	if err != ErrInvalidCode {
		t.Errorf("error = %v, want %v", err, ErrInvalidCode)
	}

	// UNION-based injection attempt.
	_, err = svc.Validate(ctx, "' UNION SELECT 1,2,3,4,5 --")
	if err != ErrInvalidCode {
		t.Errorf("error = %v, want %v", err, ErrInvalidCode)
	}

	// Verify the table still exists (no damage).
	codes, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("list after injection attempts: %v (possible SQL injection damage)", err)
	}
	_ = codes // table intact
}

func TestValidate_VeryLongCode(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	// 10 KB code string — well within SQLite text limits but unusual.
	longCode := strings.Repeat("X", 10000)
	_, err := svc.Validate(ctx, longCode)
	if err != ErrInvalidCode {
		t.Errorf("error = %v, want %v", err, ErrInvalidCode)
	}
}

func TestValidate_WhitespaceOnly(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	// Whitespace is not an empty string, but unlikely to match any code.
	_, err := svc.Validate(ctx, "    ")
	if err != ErrInvalidCode {
		t.Errorf("error = %v, want %v", err, ErrInvalidCode)
	}
}

// ─── List ───────────────────────────────────────────────────────────────────

func TestList_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	codes, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(codes) != 0 {
		t.Errorf("len = %d, want 0", len(codes))
	}
}

func TestList_MultipleOrdered(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	// Insert codes with small delays so created_at differs.
	svc.SetCustomCode(ctx, "LIST-1")
	time.Sleep(10 * time.Millisecond)
	svc.SetCustomCode(ctx, "LIST-2")
	time.Sleep(10 * time.Millisecond)
	svc.SetCustomCode(ctx, "LIST-3")

	codes, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(codes) != 3 {
		t.Fatalf("len = %d, want 3", len(codes))
	}
	// Order must be newest first (DESC).
	for i := 1; i < len(codes); i++ {
		if codes[i-1].CreatedAt.Before(codes[i].CreatedAt) {
			t.Errorf("codes[%d].CreatedAt (%v) < codes[%d].CreatedAt (%v) — not DESC",
				i-1, codes[i-1].CreatedAt, i, codes[i].CreatedAt)
		}
	}
}

func TestList_MixedActiveDeactivated(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	c1, _ := svc.SetCustomCode(ctx, "MIXED-1")
	c2, _ := svc.SetCustomCode(ctx, "MIXED-2")
	svc.Deactivate(ctx, c1.ID) // deactivate first

	codes, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(codes) != 2 {
		t.Fatalf("len = %d, want 2", len(codes))
	}
	// c2 (active, newer) should be first, c1 (deactivated) second.
	if codes[0].ID != c2.ID {
		t.Errorf("first code id = %d, want %d (newest active)", codes[0].ID, c2.ID)
	}
	if codes[1].IsActive {
		t.Error("deactivated code should have IsActive=false")
	}
}

// ─── Deactivate ─────────────────────────────────────────────────────────────

func TestDeactivate_Happy(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	code, err := svc.SetCustomCode(ctx, "DEACT-ME")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := svc.Deactivate(ctx, code.ID); err != nil {
		t.Fatalf("Deactivate() error = %v", err)
	}
	// Verify it no longer validates.
	_, err = svc.Validate(ctx, code.Code)
	if err != ErrInvalidCode {
		t.Errorf("validate after deactivate: error = %v, want %v", err, ErrInvalidCode)
	}
}

func TestDeactivate_NonExistent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	err := svc.Deactivate(ctx, 99999)
	if err != ErrCodeNotFound {
		t.Errorf("error = %v, want %v", err, ErrCodeNotFound)
	}
}

func TestDeactivate_AlreadyDeactivated(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	code, _ := svc.SetCustomCode(ctx, "DEACT-2X")
	svc.Deactivate(ctx, code.ID)

	// Second deactivation should return ErrCodeNotFound.
	err := svc.Deactivate(ctx, code.ID)
	if err != ErrCodeNotFound {
		t.Errorf("error = %v, want %v", err, ErrCodeNotFound)
	}
}

func TestDeactivate_ZeroID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	err := svc.Deactivate(ctx, 0)
	if err != ErrCodeNotFound {
		t.Errorf("error = %v, want %v", err, ErrCodeNotFound)
	}
}

func TestDeactivate_NegativeID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	err := svc.Deactivate(ctx, -1)
	if err != ErrCodeNotFound {
		t.Errorf("error = %v, want %v", err, ErrCodeNotFound)
	}
}

// ─── Activate ───────────────────────────────────────────────────────────────

func TestActivate_Happy(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	code, _ := svc.SetCustomCode(ctx, "ACTIVATE-ME")
	svc.Deactivate(ctx, code.ID)

	if err := svc.Activate(ctx, code.ID); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	// Verify it validates again.
	got, err := svc.Validate(ctx, code.Code)
	if err != nil {
		t.Fatalf("validate after activate: %v", err)
	}
	if !got.IsActive {
		t.Error("code should be active after activation")
	}
}

func TestActivate_NonExistent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	err := svc.Activate(ctx, 99999)
	if err != ErrCodeNotFound {
		t.Errorf("error = %v, want %v", err, ErrCodeNotFound)
	}
}

func TestActivate_AlreadyActive(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	code, _ := svc.SetCustomCode(ctx, "ALREADY-ON")
	// Already active — Activate should return ErrCodeNotFound.
	err := svc.Activate(ctx, code.ID)
	if err != ErrCodeNotFound {
		t.Errorf("error = %v, want %v", err, ErrCodeNotFound)
	}
}

func TestActivate_ExpiredCode(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	// Insert a deactivated, expired code — expiry no longer blocks activation.
	id := insertCode(t, db, "OLD-DEACTIVATED", false,
		time.Now().UTC().AddDate(0, -6, 0),
		time.Now().UTC().AddDate(0, -3, 0),
	)

	err := svc.Activate(ctx, id)
	if err != nil {
		t.Fatalf("Activate() error = %v, want nil (expiry no longer blocks activation)", err)
	}
}

func TestActivate_ExpiredBoundary(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	// Expired 1 second ago — expiry no longer blocks activation.
	id := insertCode(t, db, "BARELY-EXPIRED", false,
		time.Now().UTC().AddDate(0, -1, 0),
		time.Now().UTC().Add(-1*time.Second),
	)

	err := svc.Activate(ctx, id)
	if err != nil {
		t.Fatalf("Activate() error = %v, want nil (expiry no longer blocks activation)", err)
	}
}

func TestActivate_NotExpiredBoundary(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	// Expires 10 seconds from now — SHOULD be activatable.
	id := insertCode(t, db, "BARELY-VALID", false,
		time.Now().UTC().AddDate(0, -1, 0),
		time.Now().UTC().Add(10*time.Second),
	)

	err := svc.Activate(ctx, id)
	if err != nil {
		t.Fatalf("Activate() error = %v, want nil", err)
	}
	// Verify active.
	got, _ := svc.Validate(ctx, "BARELY-VALID")
	if !got.IsActive {
		t.Error("code should be active")
	}
}

// ─── Integration-style flow ─────────────────────────────────────────────────

func TestFullLifecycle(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	// 1. Create custom code.
	code, err := svc.SetCustomCode(ctx, "LIFECYCLE")
	if err != nil {
		t.Fatalf("SetCustomCode: %v", err)
	}

	// 2. Validate — pass.
	if _, err := svc.Validate(ctx, code.Code); err != nil {
		t.Fatalf("validate 1: %v", err)
	}

	// 3. Deactivate.
	if err := svc.Deactivate(ctx, code.ID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	// 4. Validate after deactivate — fail.
	if _, err := svc.Validate(ctx, code.Code); err != ErrInvalidCode {
		t.Fatalf("validate after deactivate: %v", err)
	}

	// 5. Activate.
	if err := svc.Activate(ctx, code.ID); err != nil {
		t.Fatalf("activate: %v", err)
	}

	// 6. Validate after activate — pass.
	if _, err := svc.Validate(ctx, code.Code); err != nil {
		t.Fatalf("validate after activate: %v", err)
	}

	// 7. List — code appears.
	codes, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, c := range codes {
		if c.ID == code.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("code not found in list")
	}
}
