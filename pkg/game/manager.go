package game

import (
	"errors"
	"sync"
)

// GameManager manages game state
type GameManager struct {
	// Game rules
	rules *GameRules
	// Players
	players []Player
	// Current phase
	currentPhase Phase
	// Game over flag
	isGameOver bool
	// Winner
	winner Camp
}

// NewGameManager creates new game manager
func NewGameManager() *GameManager {
	return &GameManager{
		rules:        NewGameRules(),
		players:      make([]Player, 0),
		currentPhase: PhaseNight,
		isGameOver:   false,
	}
}

// AddPlayer adds player to game
func (m *GameManager) AddPlayer(player Player) error {
	if m.isGameOver {
		return ErrInvalidGameState
	}
	m.players = append(m.players, player)
	return nil
}

// RemovePlayer removes player from game
func (m *GameManager) RemovePlayer(playerID string) error {
	if m.isGameOver {
		return ErrInvalidGameState
	}
	for i, player := range m.players {
		if player.GetID() == playerID {
			m.players = append(m.players[:i], m.players[i+1:]...)
			return nil
		}
	}
	return ErrInvalidPlayer
}

// GetPlayer returns player by ID
func (m *GameManager) GetPlayer(playerID string) (Player, error) {
	for _, player := range m.players {
		if player.GetID() == playerID {
			return player, nil
		}
	}
	return nil, ErrInvalidPlayer
}

// GetPlayers returns all players
func (m *GameManager) GetPlayers() []Player {
	return m.players
}

// GetCurrentPhase returns current phase
func (m *GameManager) GetCurrentPhase() Phase {
	return m.currentPhase
}

// NextPhase moves to next phase
func (m *GameManager) NextPhase() error {
	if m.isGameOver {
		return ErrInvalidGameState
	}
	m.currentPhase = m.rules.GetCurrentPhase()
	m.rules.NextPhase()
	return nil
}

// CheckGameOver checks if game is over
func (m *GameManager) CheckGameOver() bool {
	if m.rules.CheckGameOver(m.players) {
		m.isGameOver = true
		winner, err := m.rules.GetWinner()
		if err != nil {
			return false
		}
		m.winner = winner
		return true
	}
	return false
}

// GetWinner returns winner
func (m *GameManager) GetWinner() (Camp, error) {
	if !m.isGameOver {
		return 0, errors.New("game is not over yet")
	}
	return m.winner, nil
}

// ValidateAction validates if action is legal
func (m *GameManager) ValidateAction(player Player, target Player, skill Skill) error {
	if m.isGameOver {
		return ErrInvalidGameState
	}
	if !player.IsAlive() {
		return errors.New("player is dead")
	}
	if !target.IsAlive() {
		return errors.New("target is dead")
	}
	if m.currentPhase != PhaseNight {
		return errors.New("can only use skills at night")
	}
	return nil
}

// UseSkill uses skill
func (m *GameManager) UseSkill(player Player, target Player, skill Skill) error {
	if err := m.ValidateAction(player, target, skill); err != nil {
		return err
	}
	return skill.Put(m.currentPhase, player, target)
}

// Manager 游戏管理器，负责主导游戏进程
type Manager struct {
	// 玩家列表
	players []Player
	// 当前回合数
	round int
	// 互斥锁
	mu sync.Mutex
	// 游戏状态
	status GameStatus
	// 当前阶段
	currentPhase Phase
	// 当前行动顺序
	actionOrder []RoleType
	// 当前行动索引
	currentActionIndex int
	// 游戏是否结束
	isGameOver bool
	// 胜利阵营
	winnerCamp Camp
}

// GameStatus 游戏状态
type GameStatus int

const (
	GameStatusWaiting GameStatus = iota // 等待开始
	GameStatusRunning                   // 进行中
	GameStatusOver                      // 已结束
)

// NewManager 创建游戏管理器
func NewManager(players []Player) *Manager {
	return &Manager{
		players:      players,
		round:        1,
		status:       GameStatusWaiting,
		currentPhase: PhaseNight,
		actionOrder: []RoleType{
			RoleTypeGuard,    // 守卫先行动
			RoleTypeWerewolf, // 然后是狼人
			RoleTypeSeer,     // 预言家
			RoleTypeWitch,    // 女巫
		},
		currentActionIndex: 0,
		isGameOver:         false,
	}
}

// Start 开始游戏
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status != GameStatusWaiting {
		return errors.New("游戏已经开始或已结束")
	}

	m.status = GameStatusRunning
	return nil
}

