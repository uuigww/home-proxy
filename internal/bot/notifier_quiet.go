package bot

import (
	"context"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// flushQuietLoop runs until ctx is cancelled, emptying each admin's outbox
// when their quiet window ends. A 1-minute tick is more than enough given
// the 1-hour granularity of quiet windows.
func (n *Notifier) flushQuietLoop(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n.flushQuiet(ctx)
		}
	}
}

// flushQuiet drains outboxes for admins whose quiet window has ended.
func (n *Notifier) flushQuiet(ctx context.Context) {
	now := n.now()
	n.mu.Lock()
	var ready []int64
	for adminID := range n.outbox {
		p, err := n.prefsFor(ctx, adminID)
		if err != nil {
			continue
		}
		if !n.inQuiet(p, now) {
			ready = append(ready, adminID)
		}
	}
	n.mu.Unlock()

	for _, adminID := range ready {
		n.mu.Lock()
		evs := n.outbox[adminID]
		delete(n.outbox, adminID)
		n.mu.Unlock()
		if len(evs) == 0 {
			continue
		}
		prefs, _ := n.prefsFor(ctx, adminID)
		text := n.i18n.T(prefs.Lang, "quiet.digest_title", len(evs)) + "\n"
		for _, ev := range evs {
			text += n.i18n.T(prefs.Lang, "quiet.digest_item", n.render(prefs.Lang, ev)) + "\n"
		}
		if n.send == nil {
			n.log.Info("notifier: would send quiet digest", "admin", adminID, "count", len(evs))
			continue
		}
		if _, err := n.send.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    adminID,
			Text:      text,
			ParseMode: models.ParseModeHTML,
		}); err != nil {
			n.log.Warn("notifier: flush quiet send", "err", err, "admin", adminID)
		}
	}
}
