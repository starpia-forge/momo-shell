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

// TopicTransferTask notifies the frontend of a transfer task's creation or
// any state change. Not scoped to a task ID -- the transfer center subscribes
// once and keeps its own task list in sync.
func TopicTransferTask() string {
	return "transfer:task"
}

// TopicTransferProgress carries incremental byte-count/rate updates for one
// task, published at most a few times per second while it runs.
func TopicTransferProgress(taskID string) string {
	return "transfer:progress:" + taskID
}

// TopicTransferZmodem notifies the frontend of ZMODEM detection/phase
// changes on a session (rz/sz over the shell channel, as opposed to SFTP).
func TopicTransferZmodem(sessionID string) string {
	return "transfer:zmodem:" + sessionID
}

// TopicOSFileDrop notifies the frontend of an OS-level file drop onto the
// webview (Wails' native drag-and-drop, which resolves absolute paths that
// browser File objects don't expose). Not scoped to a session -- the
// frontend resolves which pane/panel was under the drop from (x, y).
func TopicOSFileDrop() string {
	return "os:filedrop"
}

// TopicSharePairRequest notifies the frontend of an incoming /pair request
// awaiting the local user's approve/deny decision via RespondPairing.
func TopicSharePairRequest() string {
	return "share:pair-request"
}

// TopicSharePeersUpdated notifies the frontend that the consumer-side peer
// list (discovered and/or paired) changed and should be re-fetched via
// ListPeers.
func TopicSharePeersUpdated() string {
	return "share:peers-updated"
}

// TopicSharePairRequestResolved notifies the frontend that a previously
// raised share:pair-request is no longer pending (answered, timed out, or
// the requester disconnected), so the approval dialog can dismiss itself
// even without ever calling RespondPairing for it.
func TopicSharePairRequestResolved() string {
	return "share:pair-request-resolved"
}

// TopicMCPConnectApproval notifies the frontend of a pending connect_host
// grant request awaiting the local user's approve/deny decision via
// RespondConnectApproval. Mirrors TopicSharePairRequest.
func TopicMCPConnectApproval() string {
	return "mcp:connect-approval"
}

// TopicMCPControlApproval notifies the frontend of a pending RequestControl
// grant request awaiting the local user's approve/deny decision via
// RespondControlApproval. Separate from TopicMCPConnectApproval so the
// frontend can render the right dialog for each request kind.
func TopicMCPControlApproval() string {
	return "mcp:control-approval"
}

// TopicMCPCommandApproval notifies the frontend of a pending RunCommand
// request (risk medium/high/uncertain) awaiting the local user's
// approve/deny decision via RespondCommandApproval. Separate from the
// connect/control approval topics so the frontend can render the
// command-specific dialog (risk, reasons, guarded rewrite -- FR-7).
func TopicMCPCommandApproval() string {
	return "mcp:cmd-approval"
}

// TopicMCPCommandState notifies the frontend of a run_command execution
// lifecycle transition (running/tui/done -- doc 17 §6.4's state-streaming
// handle, D1). Separate from TopicMCPCommandApproval, which is the
// pre-execution gate decision, not the post-injection execution progress.
func TopicMCPCommandState() string {
	return "mcp:cmd-state"
}
