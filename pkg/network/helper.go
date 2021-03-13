package network

type Helper struct {
	EventSender func(event *Event, networkType string) error
}

type Event struct {
	EventType string
	Reason    string
	Message   string
}
