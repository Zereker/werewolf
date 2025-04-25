package werewolf

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/phase"
	"github.com/Zereker/werewolf/pkg/game/player"
)

const (
	DefaultEventChannelSize  = 100
	DefaultSystemChannelSize = 1
	DefaultPhaseDuration     = 300 // 5分钟
	DefaultTickerDuration    = time.Second
)

var (
	ErrGameAlreadyStarted  = errors.New("werewolf already started")
	ErrGameNotStarted      = errors.New("werewolf not started")
	ErrGameInitFailed      = errors.New("werewolf init failed")
	ErrPlayerNotFound      = errors.New("player not found")
	ErrPlayerSkillNotFound = errors.New("player skill not found")
	ErrInvalidPhase        = errors.New("invalid phase")
	ErrEventChannelFull    = errors.New("event channel is full")
)

// Runtime 游戏运行时
type Runtime struct {
	sync.RWMutex

	// 玩家列表
	players map[string]*Player

	// 游戏状态
	started bool
	ended   bool
	round   int

	// 事件通道
	userEventChan   chan Event
	systemEventChan chan Event

	// 游戏阶段
	phaseIdx int          // 当前阶段索引
	phases   []game.Phase // 阶段数组

	// 阶段结果
	phaseResults map[int]map[game.PhaseType]*game.PhaseResult[game.SkillResultMap]

	// 游戏结果
	winner game.Camp
}

// NewRuntime 创建新的游戏运行时
func NewRuntime() *Runtime {
	return &Runtime{
		round:           1,
		players:         make(map[string]*Player),
		userEventChan:   make(chan Event, 100),
		systemEventChan: make(chan Event, 1),
		phaseResults:    make(map[int]map[game.PhaseType]*game.PhaseResult[game.SkillResultMap]),
	}
}

// AddPlayer 添加玩家
func (r *Runtime) AddPlayer(id string, role game.Role) error {
	r.Lock()
	defer r.Unlock()

	if r.started {
		return ErrGameAlreadyStarted
	}

	r.players[id] = New(id, player.New(role))
	return nil
}

// Init 初始化游戏
func (r *Runtime) Init() error {
	r.Lock()
	defer r.Unlock()

	if r.started {
		return ErrGameAlreadyStarted
	}

	// 游戏阶段初始化
	if err := r.initPhase(); err != nil {
		return ErrGameInitFailed
	}

	return nil
}

func (r *Runtime) Start(ctx context.Context) error {

}

// initPhase 初始化游戏阶段
func (r *Runtime) initPhase() error {
	// 按顺序创建阶段
	r.phases = []game.Phase{
		phase.NewNightPhase(),
		phase.NewDayPhase(),
		phase.NewVotePhase(),
	}

	r.phaseIdx = 0
	return nil
}

// nextPhase 进入下一阶段
func (r *Runtime) nextPhase() {
	r.Lock()
	defer r.Unlock()

	// 移动到下一个阶段
	r.phaseIdx = (r.phaseIdx + 1) % len(r.phases)

	// 如果回到第一个阶段（夜晚），回合数加1
	if r.phaseIdx == 0 {
		r.round++
	}

	// 重置所有玩家的保护状态
	for _, p := range r.players {
		p.SetProtected(false)
	}
}

// getCurrentPhase 获取当前阶段
func (r *Runtime) getCurrentPhase() game.Phase {
	r.RLock()
	defer r.RUnlock()

	if len(r.phases) == 0 {
		return nil
	}

	return r.phases[r.phaseIdx]
}

func (r *Runtime) useSkill(casterID string, targetID string, skillType game.SkillType) error {
	// 检查游戏状态
	if !r.started || r.ended {
		return ErrGameNotStarted
	}

	// 查找施法者和目标
	caster, exists := r.players[casterID]
	if !exists {
		return ErrPlayerNotFound
	}

	target, exists := r.players[targetID]
	if !exists {
		return ErrPlayerNotFound
	}

	// 查找技能
	var skill game.Skill
	for _, s := range caster.GetRole().GetAvailableSkills() {
		if s.GetName() == skillType {
			skill = s
			break
		}
	}

	if skill == nil {
		return ErrPlayerSkillNotFound
	}

	// 检查阶段
	currentPhase := r.getCurrentPhase()
	if currentPhase.GetName() != skill.GetPhase() {
		return ErrInvalidPhase
	}

	// 创建动作并执行
	action := &game.Action{
		Caster: caster,
		Target: target,
		Skill:  skill,
	}

	return currentPhase.Handle(action)
}

