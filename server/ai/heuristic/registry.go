package heuristic

import (
	"memory-game-server/game"
)

// EVFunc returns the expected value (in points) of using the power-up in the given state.
// A negative return value means "do not evaluate / do not use by heuristic" (e.g. unregistered power-up).
type EVFunc func(state *game.GameStateMsg, memory map[int]int, hidden []int, P int) float64

// PickTargetFunc returns a card index to use as target for power-ups that require one (e.g. Clairvoyance, Oblivion).
// Returns -1 if no valid target or if the power-up does not use a target.
type PickTargetFunc func(state *game.GameStateMsg, memory map[int]int, hidden []int, rows, cols int) int

type entry struct {
	ev         EVFunc
	pickTarget PickTargetFunc
}

var registry = make(map[string]entry)

// Register adds or overwrites the heuristic for a power-up. Either ev or pickTarget may be nil.
func Register(powerUpID string, ev EVFunc, pickTarget PickTargetFunc) {
	registry[powerUpID] = entry{ev: ev, pickTarget: pickTarget}
}

// EV returns the expected value for using the given power-up, or a value < 0 if not registered or not evaluated.
func EV(powerUpID string, state *game.GameStateMsg, memory map[int]int, hidden []int, P int) float64 {
	e, ok := registry[powerUpID]
	if !ok || e.ev == nil {
		return -1
	}
	return e.ev(state, memory, hidden, P)
}

// PickTarget returns the target card index for the power-up, or -1 if no target or not registered.
func PickTarget(powerUpID string, state *game.GameStateMsg, memory map[int]int, hidden []int, rows, cols int) int {
	e, ok := registry[powerUpID]
	if !ok || e.pickTarget == nil {
		return -1
	}
	return e.pickTarget(state, memory, hidden, rows, cols)
}
