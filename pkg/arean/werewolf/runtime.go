package werewolf

import (
	"errors"
	"sync"
	"time"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/phase"
	"github.com/Zereker/werewolf/pkg/game/player"
	"github.com/Zereker/werewolf/pkg/game/skill"
)

var (
	ErrGameAlreadyStarted = errors.New("werewolf already started")
	ErrGameNotStarted     = errors.New("werewolf not started")
	ErrGameEnded          = errors.New("werewolf ended")
	ErrGameInitFailed     = errors.New("werewolf init failed")
	ErrInvalidPhase       = errors.New("invalid phase")
	ErrInvalidTarget      = errors.New("invalid target")
	ErrPlayerNotFound     = errors.New("player not found")
	ErrPlayerInvalidSKill = errors.New("player not allow to found")
)

// Runtime 游戏运行时
type Runtime struct {
	sync.RWMutex

	// 玩家列表
	players map[int64]game.Player

	// 技能列表
	skills []game.Skill

	// 游戏阶段
	phase game.Phase // 当前阶段

	// 游戏状态
	started bool
	ended   bool
	round   int

	// 游戏结果
	winner game.Camp

	// 事件通道
	eventChan       chan *Event
	systemEventChan chan *Event
	subscribers     []func(*Event) // 添加订阅者列表
	stopChan        chan struct{}
}

// NewRuntime 创建新的游戏运行时
func NewRuntime() *Runtime {
	return &Runtime{
		round:           1,
		skills:          make([]game.Skill, 0),
		players:         make(map[int64]game.Player),
		eventChan:       make(chan *Event, 100),
		systemEventChan: make(chan *Event, 100),
		subscribers:     make([]func(*Event), 0),
		stopChan:        make(chan struct{}),
	}
}

func (r *Runtime) initSkills() ([]game.Skill, error) {
	var result []game.Skill

	skillTypeMap := make(map[game.SkillType]struct{})
	for _, p := range r.players {
		for _, skillType := range p.GetRole().GetAvailableSkills() {
			if _, exists := skillTypeMap[skillType]; exists {
				continue
			}
			skillTypeMap[skillType] = struct{}{}

			// 创建技能实例
			s, err := skill.New(skillType)
			if err != nil {
				return nil, ErrGameInitFailed
			}

			result = append(result, s)
		}
	}

	return result, nil
}

// buildPhaseList 构建游戏阶段链表
func (r *Runtime) buildPhaseList() game.Phase {
	// 按技能使用阶段分类
	skillMap := make(map[game.PhaseType][]game.Skill)
	for _, s := range r.skills {
		p := s.UseInPhase()
		skillMap[p] = append(skillMap[p], s)
	}

	// 创建各个阶段
	night := phase.NewPhase(string(game.PhaseNight), skillMap[game.PhaseNight])
	day := phase.NewPhase(string(game.PhaseDay), skillMap[game.PhaseDay])
	vote := phase.NewPhase(string(game.PhaseVote), skillMap[game.PhaseVote])

	// 构建循环链表
	night.SetNextPhase(day)
	day.SetNextPhase(vote)
	vote.SetNextPhase(night)

	return night
}

// init 游戏初始化
func (r *Runtime) init() error {
	r.Lock()
	defer r.Unlock()

	var err error
	if r.started {
		return ErrGameAlreadyStarted
	}

	// 技能初始化
	r.skills, err = r.initSkills()
	if err != nil {
		return ErrGameInitFailed
	}

	// 游戏阶段初始化
	r.phase = r.buildPhaseList()
	r.started = true
	return nil
}

