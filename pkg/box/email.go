package box

import (
	"strings"

	gomail "gopkg.in/mail.v2"
)

var _ Notifyer = &email{}

type emailConf struct {
	Name string `yaml:"name,omitempty"`
	Pass string `yaml:"password,omitempty"`

	Tos []string `yaml:"to,omitempty"`
}

type email struct {
	conf *emailConf

	dial *gomail.Dialer
}

func newEmail(cf *emailConf) *email {
	if cf == nil {
		return nil
	}
	smtp := smtpServer(cf.Name)
	if smtp == "" {
		return nil
	}

	// always 465
	d := gomail.NewDialer(smtp, 465, cf.Name, cf.Pass)

	return &email{
		conf: cf,
		dial: d,
	}
}
func (e *email) Name() string {
	return "email"
}
func (e *email) Push(msg *Message) error {

	m := gomail.NewMessage()

	// Set E-Mail sender
	m.SetHeader("From", e.conf.Name)

	// Set E-Mail receivers
	m.SetHeader("To", strings.Join(e.conf.Tos, ", "))

	// Set E-Mail subject
	m.SetHeader("Subject", msg.Title)

	// Set E-Mail body. You can set plain text or html with text/html
	m.SetBody("text/plain", msg.Msg)

	// Now send E-Mail
	return e.dial.DialAndSend(m)
}

func smtpServer(email string) string {
	es := strings.Split(email, "@")
	if len(es) != 2 {
		return ""
	}
	switch es[1] {
	case "gmail.com":
		return "smtp.gmail.com"
	case "qq.com":
		return "smtp.qq.com"
	case "163.com":
		return "smtp.163.com"
	default:
		return ""
	}
}
