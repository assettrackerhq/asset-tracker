package email

import (
	"fmt"
	"net"
	"net/smtp"
)

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

type Sender struct {
	cfg SMTPConfig
}

func NewSender(cfg SMTPConfig) *Sender {
	return &Sender{cfg: cfg}
}

func (s *Sender) Send(to, subject, body string) error {
	addr := net.JoinHostPort(s.cfg.Host, s.cfg.Port)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s",
		s.cfg.From, to, subject, body)

	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	return smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg))
}
