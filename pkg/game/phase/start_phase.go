package phase

import (
	"context"
	"fmt"
	"time"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
)

// StartPhase 游戏开始阶段
type StartPhase struct {
	*BasePhase
}

func NewStartPhase(players []game.Player) *StartPhase {
	return &StartPhase{
		BasePhase: NewBasePhase(players),
	}
}

func (s *StartPhase) GetName() game.PhaseType {
	return game.PhaseStart
}

// IsComplete for StartPhase always returns true as Start() is synchronous.
func (s *StartPhase) IsComplete(runtimeWrapper interface{}) bool {
	s.logger.Debug("StartPhase.IsComplete called, returning true", "phaseName", s.GetName())
	return true
}

func (s *StartPhase) Start(ctx context.Context) error {
	s.BasePhase.actions = make([]*game.Action, 0) // Clear actions, though StartPhase doesn't collect them
	s.logger.Info("StartPhase starting", "round", s.round)
	// 通知所有玩家游戏开始
	if err := s.broadcastPhaseStart(game.PhaseStart, "游戏开始，请所有玩家查看自己的身份"); err != nil {
		return fmt.Errorf("broadcast game start failed: %w", err)
	}

	// 为每个玩家分配角色并通知
	for _, player := range s.players {
		// 构建玩家信息
		playerInfo := event.PlayerInfo{
			ID:      player.GetID(),
			Role:    string(player.GetRole().GetName()),
			IsAlive: true,
		}

		// 构建阶段信息
		phaseInfo := event.PhaseInfo{
			Type:      string(game.PhaseStart),
			Round:     s.round,
			StartTime: time.Now(),
			Duration:  30, // 30秒准备时间
		}

		// 发送游戏开始事件给每个玩家
		if err := s.broadcastEvent(event.Event[event.SystemGameStartData]{
			Type:      event.SystemGameStart,
			PlayerID:  player.GetID(),
			Receivers: []string{player.GetID()}, // 只发送给该玩家
			Timestamp: time.Now(),
			Data: event.SystemGameStartData{
				Players: []event.PlayerInfo{playerInfo},
				Phase:   phaseInfo,
				Role:    string(player.GetRole().GetName()),
			},
		}); err != nil {
			return fmt.Errorf("broadcast player role failed: %w", err)
		}
	}

	// 等待准备时间
	time.Sleep(30 * time.Second)

	// 通知所有玩家游戏开始阶段结束
	if err := s.broadcastPhaseEnd(game.PhaseStart, "游戏开始阶段结束，进入夜晚阶段"); err != nil {
		return fmt.Errorf("broadcast game start phase end failed: %w", err)
	}

	return nil
}
