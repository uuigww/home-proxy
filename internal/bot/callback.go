// Package bot implements the Telegram bot UI for home-proxy.
//
// The bot speaks exclusively to admins (enforced by middleware) and mutates
// all state through a single message per admin. Callback_data strings must
// fit in Telegram's 64-byte limit, so we keep them short and namespaced by
// a two- or three-letter prefix.
package bot

// Callback-data prefixes. Payloads are appended after the delimiter where
// useful (e.g. "uc:42" → user card for id 42). Keep constants short —
// Telegram caps callback_data at 64 bytes.
const (
	CBMainMenu          = "m"
	CBHelp              = "h"
	CBUsersList         = "ul:"  // ul:<page>
	CBUserCard          = "uc:"  // uc:<id>
	CBUserToggleV       = "utv:" // utv:<id>
	CBUserToggleS       = "uts:" // uts:<id>
	CBUserToggleMTProto = "utm:" // utm:<id>
	CBUserLimit         = "ulm:" // ulm:<id>
	CBUserLinks         = "ull:" // ull:<id>
	CBUserQR            = "uqr:" // uqr:<id>
	CBUserDisable       = "ud:"  // ud:<id>
	CBUserEnable        = "ue:"  // ue:<id>
	CBUserDelete        = "udl:" // udl:<id>
	CBUserDelYes        = "udy:" // udy:<id>
	CBUserDelNo         = "udn:" // udn:<id>

	CBAddStart        = "as"
	CBAddProtoVLESS   = "ap1"
	CBAddProtoSOCKS   = "ap2"
	CBAddProtoMTProto = "ap3"
	CBAddNext         = "an"
	CBAddLimit10      = "al10"
	CBAddLimit50      = "al50"
	CBAddLimit100     = "al100"
	CBAddLimitInf     = "ali"
	CBAddLimitCustom  = "alc"
	CBAddCancel       = "ax"

	CBStatsMain = "s"

	CBServerMain          = "sv"
	CBServerRotate        = "svr"
	CBServerRotateMTProto = "svm"
	CBServerUpdateGeo     = "svg"
	CBServerNotifications = "svn"
	CBServerCheckUpdates  = "svc"
	CBServerSelfUpdate    = "svu"

	CBNotifToggleCritical  = "ntc"
	CBNotifToggleImportant = "nti"
	CBNotifToggleInfo      = "ntn"
	CBNotifToggleOthers    = "nto"
	CBNotifToggleSecurity  = "nts"
	CBNotifToggleDaily     = "ntd"

	CBNoop = "nop"
)
