package powerup

import (
	"math/rand"
	"memory-game-server/game"
)

// Rarity constants for weighted arcana selection (higher = more likely to appear in a match).
const (
	RarityCommon   = 1
	RarityUncommon = 2
	RarityRare     = 3
)

// PowerUp defines the interface that all power-ups must implement.
type PowerUp interface {
	ID() string
	Name() string
	Description() string
	Cost() int
	Rarity() int
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
		Rarity:      p.Rarity(),
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
			Rarity:      p.Rarity(),
			Apply:       p.Apply,
		})
	}
	return defs
}

// PickArcanaForMatch selects n distinct power-ups with probability proportional to Rarity (higher = more likely).
// It satisfies the game.PowerUpProvider interface.
func (r *Registry) PickArcanaForMatch(n int) []game.PowerUpDef {
	all := r.AllPowerUps()
	if n <= 0 || len(all) == 0 {
		return nil
	}
	if n >= len(all) {
		return all
	}
	// Weighted selection without replacement: weight = max(Rarity, 1)
	indices := make([]int, len(all))
	weights := make([]int, len(all))
	for i := range all {
		indices[i] = i
		w := all[i].Rarity
		if w < 1 {
			w = 1
		}
		weights[i] = w
	}
	picked := make([]game.PowerUpDef, 0, n)
	for len(picked) < n && len(indices) > 0 {
		var total int
		for _, w := range weights {
			total += w
		}
		if total <= 0 {
			break
		}
		roll := rand.Intn(total)
		var idx int
		for i, w := range weights {
			roll -= w
			if roll < 0 {
				idx = i
				break
			}
		}
		picked = append(picked, all[indices[idx]])
		// Remove chosen from indices and weights
		indices = append(indices[:idx], indices[idx+1:]...)
		weights = append(weights[:idx], weights[idx+1:]...)
	}
	return picked
}
