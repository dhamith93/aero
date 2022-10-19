package aero

import "time"

const (
	ERR = "ERROR"
	WRN = "WARN"
	MSG = "MSG"
)

type Message struct {
	Time   int64
	Type   string
	String string
}

type Messages interface {
	Add(msg string, msgType string)
	Get() *[]Message
}

type AeroMessages struct {
	messages []Message
}

func (a *AeroMessages) Add(msg string, msgType string) {
	a.messages = append(a.messages, Message{
		Time:   time.Now().Unix(),
		Type:   msgType,
		String: msg,
	})
}

func (a *AeroMessages) Get() *[]Message {
	return &a.messages
}
