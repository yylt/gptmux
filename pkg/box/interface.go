package box

import (
	"fmt"
	"time"
)

type Message struct {
	Msg   string
	Title string
	Time  time.Time
}

func (m *Message) String() string {
	return fmt.Sprintf("title:%s, msg:%s, time: %s", m.Title, m.Msg, m.Time)
}

type Namer interface {
	Name() string
}

type Notifyer interface {
	Namer
	Push(*Message) error
}

type Receiver interface {
	Namer
	Receive(func(*Message) bool) error
}

type Box interface {
	Notifyer
	Receiver
}
