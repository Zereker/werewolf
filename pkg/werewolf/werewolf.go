package werewolf

import (
	"context"

	"github.com/Zereker/werewolf/pkg/game"
)

type Werewolf interface {
	AddPlayer(id string, role game.Role) error

	Init() error
	Start(ctx context.Context) error
}
