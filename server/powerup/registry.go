package powerup

import (
	"memory-game-server/game"
)

// PowerUp defines the interface that all power-ups must implement.
type PowerUp interface {
	ID() string
	Name() string
	Description() string
	Cost() int
	Apply(board *game.Board, active *game.Player, opponent *game.Player, ctx *game.PowerUpContext) error
}

// Registry holds all registered power-ups indexed by their ID.
type Registry struct {
	powerUps map[string]PowerUp
	order    []string // registration order for deterministic AllPowerUps()
}

// NewRegistry creates a new empty power-up registry.
func NewRegistry() *Registry {
	return &Registry{
		powerUps: make(map[string]PowerUp),
		order:    nil,
	}
}

// Register adds a power-up to the registry.
func (r *Registry) Register(p PowerUp) {
	id := p.ID()
	if _, exists := r.powerUps[id]; !exists {
		r.order = append(r.order, id)
	}
	r.powerUps[id] = p
}

// GetPowerUp returns the power-up definition for the game package.
// It satisfies the game.PowerUpProvider interface.
func (r *Registry) GetPowerUp(id string) (game.PowerUpDef, bool) {
	p, ok := r.powerUps[id]
	if !ok {
		return game.PowerUpDef{}, false
	}
	return game.PowerUpDef{
		ID:          p.ID(),
		Name:        p.Name(),
		Description: p.Description(),
		Cost:        p.Cost(),
		Apply:       p.Apply,
	}, true
}

// AllPowerUps returns all registered power-ups as game.PowerUpDef slices, in registration order.
// It satisfies the game.PowerUpProvider interface.
func (r *Registry) AllPowerUps() []game.PowerUpDef {
	defs := make([]game.PowerUpDef, 0, len(r.order))
	for _, id := range r.order {
		p := r.powerUps[id]
		defs = append(defs, game.PowerUpDef{
			ID:          p.ID(),
			Name:        p.Name(),
			Description: p.Description(),
			Cost:        p.Cost(),
			Apply:       p.Apply,
		})
	}
	return defs
}
