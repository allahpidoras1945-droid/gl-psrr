package domain

import "time"

// ContactInfo stores raw extracted contact handles.
type ContactInfo struct {
	Emails   []string `json:"emails"`
	Skype    []string `json:"skype"`
	Discord  []string `json:"discord"`
	LinkedIn []string `json:"linkedin"`
	Twitter  []string `json:"twitter"`
	Telegram []string `json:"telegram"`
}

type TGStatus string

const (
	TGStatusValid     TGStatus = "VALID"
	TGStatusDeleted   TGStatus = "DELETED"
	TGStatusInvalid   TGStatus = "INVALID"
	TGStatusNotFound  TGStatus = "NOT_FOUND"
	TGStatusFloodWait TGStatus = "FLOOD_WAIT"
	TGStatusSkipped   TGStatus = "SKIPPED"
)

type TGValidationResult struct {
	Username    string    `json:"username"`
	Status      TGStatus  `json:"status"`
	UserID      int64     `json:"user_id,omitempty"`
	IsBot       bool      `json:"is_bot"`
	IsDeleted   bool      `json:"is_deleted"`
	WasVerified bool      `json:"was_verified"`
	ErrorReason string    `json:"error_reason,omitempty"`
	CheckedAt   time.Time `json:"checked_at"`
}

type AffiliateManager struct {
	Name     string   `json:"name"`
	Emails   []string `json:"emails,omitempty"`
	Skype    []string `json:"skype,omitempty"`
	Telegram []string `json:"telegram,omitempty"`
}

type NetworkMetadata struct {
	CommissionType   string             `json:"commission_type,omitempty"`
	PaymentFrequency string             `json:"payment_frequency,omitempty"`
	MinimumPayout    string             `json:"minimum_payout,omitempty"`
	ReferralRate     string             `json:"referral_rate,omitempty"`
	Managers         []AffiliateManager `json:"managers,omitempty"`
}

type Lead struct {
	ID          string               `json:"id"`
	SourceURL   string               `json:"source_url"`
	CompanyName string               `json:"company_name"`
	RawName     string               `json:"raw_name"`
	Contacts    ContactInfo          `json:"contacts"`
	IsCIS       bool                 `json:"is_cis"`
	CISReason   string               `json:"cis_reason,omitempty"`
	TGResults   []TGValidationResult `json:"tg_results,omitempty"`
	Network     *NetworkMetadata     `json:"network_metadata,omitempty"`
	MXValid     bool                 `json:"mx_valid"`
	CreatedAt   time.Time            `json:"created_at"`
}
