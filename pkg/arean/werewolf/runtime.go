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
}

// NewRuntime 创建新的游戏运行时
func NewRuntime() *Runtime {
	return &Runtime{
		round:   1,
		skills:  make([]game.Skill, 0),
		players: make(map[int64]game.Player),
	}
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

// Start 开始游戏
func (r *Runtime) Start() error {
	// 初始化游戏
	if err := r.init(); err != nil {
		return err
	}

	// 游戏主循环
	return r.gameLoop()
}

// gameLoop 游戏主循环
func (r *Runtime) gameLoop() error {
	for !r.IsEnded() {
		// 等待当前阶段完成
		if err := r.waitPhaseComplete(); err != nil {
			return err
		}

		// 检查游戏是否结束
		if r.checkGameEnd() {
			break
		}

		// 进入下一阶段
		r.nextPhase()
	}

	return nil
}

// waitPhaseComplete 等待当前阶段所有行动完成
func (r *Runtime) waitPhaseComplete() error {
	for {
		if r.isPhaseActionsCompleted() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// isPhaseActionsCompleted 检查当前阶段的所有行动是否完成
func (r *Runtime) isPhaseActionsCompleted() bool {
	r.RLock()
	defer r.RUnlock()

	// 遍历所有存活玩家
	for _, p := range r.players {
		if !p.IsAlive() {
			continue
		}

		// 检查玩家在当前阶段的行动是否完成
		if !r.isPlayerActionsCompleted(p) {
			return false
		}
	}

	return true
}

// isPlayerActionsCompleted 检查玩家在当前阶段的行动是否完成
func (r *Runtime) isPlayerActionsCompleted(p game.Player) bool {
	// 获取玩家在当前阶段可用的技能
	availableSkills := r.getPlayerAvailableSkills(p)
	if len(availableSkills) == 0 {
		return true
	}

	return true
}

// getPlayerAvailableSkills 获取玩家在当前阶段可用的技能
func (r *Runtime) getPlayerAvailableSkills(p game.Player) []game.Skill {
	var availableSkills []game.Skill

	// 获取玩家角色可用的技能类型
	roleSkillTypes := p.GetRole().GetAvailableSkills()

	// 遍历所有技能实例
	for _, s := range r.skills {
		// 检查技能是否属于该玩家角色
		isRoleSkill := false
		for _, skillType := range roleSkillTypes {
			if s.GetName() == string(skillType) {
				isRoleSkill = true
				break
			}
		}
		if !isRoleSkill {
			continue
		}

		// 检查技能是否可在当前阶段使用
		if err := r.phase.ValidateAction(s); err == nil {
			availableSkills = append(availableSkills, s)
		}
	}

	return availableSkills
}

// GetPhase 获取当前阶段
func (r *Runtime) GetPhase() game.PhaseType {
	r.RLock()
	defer r.RUnlock()

	return game.PhaseType(r.phase.GetName())
}

// GetRound 获取当前回合数
func (r *Runtime) GetRound() int {
	r.RLock()
	defer r.RUnlock()

	return r.round
}

// GetWinner 获取获胜阵营
func (r *Runtime) GetWinner() game.Camp {
	r.RLock()
	defer r.RUnlock()

	return r.winner
}

// IsEnded 游戏是否结束
func (r *Runtime) IsEnded() bool {
	r.RLock()
	defer r.RUnlock()

	return r.ended
}
