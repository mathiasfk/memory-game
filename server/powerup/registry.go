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
	Apply(board *game.Board, active *game.Player, opponent *game.Player) error
}

// Registry holds all registered power-ups indexed by their ID.
type Registry struct {
	powerUps map[string]PowerUp
}

// NewRegistry creates a new empty power-up registry.
func NewRegistry() *Registry {
	return &Registry{
		powerUps: make(map[string]PowerUp),
	}
}

// Register adds a power-up to the registry.
func (r *Registry) Register(p PowerUp) {
	r.powerUps[p.ID()] = p
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

// AllPowerUps returns all registered power-ups as game.PowerUpDef slices.
// It satisfies the game.PowerUpProvider interface.
func (r *Registry) AllPowerUps() []game.PowerUpDef {
	defs := make([]game.PowerUpDef, 0, len(r.powerUps))
	for _, p := range r.powerUps {
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
