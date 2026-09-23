package alert

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

const smtpTimeout = 30 * time.Second

// Mailer sends plain-text mail over SMTP. Port 465 uses implicit TLS; any other
// port upgrades with STARTTLS when the server offers it. Resend works through
// its SMTP relay (smtp.resend.com, username "resend", API key as password).
type Mailer struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       []string
}

func (m *Mailer) Send(subject, body string) error {
	addr := net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
	dialer := &net.Dialer{Timeout: smtpTimeout}

	var conn net.Conn
	var err error
	if m.Port == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: m.Host})
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", addr, err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(smtpTimeout)); err != nil {
		return err
	}

	c, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		return fmt.Errorf("smtp handshake with %s: %w", addr, err)
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: m.Host}); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if m.Username != "" {
		// PlainAuth refuses to send credentials over an unencrypted connection.
		if err := c.Auth(smtp.PlainAuth("", m.Username, m.Password, m.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(m.From); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	for _, to := range m.To {
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("smtp RCPT TO %s: %w", to, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write(m.message(subject, body)); err != nil {
		return fmt.Errorf("writing message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	return c.Quit()
}

func (m *Mailer) message(subject, body string) []byte {
	domain := m.From[strings.LastIndex(m.From, "@")+1:]
	id := make([]byte, 12)
	_, _ = rand.Read(id)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", m.From)
	fmt.Fprintf(&buf, "To: %s\r\n", strings.Join(m.To, ", "))
	fmt.Fprintf(&buf, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&buf, "Message-ID: <%s@%s>\r\n", hex.EncodeToString(id), domain)
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	qp := quotedprintable.NewWriter(&buf)
	_, _ = qp.Write([]byte(body))
	_ = qp.Close()
	return buf.Bytes()
}