// AddPlayer 添加玩家
func (r *Runtime) AddPlayer(id int64, role game.Role) error {
	r.Lock()
	defer r.Unlock()

	if r.started {
		return ErrGameAlreadyStarted
	}

	r.players[id] = player.New(id, role)
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

func (r *Runtime) findSkill(userID int64, skillType game.SkillType) (game.Skill, error) {
	// 检查玩家是否存在
	p, exists := r.players[userID]
	if !exists {
		return nil, ErrPlayerNotFound
	}

	// 检查玩家是否有权限使用该技能
	hasPermission := false
	for _, availableSkill := range p.GetRole().GetAvailableSkills() {
		if availableSkill == skillType {
			hasPermission = true
			break
		}
	}

	if !hasPermission {
		return nil, ErrPlayerInvalidSKill
	}

	// 查找对应类型的技能实例
	for _, s := range r.skills {
		if s.GetName() == string(skillType) {
			return s, nil
		}
	}

	return nil, ErrPlayerInvalidSKill
}

// useSkill 使用技能
func (r *Runtime) useSkill(casterID int64, targetID int64, skillType game.SkillType) error {
	r.Lock()
	defer r.Unlock()

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

	s, err := r.findSkill(casterID, skillType)
	if err != nil {
		return ErrPlayerInvalidSKill
	}

	// 使用技能
	return s.Put(r.phase.GetName(), caster, target)
}

// checkGameEnd 检查游戏是否结束
func (r *Runtime) checkGameEnd() bool {
	r.RLock()
	defer r.RUnlock()

	var goodCount, badCount int
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

	if badCount == 0 {
		r.winner = game.CampGood
		r.ended = true
		return true
	}

	if badCount >= goodCount {
		r.winner = game.CampBad
		r.ended = true
		return true
	}

	return false
}

// eventLoop 事件处理循环
func (r *Runtime) eventLoop() {
	for {
		select {
		case evt := <-r.eventChan:
			r.handleEvent(evt)
		case <-r.stopChan:
			return
		}
	}
}

// handleEvent 处理事件
func (r *Runtime) handleEvent(evt *Event) {
	r.Lock()
	defer r.Unlock()

	switch evt.Type {
	case EventUserSkill:
		r.handleUserSkill(evt)
	case EventUserVote:
		r.handleUserVote(evt)
	case EventUserSpeak:
		r.handleUserSpeak(evt)
	case EventUserReady:
		r.handleUserReady(evt)
	case EventUserUnready:
		r.handleUserUnready(evt)
	}
}

// handleUserSkill 处理用户技能事件
func (r *Runtime) handleUserSkill(evt *Event) {
	if data, ok := evt.Data.(*UserSkillData); ok {
		// 处理技能使用
		err := r.useSkill(evt.PlayerID, data.TargetID, data.SkillType)

		// 发送技能使用结果
		resultData := &SystemSkillResultData{
			SkillType: data.SkillType,
			Success:   err == nil,
			Message:   r.getSkillResultMessage(err),
		}
		r.emitSystemEvent(EventSystemSkillResult, evt.PlayerID, data.TargetID, resultData)

		// 检查阶段是否完成
		if r.isPhaseCompleted() {
			r.completeCurrentPhase()
		}
	}
}

// handleUserVote 处理用户投票事件
func (r *Runtime) handleUserVote(evt *Event) {
	if data, ok := evt.Data.(*UserVoteData); ok {
		err := r.useSkill(evt.PlayerID, data.TargetID, game.SkillTypeVote)

		// 发送投票结果
		resultData := &SystemSkillResultData{
			SkillType: game.SkillTypeVote,
			Success:   err == nil,
			Message:   r.getSkillResultMessage(err),
		}
		r.emitSystemEvent(EventSystemSkillResult, evt.PlayerID, data.TargetID, resultData)

		// 检查阶段是否完成
		if r.isPhaseCompleted() {
			r.completeCurrentPhase()
		}
	}
}

// handleUserSpeak 处理用户发言事件
func (r *Runtime) handleUserSpeak(evt *Event) {
	if _, ok := evt.Data.(*UserSpeakData); ok {
		err := r.useSkill(evt.PlayerID, evt.PlayerID, game.SkillTypeSpeak)

		// 发送发言结果
		resultData := &SystemSkillResultData{
			SkillType: game.SkillTypeSpeak,
			Success:   err == nil,
			Message:   r.getSkillResultMessage(err),
		}
		r.emitSystemEvent(EventSystemSkillResult, evt.PlayerID, evt.PlayerID, resultData)
	}
}

// handleUserReady 处理用户准备事件
func (r *Runtime) handleUserReady(evt *Event) {
	// TODO: 实现玩家准备逻辑
}

// handleUserUnready 处理用户取消准备事件
func (r *Runtime) handleUserUnready(evt *Event) {
	// TODO: 实现玩家取消准备逻辑
}

// getSkillResultMessage 获取技能使用结果消息
func (r *Runtime) getSkillResultMessage(err error) string {
	if err == nil {
		return "操作成功"
	}
	return err.Error()
}

// completeCurrentPhase 完成当前阶段
func (r *Runtime) completeCurrentPhase() {
	// 发送阶段结束事件
	phaseEndData := &SystemPhaseData{
		Phase:     r.phase.GetName(),
		Round:     r.round,
		Timestamp: time.Now(),
	}
	r.emitSystemEvent(EventSystemPhaseEnd, 0, 0, phaseEndData)

	// 检查游戏是否结束
	if r.checkGameEnd() {
		gameEndData := &SystemGameEndData{
			Winner:    r.winner,
			Round:     r.round,
			Timestamp: time.Now(),
		}
		r.emitSystemEvent(EventSystemGameEnd, 0, 0, gameEndData)
		return
	}

	// 进入下一阶段
	r.nextPhase()

	// 发送新阶段开始事件
	phaseStartData := &SystemPhaseData{
		Phase:     r.phase.GetName(),
		Round:     r.round,
		Timestamp: time.Now(),
		Duration:  r.getPhaseDefaultDuration(r.phase.GetName()),
	}
	r.emitSystemEvent(EventSystemPhaseStart, 0, 0, phaseStartData)
}

// getPhaseDefaultDuration 获取阶段默认持续时间（秒）
func (r *Runtime) getPhaseDefaultDuration(phase game.PhaseType) int {
	switch phase {
	case game.PhaseNight:
		return 30 // 夜晚阶段30秒
	case game.PhaseDay:
		return 120 // 白天阶段120秒
	case game.PhaseVote:
		return 30 // 投票阶段30秒
	default:
		return 60
	}
}

// getPlayersInfo 获取玩家信息列表
func (r *Runtime) getPlayersInfo() []PlayerInfo {
	players := make([]PlayerInfo, 0, len(r.players))
	for _, p := range r.players {
		players = append(players, PlayerInfo{
			ID:      p.GetID(),
			Role:    p.GetRole(),
			IsAlive: p.IsAlive(),
		})
	}
	return players
}

// 用户接口方法
func (r *Runtime) UseSkill(playerID int64, targetID int64, skillType game.SkillType) {
	data := &UserSkillData{
		SkillType: skillType,
		TargetID:  targetID,
	}
	evt := NewEvent(EventUserSkill, playerID, targetID, data)
	r.emitUserEvent(evt)
}

func (r *Runtime) Vote(playerID int64, targetID int64) {
	data := &UserVoteData{
		TargetID: targetID,
	}
	evt := NewEvent(EventUserVote, playerID, targetID, data)
	r.emitUserEvent(evt)
}

func (r *Runtime) Speak(playerID int64, message string) {
	data := &UserSpeakData{
		Message: message,
	}
	evt := NewEvent(EventUserSpeak, playerID, 0, data)
	r.emitUserEvent(evt)
}

// Subscribe 订阅事件
func (r *Runtime) Subscribe(handler func(*Event)) {
	r.Lock()
	defer r.Unlock()
	r.subscribers = append(r.subscribers, handler)
}

// Unsubscribe 取消订阅
func (r *Runtime) Unsubscribe(handler func(*Event)) {
	r.Lock()
	defer r.Unlock()
	for i, h := range r.subscribers {
		if &h == &handler {
			r.subscribers = append(r.subscribers[:i], r.subscribers[i+1:]...)
			break
		}
	}
}

// emitSystemEvent 发送系统事件
func (r *Runtime) emitSystemEvent(eventType EventType, playerID int64, targetID int64, data interface{}) {
	evt := NewEvent(eventType, playerID, targetID, data)

	// 通知所有订阅者
	r.RLock()
	subscribers := make([]func(*Event), len(r.subscribers))
	copy(subscribers, r.subscribers)
	r.RUnlock()

	for _, handler := range subscribers {
		handler(evt)
	}

	// 发送到系统事件通道
	select {
	case r.systemEventChan <- evt:
	default:
		// 通道已满，可以记录日志
	}
}

// emitUserEvent 发送用户事件
func (r *Runtime) emitUserEvent(evt *Event) {
	// 通知所有订阅者
	r.RLock()
	subscribers := make([]func(*Event), len(r.subscribers))
	copy(subscribers, r.subscribers)
	r.RUnlock()

	for _, handler := range subscribers {
		handler(evt)
	}

	// 发送到用户事件通道
	select {
	case r.eventChan <- evt:
	default:
		// 通道已满，可以记录日志
	}
}

// Start 开始游戏
func (r *Runtime) Start() error {
	if err := r.init(); err != nil {
		return err
	}

	// 发送游戏开始事件
	startData := &SystemGameStartData{
		Players: r.getPlayersInfo(),
		Phase: PhaseInfo{
			Type:      r.phase.GetName(),
			Round:     r.round,
			StartTime: time.Now(),
			Duration:  r.getPhaseDefaultDuration(r.phase.GetName()),
		},
	}
	r.emitSystemEvent(EventSystemGameStart, 0, 0, startData)

	// 启动事件处理循环
	go r.eventLoop()
	return nil
}

// Stop 停止游戏
func (r *Runtime) Stop() {
	close(r.stopChan)
	close(r.eventChan)
	close(r.systemEventChan)
}

// isPhaseCompleted 检查当前阶段是否完成
func (r *Runtime) isPhaseCompleted() bool {
	currentPhase := r.phase.GetName()

	// 检查所有存活玩家是否都完成了当前阶段的必要操作
	for _, p := range r.players {
		if !p.IsAlive() {
			continue
		}

		// 根据不同阶段检查必要操作
		switch currentPhase {
		case game.PhaseNight:
			// 夜晚阶段检查
			if !r.isNightPhaseCompleted(p) {
				return false
			}
		case game.PhaseVote:
			// 投票阶段检查
			if !r.isVotePhaseCompleted(p) {
				return false
			}
		case game.PhaseDay:
			// 白天阶段无强制操作
			continue
		}
	}
	return true
}

// isNightPhaseCompleted 检查夜晚阶段玩家是否完成必要操作
func (r *Runtime) isNightPhaseCompleted(p game.Player) bool {
	role := p.GetRole()
	availableSkills := role.GetAvailableSkills()

	// 检查必要技能是否已使用
	for _, skillType := range availableSkills {
		switch skillType {
		case game.SkillTypeKill:
			// 狼人必须杀人
			if role.GetName() == string(game.RoleTypeWerewolf) {
				if !r.isSkillUsed(p, skillType) {
					return false
				}
			}
		case game.SkillTypeCheck:
			// 预言家必须验人
			if role.GetName() == string(game.RoleTypeSeer) {
				if !r.isSkillUsed(p, skillType) {
					return false
				}
			}
		}
	}
	return true
}

// isVotePhaseCompleted 检查投票阶段玩家是否完成必要操作
func (r *Runtime) isVotePhaseCompleted(p game.Player) bool {
	// 所有玩家都必须投票
	return r.isSkillUsed(p, game.SkillTypeVote)
}

// isSkillUsed 检查玩家是否已使用某个技能
func (r *Runtime) isSkillUsed(p game.Player, skillType game.SkillType) bool {
	for _, s := range r.skills {
		if s.GetName() == string(skillType) {
			return s.IsUsed()
		}
	}
	return false
}
