package wsutil

import "log/slog"

// SafeSend sends data to a channel without panicking if the channel is closed.
// If the channel is full or closed, the send is skipped. Panics (e.g. "send on closed channel")
// are recovered; expected during disconnects, logged at Debug so we can verify it stays rare.
func SafeSend(ch chan []byte, data []byte) {
	defer func() {
		if r := recover(); r != nil {
			if r == "send on closed channel" {
				slog.Debug("SafeSend recovered panic (expected when client disconnects)", "tag", "wsutil", "panic", r)
			} else {
				slog.Warn("SafeSend recovered unexpected panic", "tag", "wsutil", "panic", r)
			}
		}
	}()
	select {
	case ch <- data:
	default:
	}
}
