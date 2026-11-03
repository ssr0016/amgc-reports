package email

import "context"

type Mailer interface {
	Send(ctx context.Context, message Message) error
}

type Message struct {
	To        string
	Subject   string
	PlainText string
	HTML      string
}
