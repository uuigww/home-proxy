package limits

// QuotaEvent enumerates quota-threshold transitions detected by
// ClassifyQuota.
type QuotaEvent int

// Quota transition values returned by ClassifyQuota.
const (
	// QuotaNone means no threshold was newly crossed in this update.
	QuotaNone QuotaEvent = iota
	// QuotaEighty means 80% was just crossed (but not 100%).
	QuotaEighty
	// QuotaFull means 100% was just crossed.
	QuotaFull
)

// Delta computes a non-negative byte delta between two successive
// Xray stats reads.
//
// Xray exposes monotonically increasing per-user counters that reset
// to zero when someone runs `xray api stats -reset` or when the
// process restarts. If curr < prev we treat this as a reset and
// attribute the full curr to the new period.
//
// Returns 0 when either value is negative (defensive: Xray should
// never report negative values, but we refuse to subtract junk).
func Delta(prev, curr int64) int64 {
	if prev < 0 || curr < 0 {
		return 0
	}
	if curr < prev {
		return curr
	}
	return curr - prev
}

// UsagePercent returns used/limit as an integer percent, clamped to
// 0..999 for display. A zero or negative limit is treated as "no
// quota" and returns 0 so callers never divide by zero.
func UsagePercent(used, limit int64) int {
	if limit <= 0 || used <= 0 {
		return 0
	}
	// Use 128-bit-ish math via float for safety, then clamp.
	p := (used * 100) / limit
	if p < 0 {
		return 0
	}
	if p > 999 {
		return 999
	}
	return int(p)
}

// ClassifyQuota decides whether (oldUsed -> newUsed) with the given
// limit just crossed the 80% or 100% threshold.
//
// It assumes the caller tracks "already fired" state for 80% outside
// this function: the return value is purely derived from the before/
// after numbers. Callers that want once-per-period semantics should
// consult a Dedup or an in-memory flag after reading the result.
func ClassifyQuota(oldUsed, newUsed, limit int64) QuotaEvent {
	if limit <= 0 {
		return QuotaNone
	}
	// 100% takes priority: if we crossed both in one update we only
	// report the more severe one (QuotaFull).
	if oldUsed < limit && newUsed >= limit {
		return QuotaFull
	}
	eighty := limit * 80 / 100
	if oldUsed < eighty && newUsed >= eighty {
		return QuotaEighty
	}
	return QuotaNone
}
