package phase

import (
	"fmt"
	"time"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
)

// LastWordsPhase 遗言阶段
type LastWordsPhase struct {
	round   int
	players map[string]game.Player
	deaths  []game.Player
}

func NewLastWordsPhase(round int, players []game.Player, deaths []game.Player) *LastWordsPhase {
	playerMap := make(map[string]game.Player)
	for _, player := range players {
		playerMap[player.GetID()] = player
	}

	return &LastWordsPhase{
		round:   round,
		players: playerMap,
		deaths:  deaths,
	}
}

func (p *LastWordsPhase) GetName() game.PhaseType {
	return "last_words"
}

func (p *LastWordsPhase) Start() error {
	// 如果不是第一回合，直接返回
	if p.round != 1 {
		return nil
	}

	// 如果没有死亡玩家，直接返回
	if len(p.deaths) == 0 {
		return nil
	}

	// 通知所有玩家进入遗言阶段
	if err := p.broadcastEvent(event.Event[event.PhaseStartData]{
		Type: event.EventSystemPhaseStart,
		Data: event.PhaseStartData{
			Phase: "last_words",
		},
		Receivers: p.getAlivePlayerIDs(),
		Timestamp: time.Now(),
	}); err != nil {
		return fmt.Errorf("broadcast last words phase start failed: %w", err)
	}

	// 让每个死亡的玩家发表遗言
	for _, deadPlayer := range p.deaths {
		// 通知该玩家发表遗言
		if err := p.broadcastEvent(event.Event[event.SkillResultData]{
			Type: event.EventSystemSkillResult,
			Data: event.SkillResultData{
				SkillType: string(game.SkillTypeLastWords),
				Message:   "请发表遗言",
				PlayerID:  deadPlayer.GetID(),
			},
			Receivers: []string{deadPlayer.GetID()},
			Timestamp: time.Now(),
		}); err != nil {
			return fmt.Errorf("broadcast last words skill result failed: %w", err)
		}

		// 等待玩家发表遗言
		if _, err := p.players[deadPlayer.GetID()].Read(30 * time.Second); err != nil {
			return fmt.Errorf("wait for last words failed: %w", err)
		}
	}

	// 通知所有玩家遗言阶段结束
	if err := p.broadcastEvent(event.Event[event.PhaseStartData]{
		Type: event.EventSystemPhaseEnd,
		Data: event.PhaseStartData{
			Phase: "last_words",
		},
		Receivers: p.getAlivePlayerIDs(),
		Timestamp: time.Now(),
	}); err != nil {
		return fmt.Errorf("broadcast last words phase end failed: %w", err)
	}

	return nil
}

// broadcastEvent 广播事件
func (p *LastWordsPhase) broadcastEvent(evt any) error {
	// 将事件转换为 event.Event[any] 类型
	eventAny, ok := evt.(event.Event[any])
	if !ok {
		return fmt.Errorf("invalid event type: %T", evt)
	}

	for _, receiverID := range eventAny.Receivers {
		if player, exists := p.players[receiverID]; exists {
			if err := player.Write(eventAny); err != nil {
				return err
			}
		}
	}
	return nil
}

// getAlivePlayerIDs 获取所有存活玩家的ID
func (p *LastWordsPhase) getAlivePlayerIDs() []string {
	ids := make([]string, 0)
	for id, player := range p.players {
		if player.IsAlive() {
			ids = append(ids, id)
		}
	}
	return ids
}
