package bot

import (
	"fmt"
)

// humanBytes renders n as a human-readable size using 1024-based units.
//
// The output has at most one decimal place for KB/MB/GB/TB and no decimal
// for B.
func humanBytes(n int64) string {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
		tib = 1024 * gib
	)
	switch {
	case n < kib:
		return fmt.Sprintf("%d B", n)
	case n < mib:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(kib))
	case n < gib:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mib))
	case n < tib:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(gib))
	default:
		return fmt.Sprintf("%.1f TB", float64(n)/float64(tib))
	}
}

// limitHuman renders a quota (LimitBytes) with the convention that 0 means
// unlimited.
func limitHuman(b *Bot, lang string, bytes int64) string {
	if bytes <= 0 {
		return b.deps.I18n.T(lang, "wizard.add.limit_unlimited")
	}
	return humanBytes(bytes)
}

// percent returns the integer percentage of used over limit. Returns 0 when
// limit is zero (unlimited).
func percent(used, limit int64) int {
	if limit <= 0 {
		return 0
	}
	p := (used * 100) / limit
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return int(p)
}