func (r *Runtime) broadcastEvent(evt Event) {
	r.RLock()
	defer r.RUnlock()

	// 如果没有设置接收者，默认发送给所有玩家
	if len(evt.Receivers) == 0 {
		for id := range r.players {
			evt.Receivers = append(evt.Receivers, id)
		}
	}

	select {
	case r.systemEventChan <- evt:
	default:
		// 记录日志或其他处理
	}
}

func (r *Runtime) getEventReceivers(eventType EventType, playerID string) []string {
	switch eventType {
	case EventUserSpeak:
		// 发言所有人可见
		return r.getAllPlayerIDs()
	case EventUserSkill:
		// 技能使用结果只有使用者可见
		return []string{playerID}
	case EventUserVote:
		// 投票信息所有存活玩家可见
		return r.getAlivePlayerIDs()
	default:
		return []string{playerID}
	}
}

func (r *Runtime) getAllPlayerIDs() []string {
	ids := make([]string, 0, len(r.players))
	for id := range r.players {
		ids = append(ids, id)
	}
	return ids
}

func (r *Runtime) getAlivePlayerIDs() []string {
	ids := make([]string, 0, len(r.players))
	for id, p := range r.players {
		if p.IsAlive() {
			ids = append(ids, id)
		}
	}
	return ids
}

func (r *Runtime) completeCurrentPhase() {
	currentPhase := r.getCurrentPhase()
	if currentPhase == nil || !currentPhase.IsCompleted() {
		return
	}

	result := currentPhase.GetPhaseResult()
	r.storePhaseResult(result)
	r.handlePhaseEnd(currentPhase.GetName(), result)

	if r.checkGameEnd() {
		r.endGame()
		return
	}

	r.nextPhase()
}

func (r *Runtime) storePhaseResult(result *game.PhaseResult[game.SkillResultMap]) {
	if _, exists := r.phaseResults[r.round]; !exists {
		r.phaseResults[r.round] = make(map[game.PhaseType]*game.PhaseResult[game.SkillResultMap])
	}

	r.phaseResults[r.round][r.getCurrentPhase().GetName()] = result
}

func (r *Runtime) handlePhaseEnd(phaseType game.PhaseType, result *game.PhaseResult[game.SkillResultMap]) {
	// 处理死亡玩家
	for _, p := range result.Deaths {
		r.broadcastEvent(Event{
			Type: EventSystemPlayerDeath,
			Data: map[string]interface{}{
				"player_id": p,
				"round":     r.round,
			},
			Timestamp: time.Now(),
		})
	}

	// 广播阶段结束
	r.broadcastEvent(Event{
		Type: EventSystemPhaseEnd,
		Data: &SystemPhaseData{
			Phase:     phaseType,
			Round:     r.round,
			Timestamp: time.Now(),
		},
	})
}

// checkGameEnd 检查游戏是否结束
func (r *Runtime) checkGameEnd() bool {
	r.RLock()
	defer r.RUnlock()

	goodCount := 0
	evilCount := 0
	totalAlive := 0

	// 统计存活玩家阵营情况
	for _, p := range r.players {
		if !p.IsAlive() {
			continue
		}
		totalAlive++

		switch p.GetRole().GetCamp() {
		case game.CampGood:
			goodCount++
		case game.CampEvil:
			evilCount++
		default:
			panic("unhandled default case")
		}
	}

	// 检查胜利条件
	if evilCount == 0 {
		r.winner = game.CampGood
		return true
	}

	if goodCount == 0 || goodCount <= evilCount {
		r.winner = game.CampEvil
		return true
	}

	// 检查是否所有玩家都死亡（平局）
	if totalAlive == 0 {
		r.winner = game.CampNone
		return true
	}

	return false
}

// endGame 结束游戏
func (r *Runtime) endGame() {
	r.Lock()
	defer r.Unlock()

	if r.ended {
		return
	}

	r.ended = true

	// 构建游戏结束数据
	endData := &SystemGameEndData{
		Winner:    r.winner,
		Round:     r.round,
		Timestamp: time.Now(),
		Players:   make([]PlayerInfo, 0, len(r.players)),
	}

	// 添加所有玩家的最终状态
	for _, p := range r.players {
		endData.Players = append(endData.Players, PlayerInfo{
			ID:      p.GetID(),
			Role:    p.GetRole(),
			IsAlive: p.IsAlive(),
		})
	}

	// 广播游戏结束事件
	r.broadcastEvent(Event{
		Type:      EventSystemGameEnd,
		Data:      endData,
		Receivers: r.getAllPlayerIDs(), // 发送给所有玩家
		Timestamp: time.Now(),
	})

	// 关闭事件通道
	close(r.userEventChan)
	close(r.systemEventChan)
}
