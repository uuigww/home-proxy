package deploy

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

// authChoiceLabels maps the UI labels shown by the wizard to AuthMethod values.
var authChoiceLabels = []string{
	"SSH password",
	"Private key file",
	"ssh-agent",
}

// labelToAuthMethod converts a UI label from authChoiceLabels to an AuthMethod.
func labelToAuthMethod(label string) (AuthMethod, error) {
	switch label {
	case "SSH password":
		return AuthPassword, nil
	case "Private key file":
		return AuthKey, nil
	case "ssh-agent":
		return AuthAgent, nil
	default:
		return 0, fmt.Errorf("unknown auth method label %q", label)
	}
}

// Prompt fills empty fields in p using interactive survey/v2 prompts. Already
// populated fields are left untouched so flag-based invocations can skip the
// wizard entirely.
func Prompt(p *Params) error {
	if p == nil {
		return fmt.Errorf("params is nil")
	}

	if p.Host == "" {
		if err := survey.AskOne(&survey.Input{
			Message: "Server IP or hostname:",
		}, &p.Host, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	if p.Port == 0 {
		p.Port = 22
		if err := survey.AskOne(&survey.Input{
			Message: "SSH port:",
			Default: "22",
		}, &p.Port); err != nil {
			return err
		}
	}

	if p.User == "" {
		if err := survey.AskOne(&survey.Input{
			Message: "SSH user:",
			Default: "root",
		}, &p.User); err != nil {
			return err
		}
	}

	if p.AuthMethod == AuthUnset {
		var choice string
		if err := survey.AskOne(&survey.Select{
			Message: "How do you want to authenticate?",
			Options: authChoiceLabels,
			Default: authChoiceLabels[0],
		}, &choice); err != nil {
			return err
		}
		m, err := labelToAuthMethod(choice)
		if err != nil {
			return err
		}
		p.AuthMethod = m
	}

	switch p.AuthMethod {
	case AuthPassword:
		if p.Password == "" {
			if err := survey.AskOne(&survey.Password{
				Message: "SSH password:",
			}, &p.Password, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
		}
	case AuthKey:
		if p.KeyPath == "" {
			if err := survey.AskOne(&survey.Input{
				Message: "Path to SSH private key:",
			}, &p.KeyPath, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
		}
		if p.KeyPass == "" {
			// Empty passphrase is allowed: only ask once, never require.
			_ = survey.AskOne(&survey.Password{
				Message: "Key passphrase (leave empty if none):",
			}, &p.KeyPass)
		}
	case AuthAgent:
		// Nothing to ask.
	}

	if p.BotToken == "" {
		if err := survey.AskOne(&survey.Password{
			Message: "Telegram bot token:",
		}, &p.BotToken, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	if len(p.Admins) == 0 {
		var raw string
		if err := survey.AskOne(&survey.Input{
			Message: "Admin Telegram IDs (comma-separated):",
		}, &raw, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
		ids, err := ParseAdmins(raw)
		if err != nil {
			return err
		}
		p.Admins = ids
	}

	if p.Lang == "" {
		if err := survey.AskOne(&survey.Select{
			Message: "Bot UI language:",
			Options: []string{"ru", "en"},
			Default: "ru",
		}, &p.Lang); err != nil {
			return err
		}
	}

	return nil
}

// ParseAdmins parses a comma-separated list of Telegram user IDs into []int64.
// Whitespace around entries is trimmed; empty input yields an error.
func ParseAdmins(raw string) ([]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("admins list is empty")
	}
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse admin id %q: %w", part, err)
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("admins list has no valid ids")
	}
	return out, nil
}
