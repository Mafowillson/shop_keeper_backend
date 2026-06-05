package email

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type Service struct {
	cfg Config
}

func NewService(cfg Config) *Service {
	return &Service{cfg: cfg}
}

// SendVerificationCode sends the email-verification OTP.
// When SMTP_HOST is empty (dev mode), the code is logged to the console instead.
func (s *Service) SendVerificationCode(toEmail, ownerName, code string) error {
	if s.cfg.Host == "" {
		log.Printf("[EMAIL-DEV] Email verification code for %s: %s", toEmail, code)
		return nil
	}
	subject := "ShopKeeper: Your Verification Code"
	html := buildOTPEmail(
		ownerName,
		code,
		"Verify your email address",
		"Use the code below to verify your ShopKeeper account. It expires in 15 minutes.",
		"If you didn't create a ShopKeeper account, you can safely ignore this email.",
	)
	return s.sendMail(toEmail, subject, html)
}

// SendPasswordResetCode sends the password-reset OTP.
func (s *Service) SendPasswordResetCode(toEmail, ownerName, code string) error {
	if s.cfg.Host == "" {
		log.Printf("[EMAIL-DEV] Password reset code for %s: %s", toEmail, code)
		return nil
	}
	subject := "ShopKeeper — Password Reset Code"
	html := buildOTPEmail(
		ownerName,
		code,
		"Reset your password",
		"Use the code below to reset your ShopKeeper password. It expires in 15 minutes.",
		"If you didn't request a password reset, you can safely ignore this email.",
	)
	return s.sendMail(toEmail, subject, html)
}

func buildCodeBoxes(code string) string {
	var b strings.Builder
	for i, ch := range code {
		if i > 0 {
			b.WriteString(`<td style="width:8px;"></td>`)
		}
		fmt.Fprintf(&b,
			`<td style="width:52px;height:64px;background:#f8fafc;border:2px solid #e2e8f0;`+
				`border-radius:12px;text-align:center;vertical-align:middle;`+
				`font-size:32px;font-weight:700;color:#1e293b;font-family:monospace;" align="center">%c</td>`,
			ch,
		)
	}
	return b.String()
}

func buildOTPEmail(name, code, title, intro, disclaimer string) string {
	boxes := buildCodeBoxes(code)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1.0">
  <title>ShopKeeper</title>
</head>
<body style="margin:0;padding:0;background-color:#f1f5f9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table width="100%%" cellpadding="0" cellspacing="0" border="0" style="background:#f1f5f9;padding:48px 16px;">
    <tr>
      <td align="center">
        <table width="100%%" cellpadding="0" cellspacing="0" border="0" style="max-width:480px;">

          <!-- Header -->
          <tr>
            <td style="background:#0f172a;border-radius:16px 16px 0 0;padding:28px 40px;text-align:center;">
              <span style="font-size:22px;font-weight:800;color:#ffffff;letter-spacing:0.5px;">ShopKeeper</span>
            </td>
          </tr>

          <!-- Body -->
          <tr>
            <td style="background:#ffffff;padding:40px;text-align:center;">
              <h1 style="margin:0 0 10px;font-size:22px;font-weight:700;color:#0f172a;">%s</h1>
              <p style="margin:0 0 36px;font-size:15px;color:#64748b;line-height:1.6;">
                Hi <strong style="color:#0f172a;">%s</strong>, %s
              </p>

              <!-- OTP boxes -->
              <table cellpadding="0" cellspacing="0" border="0" style="margin:0 auto 36px;">
                <tr>%s</tr>
              </table>

              <p style="margin:0 0 6px;font-size:13px;color:#94a3b8;">
                This code expires in <strong style="color:#0f172a;">15 minutes</strong>.
              </p>
              <p style="margin:0;font-size:12px;color:#cbd5e1;">%s</p>
            </td>
          </tr>

          <!-- Footer -->
          <tr>
            <td style="background:#f8fafc;border-top:1px solid #e2e8f0;border-radius:0 0 16px 16px;padding:20px 40px;text-align:center;">
              <p style="margin:0;font-size:12px;color:#94a3b8;">&copy; 2026 ShopKeeper &mdash; All rights reserved.</p>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, title, name, intro, boxes, disclaimer)
}

func messageID(domain string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(b), domain)
}

func (s *Service) sendMail(to, subject, html string) error {
	envelopeFrom := s.cfg.From
	if parsed, err := mail.ParseAddress(s.cfg.From); err == nil {
		envelopeFrom = parsed.Address
	}
	domain := "shopkeeper.cm"
	if idx := strings.Index(envelopeFrom, "@"); idx != -1 {
		domain = envelopeFrom[idx+1:]
	}

	var msg strings.Builder
	msg.WriteString("From: " + s.cfg.From + "\r\n")
	msg.WriteString("To: " + to + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n")
	msg.WriteString("Message-ID: " + messageID(domain) + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(html)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	return smtp.SendMail(addr, auth, envelopeFrom, []string{to}, []byte(msg.String()))
}
