package gfw

type Message struct {
	ID   int32
	Body []byte
}

func NewMessage(id int32, body []byte) *Message {
	return &Message{
		ID:   id,
		Body: body,
	}
}
