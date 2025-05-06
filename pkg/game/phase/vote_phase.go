package phase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
)

// VotePhase 投票阶段
type VotePhase struct {
	*BasePhase

	deaths       []game.Player
	skillResults game.SkillResultMap
}

func NewVotePhase(players []game.Player) *VotePhase {
	return &VotePhase{
		BasePhase:    NewBasePhase(players),
		deaths:       make([]game.Player, 0),
		skillResults: make(game.SkillResultMap),
	}
}

func (v *VotePhase) GetName() game.PhaseType {
	return game.PhaseVote
}

// broadcastEvent 广播事件
func (v *VotePhase) broadcastEvent(evt any) error {
	// 将事件转换为 event.Event[any] 类型
	eventAny, ok := evt.(event.Event[any])
	if !ok {
		return fmt.Errorf("invalid event type: %T", evt)
	}

	for _, receiverID := range eventAny.Receivers {
		if player, exists := v.players[receiverID]; exists {
			if err := player.Write(eventAny); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *VotePhase) Start(ctx context.Context) error {
	// 通知所有玩家进入投票阶段
	if err := v.broadcastPhaseStart(game.PhaseVote, "请所有玩家投票"); err != nil {
		return fmt.Errorf("broadcast vote phase start failed: %w", err)
	}

	// 等待所有玩家投票
	if err := v.waitForVotes(); err != nil {
		return err
	}

	// 计算投票结果
	phaseResult := v.GetPhaseResult()

	// 通知所有玩家投票阶段结束
	message := "投票阶段结束"
	if len(phaseResult.Deaths) > 0 {
		deathNames := make([]string, 0, len(phaseResult.Deaths))
		for _, player := range phaseResult.Deaths {
			deathNames = append(deathNames, player.GetID())
		}
		message = fmt.Sprintf("投票阶段结束。被投票处决的玩家是：%s", strings.Join(deathNames, "、"))
	}

	if err := v.broadcastPhaseEnd(game.PhaseVote, message); err != nil {
		return fmt.Errorf("broadcast vote phase end failed: %w", err)
	}

	return nil
}

// waitForVotes 等待所有玩家投票
func (v *VotePhase) waitForVotes() error {
	// 获取所有存活的玩家
	alivePlayers := v.getAlivePlayerIDs()
	if len(alivePlayers) == 0 {
		return nil
	}

	// 为每个玩家创建一个等待组
	for _, playerID := range alivePlayers {
		player := v.players[playerID]
		if player == nil {
			continue
		}

		// 等待该玩家的投票
		evt, err := player.Read(30 * time.Second)
		if err != nil {
			continue
		}

		// 处理用户事件
		if evt.Type == event.UserSkill {
			skillData := evt.Data.(*event.UserSkillData)
			// 将用户事件转换为玩家行动
			action := game.Action{
				Caster: player,
				Target: v.players[skillData.TargetID],
				Skill:  v.getSkillByType(game.SkillTypeVote),
			}

			// 执行行动
			if err := action.Skill.Check(v.GetName(), action.Caster, action.Target); err != nil {
				continue
			}

			v.AddAction(&action)
		}
	}

	return nil
}

// GetPhaseResult 获取阶段结果
func (v *VotePhase) GetPhaseResult() *game.PhaseResult[game.SkillResultMap] {
	// 按优先级排序所有投票行为
	sort.Slice(v.actions, func(i, j int) bool {
		return v.actions[i].Skill.GetPriority() < v.actions[j].Skill.GetPriority()
	})

	// 执行所有投票并统计
	voteCount := make(map[game.Player]int)
	voteRecord := make(map[game.Player]game.Player)

	// 执行投票并记录
	for _, action := range v.actions {
		// 执行技能
		action.Skill.Put(action.Caster, action.Target, game.PutOption{
			Content: action.Content,
		})

		// 记录投票
		voteCount[action.Target]++
		voteRecord[action.Caster] = action.Target
	}

	// 找出票数最多的玩家
	var maxVotes int
	var votedOut []game.Player
	for player, count := range voteCount {
		if count > maxVotes {
			maxVotes = count
			votedOut = []game.Player{player}
		} else if count == maxVotes {
			votedOut = append(votedOut, player)
		}
	}

	// 只有在以下条件都满足时才处理出局：
	// 1. 有最高票的玩家
	// 2. 不是平票
	// 3. 总投票数超过最小阈值（这里设置为2票）
	const minVotesRequired = 2
	if len(votedOut) == 1 && maxVotes > minVotesRequired {
		player := votedOut[0]
		player.SetAlive(false)
		v.deaths = append(v.deaths, player)
	} else {
		// 如果条件不满足，清空出局列表
		votedOut = nil
	}

	// 记录投票结果
	v.AddSkillResult(game.SkillTypeVote, &game.SkillResult{
		Success: true,
		Message: "投票结果",
		Data: map[string]interface{}{
			"votes":     voteRecord, // 投票记录
			"voteCount": voteCount,  // 票数统计
			"votedOut":  votedOut,   // 被投出的玩家
		},
	})

	return &game.PhaseResult[game.SkillResultMap]{
		Deaths:    v.deaths,
		ExtraData: v.skillResults,
	}
}

func (v *VotePhase) GetType() game.PhaseType {
	return game.PhaseVote
}
