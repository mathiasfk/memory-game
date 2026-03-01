package wsutil

import "log/slog"

// SafeSend sends data to a channel without panicking if the channel is closed.
// If the channel is full or closed, the send is skipped. Panics are recovered
// and logged for debugging.
func SafeSend(ch chan []byte, data []byte) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("SafeSend recovered panic", "tag", "wsutil", "panic", r)
		}
	}()
	select {
	case ch <- data:
	default:
	}
}