// NextPhase 进入下一个阶段
func (m *Manager) NextPhase() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status != GameStatusRunning {
		return errors.New("游戏未开始或已结束")
	}

	// 检查游戏是否结束
	if m.checkGameOver() {
		m.status = GameStatusOver
		return nil
	}

	// 进入下一个阶段
	switch m.currentPhase {
	case PhaseNight:
		m.currentPhase = PhaseDay
	case PhaseDay:
		m.currentPhase = PhaseVote
	case PhaseVote:
		m.currentPhase = PhaseNight
		// 如果是新的一轮开始，重置所有技能和行动顺序
		m.resetAllSkills()
		m.ResetActionOrder()
		m.round++
	}

	return nil
}

// GetCurrentPhase 获取当前阶段
func (m *Manager) GetCurrentPhase() Phase {
	return m.currentPhase
}

// GetRound 获取当前回合数
func (m *Manager) GetRound() int {
	return m.round
}

// GetStatus 获取游戏状态
func (m *Manager) GetStatus() GameStatus {
	return m.status
}

// GetPlayers 获取玩家列表
func (m *Manager) GetPlayers() []Player {
	return m.players
}

// GetCurrentActionRole 获取当前应该行动的角色类型
func (m *Manager) GetCurrentActionRole() RoleType {
	return m.actionOrder[m.currentActionIndex]
}

// NextAction 进入下一个行动
func (m *Manager) NextAction() {
	m.currentActionIndex = (m.currentActionIndex + 1) % len(m.actionOrder)
}

// PlayerAction 玩家行动
func (m *Manager) PlayerAction(player Player, target Player, skill Skill) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status != GameStatusRunning {
		return errors.New("游戏未开始或已结束")
	}

	// 验证是否是当前角色的行动回合
	if player.GetRole().GetName() != string(m.GetCurrentActionRole()) {
		return errors.New("不是当前角色的行动回合")
	}

	// 验证行动是否合法
	if err := m.validateAction(player, target, skill); err != nil {
		return err
	}

	// 执行技能
	err := player.UseSkill(m.GetCurrentPhase(), target, skill)
	if err != nil {
		return err
	}

	// 进入下一个行动
	m.NextAction()
	return nil
}

// resetAllSkills 重置所有玩家的技能状态
func (m *Manager) resetAllSkills() {
	for _, player := range m.players {
		// 获取玩家所有技能并重置
		role := player.GetRole()
		for _, skillName := range role.GetAvailableSkills() {
			// 这里需要根据技能名称获取对应的技能对象并重置
			// 具体实现取决于技能的管理方式
			_ = skillName // 暂时忽略未使用的变量警告
		}
	}
}

// ResetActionOrder 重置行动顺序
func (m *Manager) ResetActionOrder() {
	m.currentActionIndex = 0
}

// GetGameResult 获取游戏结果
func (m *Manager) GetGameResult() (Camp, error) {
	if m.status != GameStatusOver {
		return 0, errors.New("游戏尚未结束")
	}
	return m.winnerCamp, nil
}

// checkGameOver 检查游戏是否结束
func (m *Manager) checkGameOver() bool {
	// 统计存活玩家阵营
	goodCount := 0
	badCount := 0

	for _, player := range m.players {
		if !player.IsAlive() {
			continue
		}

		if player.GetCamp() == CampGood {
			goodCount++
		} else {
			badCount++
		}
	}

	// 判断游戏是否结束
	if goodCount == 0 {
		m.isGameOver = true
		m.winnerCamp = CampBad
		return true
	}

	if badCount == 0 {
		m.isGameOver = true
		m.winnerCamp = CampGood
		return true
	}

	return false
}

// validateAction 验证玩家行动是否合法
func (m *Manager) validateAction(player Player, target Player, skill Skill) error {
	// 检查玩家是否存活
	if !player.IsAlive() {
		return errors.New("玩家已死亡，无法行动")
	}

	// 检查目标是否存活（除了遗言技能）
	if skill.GetName() != string(SkillTypeLastWords) && !target.IsAlive() {
		return errors.New("目标已死亡")
	}

	// 检查技能是否在当前阶段可用
	switch m.currentPhase {
	case PhaseNight:
		// 夜晚只能使用夜晚技能
		nightSkills := map[string]bool{
			string(SkillTypeKill):     true,
			string(SkillTypeCheck):    true,
			string(SkillTypeAntidote): true,
			string(SkillTypePoison):   true,
			string(SkillTypeProtect):  true,
		}
		if !nightSkills[skill.GetName()] {
			return errors.New("技能不能在夜晚使用")
		}
	case PhaseDay:
		// 白天只能使用发言技能
		if skill.GetName() != string(SkillTypeSpeak) {
			return errors.New("技能不能在白天使用")
		}
	case PhaseVote:
		// 投票阶段只能使用投票技能
		if skill.GetName() != string(SkillTypeVote) {
			return errors.New("技能不能在投票阶段使用")
		}
	}

	return nil
}
