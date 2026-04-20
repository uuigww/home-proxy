package store

import "time"

// User is a proxy user persisted in the users table.
//
// VLESSUUID, SOCKSUser and SOCKSPass are pointers because a user may have only
// one of the two protocols enabled. LimitBytes = 0 means "no quota".
//
// MTProtoEnabled is a UI-only flag: when true, the bot exposes the shared
// MTProto link to this user. mtg uses a single server-wide secret (see
// MTGConfig) so this flag does not translate to any per-user enforcement.
type User struct {
	ID             int64
	Name           string
	VLESSUUID      *string
	SOCKSUser      *string
	SOCKSPass      *string
	LimitBytes     int64
	UsedBytes      int64
	Enabled        bool
	MTProtoEnabled bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// AdminPrefs captures per-admin notification and locale preferences.
//
// The field layout matches the admin_prefs table in migration 001. See
// docs/notifications.md for the behavioural contract behind each toggle.
type AdminPrefs struct {
	TGID                 int64
	Lang                 string
	NotifyCritical       bool
	NotifyImportant      bool
	NotifyInfo           bool
	NotifyInfoOthersOnly bool
	NotifySecurity       bool
	NotifyDaily          bool
	NotifyNonadminSpam   bool
	DigestHour           int
	QuietFromHour        int
	QuietToHour          int
	UpdatedAt            time.Time
}

// Session is the bot's single-message UX state for one admin.
//
// WizardJSON is opaque to the store; the bot package serialises its current
// multi-step wizard state there and reloads it on the next callback.
type Session struct {
	TGID       int64
	ChatID     int64
	MessageID  int
	Screen     string
	WizardJSON string
	UpdatedAt  time.Time
}

// UsageDay is one day of traffic counters for a user, used for digests.
//
// Day is normalised to 00:00 UTC of the calendar day it represents.
type UsageDay struct {
	UserID        int64
	Day           time.Time
	UplinkBytes   int64
	DownlinkBytes int64
}

// RealityKeys is the singleton row from the reality_keys table.
type RealityKeys struct {
	PrivateKey string
	PublicKey  string
	ShortID    string
	Dest       string
	ServerName string
	CreatedAt  time.Time
}

// WarpPeer is the singleton row from the warp_peer table.
type WarpPeer struct {
	PrivateKey    string
	PeerPublicKey string
	IPv4          string
	IPv6          string
	Endpoint      string
	MTU           int
	UpdatedAt     time.Time
}

// MTGConfig is the singleton row from the mtg_config table.
//
// A single hex-encoded Fake-TLS secret is shared across all allowed MTProto
// users; revocation is achieved by rotating the secret (and restarting the
// mtg systemd unit). Port is the TCP port mtg binds to (default 8443);
// FakeTLSHost is the whitelisted SNI host mtg pretends to be.
type MTGConfig struct {
	Secret      string
	FakeTLSHost string
	Port        int
	UpdatedAt   time.Time
}

// DefaultNotifyFlags is the canonical default set of per-admin preferences.
//
// It is exposed as a value (not a function) so callers can compare against or
// spread it; use NewDefaultAdminPrefs when you need a ready-to-insert struct
// with TGID and a fresh UpdatedAt populated.
var DefaultNotifyFlags = AdminPrefs{
	Lang:            "ru",
	NotifyCritical:  true,
	NotifyImportant: true,
	NotifyInfo:      true,
	NotifySecurity:  true,
	NotifyDaily:     true,
	DigestHour:      9,
	QuietFromHour:   23,
	QuietToHour:     7,
}

// NewDefaultAdminPrefs returns DefaultNotifyFlags with TGID, Lang and
// UpdatedAt populated, ready for UpsertAdminPrefs.
//
// If lang is empty, "ru" is used.
func NewDefaultAdminPrefs(tgID int64, lang string) AdminPrefs {
	p := DefaultNotifyFlags
	p.TGID = tgID
	if lang != "" {
		p.Lang = lang
	}
	p.UpdatedAt = time.Now().UTC()
	return p
}
