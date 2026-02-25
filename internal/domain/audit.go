package domain

import "time"

// AuditLog records every significant action in the system for compliance.
type AuditLog struct {
	ID         string    `json:"id"         db:"id"`
	UserID     string    `json:"user_id"    db:"user_id"`
	Action     string    `json:"action"     db:"action"`
	Resource   string    `json:"resource"   db:"resource"`
	ResourceID string    `json:"resource_id" db:"resource_id"`
	Details    string    `json:"details"    db:"details"` // JSON blob
	IP         string    `json:"ip"         db:"ip"`
	UserAgent  string    `json:"user_agent" db:"user_agent"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// Audit action constants.
const (
	AuditActionLogin       = "login"
	AuditActionLogout      = "logout"
	AuditActionRepoAccess  = "repo_access"
	AuditActionRepoClone   = "repo_clone"
	AuditActionAnalysisRun = "analysis_run"
	AuditActionRAGQuery    = "rag_query"
	AuditActionMCPCall     = "mcp_call"
)
