package email

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"
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
	subject := "ShopKeeper — Email Verification Code"
	body := fmt.Sprintf(`Hello %s,

Thank you for creating a ShopKeeper account.

Your email verification code is:

    %s

This code expires in 15 minutes. If you did not create this account, please ignore this email.

— The ShopKeeper Team`, ownerName, code)
	return s.sendMail(toEmail, subject, body)
}

// SendPasswordResetCode sends the password-reset OTP.
func (s *Service) SendPasswordResetCode(toEmail, ownerName, code string) error {
	if s.cfg.Host == "" {
		log.Printf("[EMAIL-DEV] Password reset code for %s: %s", toEmail, code)
		return nil
	}
	subject := "ShopKeeper — Password Reset Code"
	body := fmt.Sprintf(`Hello %s,

We received a request to reset the password for your ShopKeeper account.

Your password reset code is:

    %s

This code expires in 15 minutes. If you did not request a password reset, please ignore this email.

— The ShopKeeper Team`, ownerName, code)
	return s.sendMail(toEmail, subject, body)
}

func (s *Service) sendMail(to, subject, body string) error {
	var msg strings.Builder
	msg.WriteString("From: " + s.cfg.From + "\r\n")
	msg.WriteString("To: " + to + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	return smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg.String()))
}
