package matcherrors

import "errors"

// Rejoin/matchmaking sentinel errors. Used by both matchmaking and ws packages
// to avoid circular imports.
var (
	ErrGameNotFound    = errors.New("game not found")
	ErrGameFinished    = errors.New("game finished")
	ErrInvalidToken    = errors.New("invalid rejoin token")
	ErrNotDisconnected = errors.New("this player is not disconnected")
	ErrNoActiveGame    = errors.New("no active game for this user")
)
