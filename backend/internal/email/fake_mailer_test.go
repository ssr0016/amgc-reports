package email

import "context"

type FakeMailer struct {
	Messages []Message
	Err      error
}

func (f *FakeMailer) Send(_ context.Context, message Message) error {
	if f.Err != nil {
		return f.Err
	}
	f.Messages = append(f.Messages, message)
	return nil
}
