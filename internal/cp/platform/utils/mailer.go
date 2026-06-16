package utils

import (
	"bytes"
	"context"
	"fmt"
	"github.com/JohnnyAsh-U/ashrix-api/internal/cp/platform/config"
	"html/template"
	"net/smtp"
	"strings"
)

// Mailer is the interface for sending emails.
// Keeps the service decoupled from any specific mail provider.
type MailerService interface {
	SendPasswordReset(ctx context.Context, email, resetURL string) error
}

// Mailer sends transactional emails via SMTP.
// Used for admin invites and cert expiry alerts.
// In development (no SMTP_HOST set), emails are printed to stdout.
type Mailer struct {
	cfg *config.Config
}

func NewMailerService(cfg *config.Config) MailerService {
	return &Mailer{cfg: cfg}
}

// ── Email types ────────────────────────────────────────────────────────────

type AdminInviteData struct {
	InviterEmail string
	OrgName      string
	OrgSlug      string
	Role         string
	InviteURL    string // one-time setup link
}

type CertExpiryData struct {
	OrgName       string
	ComponentName string
	ComponentType string
	ExpiresIn     string // e.g. "7 days"
	DashboardURL  string
}

type SSOBoundData struct {
	AdminEmail   string
	OrgName      string
	DashboardURL string
}

// ── Send methods ───────────────────────────────────────────────────────────

//Send PasswordReset Email
func (m *Mailer) SendPasswordReset(ctx context.Context, email, resetURL string) error {
	subject := "Ashrix Password Reset"
	body, err := render(passwordResetTemplate, resetURL)
	if err != nil {
		return fmt.Errorf("render password reset email: %w", err)
	}
	return m.send(email, subject, body)
}





// SendAdminInvite sends a dashboard invite email to a new admin.
// The invite URL contains a one-time token for account setup.
func (m *Mailer) SendAdminInvite(to string, data AdminInviteData) error {
	subject := fmt.Sprintf("You've been invited to %s on Ashrix", data.OrgName)
	body, err := render(adminInviteTemplate, data)
	if err != nil {
		return fmt.Errorf("render invite email: %w", err)
	}
	return m.send(to, subject, body)
}

// SendCertExpiryAlert alerts an org owner that a component cert is expiring soon.
func (m *Mailer) SendCertExpiryAlert(to string, data CertExpiryData) error {
	subject := fmt.Sprintf("[Ashrix] Certificate expiring in %s — %s", data.ExpiresIn, data.ComponentName)
	body, err := render(certExpiryTemplate, data)
	if err != nil {
		return fmt.Errorf("render cert expiry email: %w", err)
	}
	return m.send(to, subject, body)
}

// SendSSOBoundConfirmation emails the admin after SSO binding completes.
func (m *Mailer) SendSSOBoundConfirmation(to string, data SSOBoundData) error {
	subject := fmt.Sprintf("[Ashrix] SSO login enabled for %s", data.OrgName)
	body, err := render(ssoBoundTemplate, data)
	if err != nil {
		return fmt.Errorf("render sso bound email: %w", err)
	}
	return m.send(to, subject, body)
}

// ── Core send ──────────────────────────────────────────────────────────────

func (m *Mailer) send(to, subject, htmlBody string) error {
	// Dev mode — no SMTP configured, print to stdout
	if m.cfg.SMTPHost == "" {
		fmt.Printf("\n─── [DEV EMAIL] ───────────────────────────────\n")
		fmt.Printf("To:      %s\n", to)
		fmt.Printf("Subject: %s\n", subject)
		fmt.Printf("Body:\n%s\n", htmlBody)
		fmt.Printf("───────────────────────────────────────────────\n\n")
		return nil
	}

	msg := buildMessage(m.cfg.SMTPFrom, to, subject, htmlBody)
	addr := fmt.Sprintf("%s:%d", m.cfg.SMTPHost, m.cfg.SMTPPort)

	var auth smtp.Auth
	if m.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPassword, m.cfg.SMTPHost)
	}

	if err := smtp.SendMail(addr, auth, m.cfg.SMTPFrom, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}

	return nil
}

