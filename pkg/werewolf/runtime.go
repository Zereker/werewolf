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

var (
	ErrGameAlreadyStarted = errors.New("werewolf already started")
	ErrGameNotStarted     = errors.New("werewolf not started")
	ErrGameInitFailed     = errors.New("werewolf init failed")
	ErrInvalidPhase       = errors.New("invalid phase")
	ErrPlayerNotFound     = errors.New("player not found")
	ErrPlayerInvalidSkill = errors.New("player not allowed to use this skill")
)

// Runtime 游戏运行时
type Runtime struct {
	sync.RWMutex

	// 技能列表
	skills []game.Skill

	// 玩家列表
	players map[string]*Player

	// 游戏阶段
	phase game.Phase // 当前阶段

	// 游戏状态
	started bool
	ended   bool
	round   int

	// 游戏结果
	winner game.Camp

	// 事件通道
	userEventChan   chan Event
	systemEventChan chan Event

	// 阶段结果
	phaseResults map[int]map[game.PhaseType]*game.PhaseResult
}

// NewRuntime 创建新的游戏运行时
func NewRuntime() *Runtime {
	return &Runtime{
		round:           1,
		players:         make(map[string]*Player),
		userEventChan:   make(chan Event, 100),
		systemEventChan: make(chan Event, 1),
		phaseResults:    make(map[int]map[game.PhaseType]*game.PhaseResult),
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

func (r *Runtime) initPhase() error {
	// 获取所有玩家列表
	var players []game.Player
	for _, p := range r.players {
		players = append(players, p)
	}

	// 创建各个阶段
	night := phase.NewNightPhase()
	day := phase.NewDayPhase()
	vote := phase.NewVotePhase()

	// 构建循环链表
	night.SetNextPhase(day)
	day.SetNextPhase(vote)
	vote.SetNextPhase(night)

	r.phase = night
	return nil
}

// Init 初始化游戏
func (r *Runtime) Init() error {
	r.Lock()
	defer r.Unlock()

	var err error
	if r.started {
		return ErrGameAlreadyStarted
	}

	// 游戏阶段初始化
	if err := r.initPhase(); err != nil {
		return ErrGameInitFailed
	}

	return nil
}

// nextPhase 进入下一阶段
func (r *Runtime) nextPhase() {
	r.Lock()
	defer r.Unlock()

	currentPhase := r.phase.GetName()
	r.phase = r.phase.GetNextPhase()

	// 如果从投票阶段回到夜晚阶段，回合数加1
	if currentPhase == (game.PhaseVote) {
		r.round++
	}

	// 重置所有玩家的保护状态
	for _, p := range r.players {
		p.SetProtected(false)
	}

	// 重置所有技能
	for _, s := range r.skills {
		s.Reset()
	}
}

func (r *Runtime) findSkill(userID string, skillType game.SkillType) (game.Skill, error) {
	p, exists := r.players[userID]
	if !exists {
		return nil, ErrPlayerNotFound
	}

	return p.GetSkill(skillType)
}

// useSkill 使用技能
func (r *Runtime) useSkill(casterID string, targetID string, skillType game.SkillType) error {
	r.Lock()
	defer r.Unlock()

	if !r.started || r.ended {
		return ErrGameNotStarted
	}

	// 查找施法者
	caster, exists := r.players[casterID]
	if !exists {
		return ErrPlayerNotFound
	}

	// 查找目标
	target, exists := r.players[targetID]
	if !exists {
		return ErrPlayerNotFound
	}

	// 查找技能
	s, err := r.findSkill(casterID, skillType)
	if err != nil {
		return err
	}

	// 检查技能是否可以在当前阶段使用
	if s.UseInPhase() != r.phase.GetName() {
		return ErrInvalidPhase
	}

	// 使用技能
	return r.phase.Handle(caster, target, s)
}

// completeCurrentPhase 完成当前阶段
func (r *Runtime) completeCurrentPhase() {
	if !r.phase.IsCompleted() {
		return
	}

	// 获取阶段结果
	result := r.phase.GetPhaseResult()

	// 存储阶段结果
	if _, exists := r.phaseResults[r.round]; !exists {
		r.phaseResults[r.round] = make(map[game.PhaseType]*game.PhaseResult)
	}
	r.phaseResults[r.round][r.phase.GetName()] = result

	// 处理阶段结果
	switch r.phase.GetName() {
	case game.PhaseNight:
		r.handleNightPhaseEnd(result)
	case game.PhaseDay:
		r.handleDayPhaseEnd(result)
	case game.PhaseVote:
		r.handleVotePhaseEnd(result)
	}

	// 检查游戏是否结束
	if r.checkGameEnd() {
		r.endGame()
		return
	}

	// 进入下一阶段
	r.nextPhase()
}

// handleNightPhaseEnd 处理夜晚阶段结束
func (r *Runtime) handleNightPhaseEnd(result *game.PhaseResult) {
	// 死亡通知事件 - 所有玩家可见
	for _, p := range result.Deaths {
		var receivers []string
		for id := range r.players {
			receivers = append(receivers, id)
		}

		r.broadcastEvent(Event{
			Type: EventSystemPlayerDeath,
			Data: map[string]interface{}{
				"player_id": p,
				"round":     r.round,
			},
			Receivers: receivers, // 设置所有玩家为接收者
			Timestamp: time.Now(),
		})
	}

	// 夜晚结束事件 - 所有玩家可见
	var receivers []string
	for id := range r.players {
		receivers = append(receivers, id)
	}

	r.broadcastEvent(Event{
		Type: EventSystemPhaseEnd,
		Data: &SystemPhaseData{
			Phase:     game.PhaseNight,
			Round:     r.round,
			Timestamp: time.Now(),
		},
		Receivers: receivers, // 设置所有玩家为接收者
	})
}

// handleDayPhaseEnd 处理白天阶段结束
func (r *Runtime) handleDayPhaseEnd(result *game.PhaseResult) {
	// 广播白天结果
	r.broadcastEvent(Event{
		Type: EventSystemPhaseEnd,
		Data: &SystemPhaseData{
			Phase:     game.PhaseDay,
			Round:     r.round,
			Timestamp: time.Now(),
		},
	})
}

// handleVotePhaseEnd 处理投票阶段结束
func (r *Runtime) handleVotePhaseEnd(result *game.PhaseResult) {
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
			Phase:     game.PhaseVote,
			Round:     r.round,
			Timestamp: time.Now(),
		},
	})
}

