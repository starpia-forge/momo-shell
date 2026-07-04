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

// TopicHistoryAppended notifies the frontend a new command-history entry
// was saved (or an existing one's timestamp was bumped). Not scoped to one
// session ID -- the history panel shows entries from every session.
func TopicHistoryAppended() string {
	return "history:appended"
}
