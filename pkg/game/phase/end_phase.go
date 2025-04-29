package phase

import (
	"context"
	"fmt"
	"time"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
)

// EndPhase 游戏结束阶段
type EndPhase struct {
	*BasePhase
	winner game.Camp
}

func NewEndPhase(players []game.Player, winner game.Camp) *EndPhase {
	return &EndPhase{
		BasePhase: NewBasePhase(players),
		winner:    winner,
	}
}

func (e *EndPhase) GetName() game.PhaseType {
	return game.PhaseEnd
}

func (e *EndPhase) Start(ctx context.Context) error {
	// 构建游戏结果数据
	gameResult := event.GameResultData{
		Winner:    e.winner.String(),
		Survivors: make([]string, 0),
		Roles:     make(map[string]string),
	}

	// 收集存活玩家和角色信息
	for _, player := range e.players {
		if player.IsAlive() {
			gameResult.Survivors = append(gameResult.Survivors, player.GetID())
		}
		gameResult.Roles[player.GetID()] = string(player.GetRole().GetName())
	}

	// 通知所有玩家游戏结束
	if err := e.broadcastEvent(event.Event[event.SystemGameEndData]{
		Type:      event.EventSystemGameEnd,
		Receivers: e.getAllPlayerIDs(),
		Timestamp: time.Now(),
		Data: event.SystemGameEndData{
			Winner:    e.winner.String(),
			Round:     e.round,
			Timestamp: time.Now(),
			Players:   e.getPlayerInfos(),
		},
	}); err != nil {
		return fmt.Errorf("broadcast game end failed: %w", err)
	}

	// 通知所有玩家游戏结束阶段结束
	if err := e.broadcastPhaseEnd(game.PhaseEnd, "游戏结束，感谢参与"); err != nil {
		return fmt.Errorf("broadcast game end phase end failed: %w", err)
	}

	return nil
}

// getPlayerInfos 获取所有玩家的信息
func (e *EndPhase) getPlayerInfos() []event.PlayerInfo {
	infos := make([]event.PlayerInfo, 0, len(e.players))
	for _, player := range e.players {
		infos = append(infos, event.PlayerInfo{
			ID:      player.GetID(),
			Role:    string(player.GetRole().GetName()),
			IsAlive: player.IsAlive(),
		})
	}
	return infos
}