// checkGameEnd 检查游戏是否结束
func (r *Runtime) checkGameEnd() bool {
	goodCount := 0
	badCount := 0

	for _, p := range r.players {
		if !p.IsAlive() {
			continue
		}

		if p.GetCamp() == game.CampGood {
			goodCount++
		} else {
			badCount++
		}
	}

	// 如果某一阵营人数为0，游戏结束
	if goodCount == 0 {
		r.winner = game.CampBad
		return true
	}
	if badCount == 0 {
		r.winner = game.CampGood
		return true
	}

	return false
}

// handleUserSkill 处理用户技能事件
func (r *Runtime) handleUserSkill(evt Event) {
	data, ok := evt.Data.(*UserSkillData)
	if !ok {
		return
	}

	err := r.useSkill(evt.PlayerID, data.TargetID, data.SkillType)
	if err != nil {
		// 技能使用失败事件 - 只有使用技能的玩家可见
		r.broadcastEvent(Event{
			Type: EventSystemSkillResult,
			Data: &SystemSkillResultData{
				SkillType: data.SkillType,
				Success:   false,
				Message:   err.Error(),
			},
			PlayerID:  evt.PlayerID,
			Receivers: []string{evt.PlayerID}, // 只设置技能使用者为接收者
			Timestamp: time.Now(),
		})
		return
	}

	// 技能使用成功事件 - 根据技能类型决定接收者
	var receivers []string
	switch data.SkillType {
	case game.SkillTypeVote:
		// 投票结果所有存活玩家可见
		for id, p := range r.players {
			if p.IsAlive() {
				receivers = append(receivers, id)
			}
		}
	default:
		// 默认只有使用者可见
		receivers = []string{evt.PlayerID}
	}

	r.broadcastEvent(Event{
		Type: EventSystemSkillResult,
		Data: &SystemSkillResultData{
			SkillType: data.SkillType,
			Success:   true,
			Message:   "技能使用成功",
		},
		PlayerID:  evt.PlayerID,
		Receivers: receivers,
		Timestamp: time.Now(),
	})
}

// handleUserSpeak 处理用户发言事件
func (r *Runtime) handleUserSpeak(evt Event) {
	data, ok := evt.Data.(*UserSpeakData)
	if !ok {
		return
	}

	// 检查是否是当前发言玩家
	if r.phase.GetName() != game.PhaseDay {
		return
	}

	// 广播发言内容
	r.broadcastEvent(Event{
		Type:      EventUserSpeak,
		PlayerID:  evt.PlayerID,
		Data:      data,
		Timestamp: time.Now(),
	})
}

