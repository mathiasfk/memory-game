package wsutil

import "log"

// SafeSend sends data to a channel without panicking if the channel is closed.
// If the channel is full or closed, the send is skipped. Panics are recovered
// and logged for debugging.
func SafeSend(ch chan []byte, data []byte) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[wsutil] SafeSend recovered panic: %v", r)
		}
	}()
	select {
	case ch <- data:
	default:
	}
}
