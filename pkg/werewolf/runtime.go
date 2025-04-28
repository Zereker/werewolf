package werewolf

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/phase"
	"github.com/Zereker/werewolf/pkg/game/player"
)

var (
	ErrGameAlreadyStarted  = errors.New("werewolf already started")
	ErrGameNotStarted      = errors.New("werewolf not started")
	ErrGameInitFailed      = errors.New("werewolf init failed")
	ErrPlayerNotFound      = errors.New("player not found")
	ErrPlayerSkillNotFound = errors.New("player skill not found")
	ErrInvalidPhase        = errors.New("invalid phase")
	ErrEventChannelFull    = errors.New("event channel is full")
	ErrInvalidPlayerCount  = errors.New("invalid player count")
)

const (
	MinPlayerCount           = 6
	DefaultEventChannelSize  = 100
	DefaultSystemChannelSize = 9
)

// Runtime 游戏运行时
type Runtime struct {
	sync.RWMutex

	logger *slog.Logger

	players  map[string]*Player
	started  bool
	ended    bool
	round    int
	phaseIdx int
	phases   []game.Phase
	winner   game.Camp

	userEventChan   chan Event
	systemEventChan chan Event
	phaseResults    map[int]map[game.PhaseType]*game.PhaseResult[game.SkillResultMap]
}

func NewRuntime() *Runtime {
	return &Runtime{
		logger:  slog.Default().With("game", "werewolf"),
		players: make(map[string]*Player),

		userEventChan:   make(chan Event, DefaultEventChannelSize),
		systemEventChan: make(chan Event, DefaultSystemChannelSize),
		phaseResults:    make(map[int]map[game.PhaseType]*game.PhaseResult[game.SkillResultMap]),
	}
}

func (r *Runtime) AddPlayer(id string, role game.Role) error {
	r.Lock()
	defer r.Unlock()

	if r.started {
		return ErrGameAlreadyStarted
	}

	r.players[id] = New(id, player.New(role))
	return nil
}

func (r *Runtime) init() error {
	r.Lock()
	defer r.Unlock()

	r.phases = []game.Phase{
		phase.NewNightPhase(),
		phase.NewDayPhase(),
		phase.NewVotePhase(),
	}
	r.phaseIdx = 0

	r.started = true
	r.ended = false
	r.round = 1

	return nil
}

func (r *Runtime) check() error {
	r.Lock()
	defer r.Unlock()

	if r.started {
		return ErrGameAlreadyStarted
	}

	if len(r.players) < MinPlayerCount {
		return fmt.Errorf("%w: need at least %d players", ErrInvalidPlayerCount, MinPlayerCount)
	}

	return nil
}

func (r *Runtime) Start(ctx context.Context) error {
	if err := r.check(); err != nil {
		return err
	}

	if err := r.init(); err != nil {
		return fmt.Errorf("%w: %v", ErrGameInitFailed, err)
	}

	r.broadcastGameStart()
	go r.eventLoop(ctx)

	r.handlePhase()
	return nil
}

func (r *Runtime) eventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			r.endGame()
			return

		case evt := <-r.systemEventChan:
			r.handleSysEvent(evt)

		case evt := <-r.userEventChan:
			if r.isGameActive() {
				r.handleUserEvent(evt)
			}
		}

		// 检查阶段是否完成
		if r.isGameActive() {
			if p := r.getCurrentPhase(); p != nil && p.IsCompleted() {
				r.completeCurrentPhase()
			}
		}
	}
}

func (r *Runtime) isGameActive() bool {
	r.RLock()
	defer r.RUnlock()
	return r.started && !r.ended
}

func (r *Runtime) handleUserEvent(evt Event) {
	currentPhase := r.getCurrentPhase()

	var action *game.Action

	switch evt.Type {
	case EventUserSkill:
		data, ok := evt.Data.(*UserSkillData)
		if !ok {
			return
		}

		caster := r.players[evt.PlayerID]
		target := r.players[data.TargetID]
		skill := r.findSkill(caster, data.SkillType)
		if skill == nil {
			return
		}

		action = &game.Action{
			Caster: caster,
			Target: target,
			Skill:  skill,
		}
	case EventUserSpeak:
		data, ok := evt.Data.(*UserSpeakData)
		if !ok {
			return
		}

		caster := r.players[evt.PlayerID]
		// 假设发言也是一种技能
		skill := r.findSkill(caster, game.SkillTypeSpeak)
		if skill == nil {
			return
		}

		action = &game.Action{
			Caster:  caster,
			Skill:   skill,
			Content: data.Message,
		}
	case EventUserVote:
		data, ok := evt.Data.(*UserVoteData)
		if !ok {
			return
		}

		caster := r.players[evt.PlayerID]
		target := r.players[data.TargetID]
		skill := r.findSkill(caster, game.SkillTypeVote)
		if skill == nil {
			return
		}

		action = &game.Action{
			Caster: caster,
			Target: target,
			Skill:  skill,
		}
	}

	if action != nil {
		if err := currentPhase.Handle(action); err != nil {
			r.logger.Error("phase handle failed", "err", err)
		}
	}
}