// handleUserVote 处理用户投票事件
func (r *Runtime) handleUserVote(evt Event) {
	data, ok := evt.Data.(*UserVoteData)
	if !ok {
		return
	}

	// 检查是否在投票阶段
	if r.phase.GetName() != game.PhaseVote {
		return
	}

	// 使用投票技能
	err := r.useSkill(evt.PlayerID, data.TargetID, game.SkillTypeVote)
	if err != nil {
		// 发送投票失败事件
		r.broadcastEvent(Event{
			Type: EventSystemVoteResult,
			Data: &SystemVoteResultData{
				Round:   r.round,
				Message: err.Error(),
				Success: false,
			},
			PlayerID:  evt.PlayerID,
			Receivers: []string{evt.PlayerID},
			Timestamp: time.Now(),
		})
	}
}

// Start 启动游戏
func (r *Runtime) Start(ctx context.Context) error {
	r.Lock()
	if r.started {
		return ErrGameAlreadyStarted
	}
	r.started = true
	r.Unlock()

	// 广播游戏开始事件
	var players []PlayerInfo
	for _, p := range r.players {
		players = append(players, PlayerInfo{
			ID:      p.GetID(),
			Role:    p.GetRole(),
			IsAlive: p.IsAlive(),
		})
	}

	r.broadcastEvent(Event{
		Type: EventSystemGameStart,
		Data: &SystemGameStartData{
			Players: players,
			Phase: PhaseInfo{
				Type:      r.phase.GetName(),
				Round:     r.round,
				StartTime: time.Now(),
				Duration:  300, // 设置默认阶段持续时间为5分钟
			},
		},
		Timestamp: time.Now(),
	})

	// 启动事件循环
	go r.eventLoop(ctx)

	return nil
}

// endGame 结束游戏
func (r *Runtime) endGame() {
	r.Lock()
	defer r.Unlock()

	if r.ended {
		return
	}

	r.ended = true

	// 广播游戏结束事件
	r.broadcastEvent(Event{
		Type: EventSystemGameEnd,
		Data: &SystemGameEndData{
			Winner:    r.winner,
			Round:     r.round,
			Timestamp: time.Now(),
		},
	})
}

// eventLoop 事件循环
func (r *Runtime) eventLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-r.systemEventChan:
			r.handleSysEvent(evt)
		case evt := <-r.userEventChan:
			r.handleUserEvent(evt)
		case <-ticker.C:
			r.completeCurrentPhase()
		}
	}
}

// broadcastEvent 广播事件到系统事件通道
func (r *Runtime) broadcastEvent(evt Event) {
	r.RLock()
	defer r.RUnlock()

	select {
	case r.systemEventChan <- evt:
	default:
		// 如果channel已满，跳过
	}
}

// handleUserEvent 处理用户事件
func (r *Runtime) handleUserEvent(evt Event) {
	switch evt.Type {
	case EventUserSkill:
		r.handleUserSkill(evt)
	case EventUserSpeak:
		r.handleUserSpeak(evt)
	case EventUserVote:
		r.handleUserVote(evt)
	}
}

func (r *Runtime) handleSysEvent(evt Event) {
	r.RLock()
	defer r.RUnlock()

	// 将事件发送给所有接收者
	for _, receiverID := range evt.Receivers {
		if p, exists := r.players[receiverID]; exists {
			p.Send(evt)
		}
	}
}

// SendUserEvent 用户发送事件到游戏运行时
func (r *Runtime) SendUserEvent(playerID string, eventType EventType, data interface{}) error {
	r.RLock()
	defer r.RUnlock()

	// 检查玩家是否存在
	if _, exists := r.players[playerID]; !exists {
		return ErrPlayerNotFound
	}

	// 检查游戏状态
	if !r.started || r.ended {
		return ErrGameNotStarted
	}

	// 创建用户事件
	evt := Event{
		Type:      eventType,
		PlayerID:  playerID,
		Data:      data,
		Timestamp: time.Now(),
	}

	// 根据事件类型设置接收者
	switch eventType {
	case EventUserSpeak:
		// 发言所有人可见
		var receivers []string
		for id := range r.players {
			receivers = append(receivers, id)
		}
		evt.Receivers = receivers

	case EventUserSkill:
		// 技能使用结果只有使用者可见
		evt.Receivers = []string{playerID}

	case EventUserVote:
		// 投票信息所有存活玩家可见
		var receivers []string
		for id, p := range r.players {
			if p.IsAlive() {
				receivers = append(receivers, id)
			}
		}
		evt.Receivers = receivers
	}

	// 发送到用户事件通道
	select {
	case r.userEventChan <- evt:
		return nil
	default:
		// 如果channel已满，返回错误
		return errors.New("event channel is full")
	}
}
