package werewolf

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/phase"
	"github.com/Zereker/werewolf/pkg/game/player"
)

// Runtime 游戏运行时
type Runtime struct {
	sync.RWMutex

	winner  game.Camp
	players []game.Player

	started bool

	round    int
	phaseIdx int
	phases   []game.Phase

	logger *slog.Logger
}

// NewRuntime 创建游戏运行时
func NewRuntime() *Runtime {
	return &Runtime{
		round:  1,
		logger: slog.Default().With("game", "werewolf"),
	}
}

// AddPlayer 添加玩家
func (r *Runtime) AddPlayer(id string, role game.Role) error {
	r.Lock()
	defer r.Unlock()

	// 创建玩家
	p := player.New(id, role)
	r.players = append(r.players, p)
	return nil
}

// Init 初始化游戏
func (r *Runtime) Init() error {
	r.Lock()
	defer r.Unlock()

	// 检查玩家数量
	if len(r.players) < 6 {
		return fmt.Errorf("need at least 6 players")
	}

	// 初始化阶段列表
	r.phaseIdx = 0
	r.phases = []game.Phase{
		phase.NewNightPhase(r.players),
		phase.NewDayPhase(r.players),
		phase.NewVotePhase(r.players),
	}

	return nil
}

// Start 开始游戏
func (r *Runtime) Start(ctx context.Context) error {
	r.Lock()
	r.started = true
	r.Unlock()

	// 创建游戏开始阶段
	startPhase := phase.NewStartPhase(r.players)
	if err := startPhase.Start(ctx); err != nil {
		return fmt.Errorf("start phase failed: %w", err)
	}

	// 游戏主循环
	for r.checkGameEnd() == game.CampNone {
		// 获取当前阶段
		currentPhase := r.phases[r.phaseIdx]
		if err := currentPhase.Start(ctx); err != nil {
			return fmt.Errorf("phase %d failed: %w", r.phaseIdx, err)
		}

		// 推进到下一个阶段
		r.phaseIdx = (r.phaseIdx + 1) % len(r.phases)
		if r.phaseIdx == 0 {
			// 如果回到第一个阶段，说明一轮结束，回合数加1
			r.round++
		}
	}

	// 游戏结束，创建结束阶段
	endPhase := phase.NewEndPhase(r.players, r.checkGameEnd())
	if err := endPhase.Start(ctx); err != nil {
		return fmt.Errorf("end phase failed: %w", err)
	}

	return nil
}

// checkGameEnd 检查游戏是否结束
func (r *Runtime) checkGameEnd() game.Camp {
	r.RLock()
	defer r.RUnlock()

	// 统计各阵营存活人数
	goodCount := 0
	evilCount := 0

	for _, p := range r.players {
		if !p.IsAlive() {
			continue
		}

		switch p.GetRole().GetCamp() {
		case game.CampGood:
			goodCount++
		case game.CampEvil:
			evilCount++
		case game.CampNone:
			panic("unhandled default case")
		}
	}

	// 判断游戏是否结束
	if goodCount == 0 {
		return game.CampEvil
	}

	if evilCount == 0 {
		return game.CampGood
	}

	return game.CampNone
}
