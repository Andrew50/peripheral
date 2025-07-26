package data

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
)

// Invite represents an invitation record
type Invite struct {
	Code      string    `json:"code"`
	PlanName  string    `json:"plan_name"`
	TrialDays int       `json:"trial_days"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateInvite creates a new invite with a random code
func CreateInvite(conn *Conn, planName string, trialDays int) (*Invite, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Generate a random 32-character code
	code, err := generateInviteCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate invite code: %v", err)
	}

	invite := &Invite{
		Code:      code,
		PlanName:  planName,
		TrialDays: trialDays,
		Used:      false,
		CreatedAt: time.Now(),
	}

	// Insert the invite
	_, err = ExecWithRetry(ctx, conn.DB, `
		INSERT INTO invites (code, plan_name, trial_days, used, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		invite.Code, invite.PlanName, invite.TrialDays, invite.Used, invite.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create invite: %v", err)
	}

	return invite, nil
}

// GetInviteByCode retrieves an invite by its code
func GetInviteByCode(conn *Conn, code string) (*Invite, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var invite Invite
	err := conn.DB.QueryRow(ctx, `
		SELECT code, plan_name, trial_days, used, created_at 
		FROM invites 
		WHERE code = $1`,
		code).Scan(&invite.Code, &invite.PlanName, &invite.TrialDays, &invite.Used, &invite.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("invite not found: %v", err)
	}

	return &invite, nil
}

// MarkInviteUsed marks an invite as used
func MarkInviteUsed(conn *Conn, code string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ExecWithRetry(ctx, conn.DB, `
		UPDATE invites 
		SET used = true 
		WHERE code = $1 AND used = false`,
		code)

	if err != nil {
		return fmt.Errorf("failed to mark invite as used: %v", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("invite not found or already used")
	}

	return nil
}

// MarkInviteUsedTx marks an invite as used within a transaction
func MarkInviteUsedTx(ctx context.Context, tx pgx.Tx, code string) error {
	result, err := tx.Exec(ctx, `
		UPDATE invites 
		SET used = true 
		WHERE code = $1 AND used = false`,
		code)

	if err != nil {
		return fmt.Errorf("failed to mark invite as used: %v", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("invite not found or already used")
	}

	return nil
}

// generateInviteCode generates a random 32-character hexadecimal code
func generateInviteCode() (string, error) {
	bytes := make([]byte, 16) // 16 bytes = 32 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