func buildMessage(from, to, subject, htmlBody string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("From: Ashrix <%s>\r\n", from))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", to))
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(htmlBody)
	return sb.String()
}

func render(tmpl string, data any) (string, error) {
	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ── Templates ──────────────────────────────────────────────────────────────

const passwordResetTemplate = `<!DOCTYPE html>
<html>
<body style="font-family: sans-serif; color: #111; max-width: 560px; margin: 40px auto; padding: 0 20px;">
  <h2 style="color: #1a1a1a;">Password reset</h2>
  <p>
    You recently requested a password reset for your Ashrix account.
	{{ . }}
</p>
  `



const adminInviteTemplate = `<!DOCTYPE html>
<html>
<body style="font-family: sans-serif; color: #111; max-width: 560px; margin: 40px auto; padding: 0 20px;">
  <h2 style="color: #1a1a1a;">You've been invited to {{.OrgName}}</h2>
  <p>
    <strong>{{.InviterEmail}}</strong> has invited you to manage
    <strong>{{.OrgName}}</strong> on Ashrix as an <strong>{{.Role}}</strong>.
  </p>
  <p>Click the button below to set up your account. This link expires in 24 hours.</p>
  <p style="margin: 32px 0;">
    <a href="{{.InviteURL}}"
       style="background:#111;color:#fff;padding:12px 24px;border-radius:6px;
              text-decoration:none;font-weight:600;">
      Accept Invitation
    </a>
  </p>
  <p style="color:#666;font-size:13px;">
    If you weren't expecting this, you can safely ignore this email.
  </p>
  <hr style="border:none;border-top:1px solid #eee;margin:32px 0;">
  <p style="color:#999;font-size:12px;">Ashrix — Secure application access</p>
</body>
</html>`

const certExpiryTemplate = `<!DOCTYPE html>
<html>
<body style="font-family: sans-serif; color: #111; max-width: 560px; margin: 40px auto; padding: 0 20px;">
  <h2 style="color: #c0392b;">Certificate expiring soon</h2>
  <p>
    The TLS certificate for <strong>{{.ComponentName}}</strong>
    ({{.ComponentType}}) in <strong>{{.OrgName}}</strong>
    will expire in <strong>{{.ExpiresIn}}</strong>.
  </p>
  <p>Log in to your Ashrix dashboard to rotate the certificate before it expires.</p>
  <p style="margin: 32px 0;">
    <a href="{{.DashboardURL}}"
       style="background:#c0392b;color:#fff;padding:12px 24px;border-radius:6px;
              text-decoration:none;font-weight:600;">
      Go to Dashboard
    </a>
  </p>
  <hr style="border:none;border-top:1px solid #eee;margin:32px 0;">
  <p style="color:#999;font-size:12px;">Ashrix — Secure application access</p>
</body>
</html>`

const ssoBoundTemplate = `<!DOCTYPE html>
<html>
<body style="font-family: sans-serif; color: #111; max-width: 560px; margin: 40px auto; padding: 0 20px;">
  <h2 style="color: #1a1a1a;">SSO login enabled</h2>
  <p>
    SSO has been successfully configured for <strong>{{.OrgName}}</strong>.
    Your account <strong>{{.AdminEmail}}</strong> is now linked to your
    identity provider.
  </p>
  <p>
    From now on, use your organisation's SSO to log in to the Ashrix dashboard.
  </p>
  <p style="margin: 32px 0;">
    <a href="{{.DashboardURL}}"
       style="background:#111;color:#fff;padding:12px 24px;border-radius:6px;
              text-decoration:none;font-weight:600;">
      Go to Dashboard
    </a>
  </p>
  <p style="color:#666;font-size:13px;">
    If you did not make this change, contact your Ashrix administrator immediately.
  </p>
  <hr style="border:none;border-top:1px solid #eee;margin:32px 0;">
  <p style="color:#999;font-size:12px;">Ashrix — Secure application access</p>
</body>
</html>`
