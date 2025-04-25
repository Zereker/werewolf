package werewolf

import (
	"context"
	"errors"
	"fmt"
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
	DefaultSystemChannelSize = 1
)

// Runtime 游戏运行时
type Runtime struct {
	sync.RWMutex

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
		round:           1,
		players:         make(map[string]*Player),
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

func (r *Runtime) Start(ctx context.Context) error {
	r.Lock()
	defer r.Unlock()

	if r.started {
		return ErrGameAlreadyStarted
	}

	if len(r.players) < MinPlayerCount {
		return fmt.Errorf("%w: need at least %d players", ErrInvalidPlayerCount, MinPlayerCount)
	}

	if err := r.initPhase(); err != nil {
		return fmt.Errorf("%w: %v", ErrGameInitFailed, err)
	}

	r.started = true
	r.ended = false
	r.round = 1

	r.broadcastGameStart()
	go r.eventLoop(ctx)

	return nil
}

func (r *Runtime) initPhase() error {
	r.phases = []game.Phase{
		phase.NewNightPhase(),
		phase.NewDayPhase(),
		phase.NewVotePhase(),
	}
	r.phaseIdx = 0
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
	switch evt.Type {
	case EventUserSkill:
		r.handleSkillEvent(evt)
	case EventUserSpeak:
		r.handleSpeakEvent(evt)
	case EventUserVote:
		r.handleVoteEvent(evt)
	}
}

func (r *Runtime) handleSkillEvent(evt Event) {
	data, ok := evt.Data.(*UserSkillData)
	if !ok {
		return
	}

	err := r.useSkill(evt.PlayerID, data.TargetID, data.SkillType)
	r.broadcastSkillResult(evt.PlayerID, data.SkillType, err)
}

func (r *Runtime) handleSpeakEvent(evt Event) {
	data, ok := evt.Data.(*UserSpeakData)
	if !ok || r.getCurrentPhase().GetName() != game.PhaseDay {
		return
	}

	r.broadcastEvent(Event{
		Type:      EventUserSpeak,
		PlayerID:  evt.PlayerID,
		Data:      data,
		Receivers: r.getAllPlayerIDs(),
		Timestamp: time.Now(),
	})
}

func (r *Runtime) handleVoteEvent(evt Event) {
	data, ok := evt.Data.(*UserVoteData)
	if !ok || r.getCurrentPhase().GetName() != game.PhaseVote {
		return
	}

	err := r.useSkill(evt.PlayerID, data.TargetID, game.SkillTypeVote)
	r.broadcastVoteResult(evt.PlayerID, data.TargetID, err)
}

func (r *Runtime) useSkill(casterID, targetID string, skillType game.SkillType) error {
	r.RLock()
	defer r.RUnlock()

	if !r.started || r.ended {
		return ErrGameNotStarted
	}

	caster, exists := r.players[casterID]
	if !exists {
		return ErrPlayerNotFound
	}

	target, exists := r.players[targetID]
	if !exists {
		return ErrPlayerNotFound
	}

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

	if r.getCurrentPhase().GetName() != skill.GetPhase() {
		return ErrInvalidPhase
	}

	return r.getCurrentPhase().Handle(&game.Action{
		Caster: caster,
		Target: target,
		Skill:  skill,
	})
}

func (r *Runtime) broadcastSkillResult(playerID string, skillType game.SkillType, err error) {
	var receivers []string
	var success bool
	var message string

	if err != nil {
		success = false
		message = err.Error()
		receivers = []string{playerID}
	} else {
		success = true
		message = "技能使用成功"
		receivers = skillType == game.SkillTypeVote ? r.getAlivePlayerIDs() : []string{playerID}
	}

	r.broadcastEvent(Event{
		Type: EventSystemSkillResult,
		Data: &SystemSkillResultData{
			SkillType: skillType,
			Success:   success,
			Message:   message,
		},
		PlayerID:  playerID,
		Receivers: receivers,
		Timestamp: time.Now(),
	})
}

func (r *Runtime) broadcastVoteResult(voterID, targetID string, err error) {
	if err != nil {
		r.broadcastEvent(Event{
			Type: EventSystemVoteResult,
			Data: &SystemVoteResultData{
				Round:   r.round,
				Success: false,
				Message: err.Error(),
			},
			PlayerID:  voterID,
			Receivers: []string{voterID},
			Timestamp: time.Now(),
		})
		return
	}

	r.broadcastEvent(Event{
		Type: EventSystemVoteResult,
		Data: &SystemVoteResultData{
			Round:    r.round,
			Success:  true,
			VoterID:  voterID,
			TargetID: targetID,
			Message:  "投票成功",
		},
		PlayerID:  voterID,
		Receivers: r.getAlivePlayerIDs(),
		Timestamp: time.Now(),
	})
}

func (r *Runtime) broadcastGameStart() {
	players := make([]PlayerInfo, 0, len(r.players))
	for _, p := range r.players {
		players = append(players, PlayerInfo{
			ID:      p.GetID(),
			Role:    p.GetRole(),
			IsAlive: p.IsAlive(),
		})
	}

	currentPhase := r.getCurrentPhase()
	phaseInfo := PhaseInfo{
		Type:  currentPhase.GetName(),
		Round: r.round,
	}

	for playerID, p := range r.players {
		personalStartData := &SystemGameStartData{
			Players:    players,
			Phase:      phaseInfo,
			PlayerRole: p.GetRole(),
		}

		r.broadcastEvent(Event{
			Type:      EventSystemGameStart,
			Data:      personalStartData,
			PlayerID:  playerID,
			Receivers: []string{playerID},
			Timestamp: time.Now(),
		})
	}
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

func (r *Runtime) broadcastEvent(evt Event) {
	r.RLock()
	defer r.RUnlock()

	if len(evt.Receivers) == 0 {
		for id := range r.players {
			evt.Receivers = append(evt.Receivers, id)
		}
	}

	select {
	case r.systemEventChan <- evt:
	default:
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

	r.phaseIdx = (r.phaseIdx + 1) % len(r.phases)
	if r.phaseIdx == 0 {
		r.round++
	}

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