func (r *Runtime) findSkill(p *Player, skillType game.SkillType) game.Skill {
	for _, s := range p.GetRole().GetAvailableSkills() {
		if s.GetName() == skillType {
			return s
		}
	}

	return nil
}

func (r *Runtime) handleSysEvent(evt Event) {
	r.RLock()
	defer r.RUnlock()

	for _, receiverID := range evt.Receivers {
		if p, exists := r.players[receiverID]; exists {
			p.Send(evt)
		}
	}
}

func (r *Runtime) getAllPlayerIDs() []string {
	r.RLock()
	defer r.RUnlock()

	ids := make([]string, 0, len(r.players))
	for id := range r.players {
		ids = append(ids, id)
	}
	return ids
}

func (r *Runtime) getAlivePlayerIDs() []string {
	r.RLock()
	defer r.RUnlock()

	ids := make([]string, 0, len(r.players))
	for id, p := range r.players {
		if p.IsAlive() {
			ids = append(ids, id)
		}
	}
	return ids
}

func (r *Runtime) getCurrentPhase() game.Phase {
	r.RLock()
	defer r.RUnlock()

	if len(r.phases) == 0 {
		return nil
	}

	return r.phases[r.phaseIdx]
}

func (r *Runtime) nextPhase() {
	r.Lock()
	defer r.Unlock()

	// 移动到下一个阶段
	r.phaseIdx = (r.phaseIdx + 1) % len(r.phases)

	// 如果是夜晚阶段开始，通知狼人队友
	if r.getCurrentPhase().GetName() == game.PhaseNight {
		r.notifyWerewolfTeammates()
	}

	// 如果是投票阶段结束，增加回合数
	if r.getCurrentPhase().GetName() == game.PhaseNight && r.phaseIdx == 0 {
		r.round++
	}

	// 重置所有玩家的保护状态
	for _, p := range r.players {
		p.SetProtected(false)
	}
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

	r.broadcastEvent(Event{
		Type: EventSystemPhaseEnd,
		Data: &SystemPhaseData{
			Phase:     phaseType,
			Round:     r.round,
			Timestamp: time.Now(),
		},
	})
}

func (r *Runtime) checkGameEnd() bool {
	r.RLock()
	defer r.RUnlock()

	goodCount := 0
	evilCount := 0
	totalAlive := 0

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

	if evilCount == 0 {
		r.winner = game.CampGood
		return true
	}

	if goodCount == 0 || goodCount <= evilCount {
		r.winner = game.CampEvil
		return true
	}

	if totalAlive == 0 {
		r.winner = game.CampNone
		return true
	}

	return false
}

func (r *Runtime) endGame() {
	r.Lock()
	defer r.Unlock()

	if r.ended {
		return
	}

	r.ended = true

	endData := &SystemGameEndData{
		Winner:    r.winner,
		Round:     r.round,
		Timestamp: time.Now(),
		Players:   make([]PlayerInfo, 0, len(r.players)),
	}

	for _, p := range r.players {
		endData.Players = append(endData.Players, PlayerInfo{
			ID:      p.GetID(),
			Role:    p.GetRole(),
			IsAlive: p.IsAlive(),
		})
	}

	r.broadcastEvent(Event{
		Type:      EventSystemGameEnd,
		Data:      endData,
		Receivers: r.getAllPlayerIDs(),
		Timestamp: time.Now(),
	})

	close(r.userEventChan)
	close(r.systemEventChan)
}

func (r *Runtime) SendUserEvent(playerID string, eventType EventType, data interface{}) error {
	r.RLock()
	defer r.RUnlock()

	if _, exists := r.players[playerID]; !exists {
		return ErrPlayerNotFound
	}

	if !r.started || r.ended {
		return ErrGameNotStarted
	}

	evt := Event{
		Type:      eventType,
		PlayerID:  playerID,
		Data:      data,
		Timestamp: time.Now(),
	}

	evt.Receivers = r.getEventReceivers(eventType, playerID)

	select {
	case r.userEventChan <- evt:
		return nil
	default:
		return ErrEventChannelFull
	}
}

func (r *Runtime) getEventReceivers(eventType EventType, playerID string) []string {
	switch eventType {
	case EventUserSpeak:
		return r.getAllPlayerIDs()
	case EventUserSkill:
		return []string{playerID}
	case EventUserVote:
		return r.getAlivePlayerIDs()
	default:
		return []string{playerID}
	}
}

func (r *Runtime) notifyWerewolfTeammates() {
	r.RLock()
	defer r.RUnlock()

	// 找出所有狼人
	var werewolves []*Player
	for _, p := range r.players {
		if p.GetRole().GetName() == game.RoleTypeWerewolf {
			werewolves = append(werewolves, p)
		}
	}

	// 给每个狼人推送队友信息
	for _, wolf := range werewolves {
		teammates := []string{}
		for _, teammate := range werewolves {
			if teammate != wolf {
				teammates = append(teammates, teammate.GetID())
			}
		}

		// 发送事件
		r.broadcastEvent(Event{
			Type: EventSystemSkillResult,
			Data: map[string]interface{}{
				"message": fmt.Sprintf("你的狼人队友有：%v", teammates),
			},
			Receivers: []string{wolf.GetID()},
			Timestamp: time.Now(),
		})
	}
}
