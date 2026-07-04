package out

// EventPublisher is the driven port for one-way backend-to-frontend notifications.
type EventPublisher interface {
	Publish(topic string, payload any)
}

// Topic helpers keep the event name strings defined in exactly one place.

func TopicSessionData(id string) string {
	return "session:data:" + id
}

func TopicSessionState(id string) string {
	return "session:state:" + id
}

func TopicSessionClosed(id string) string {
	return "session:closed:" + id
}

// TopicSessionHostKey notifies the frontend of an unrecognized SSH host
// key awaiting a trust/once/cancel decision via RespondHostKey.
func TopicSessionHostKey(id string) string {
	return "session:hostkey:" + id
}
