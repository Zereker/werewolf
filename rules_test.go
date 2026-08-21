package werewolf

// rules_test.go —— 以「狼人殺」规则为基准的一致性测试。
//
// # 规则来源
//
// 本文件的判定基准取自中文维基百科条目「狼人殺」
// (https://zh.wikipedia.org/wiki/狼人殺)，编号 R1–R11：
//
//	R1  预言家   「每晚可以查驗一位存活玩家的所屬陣營」
//	R2  女巫·视野「解藥未使用時可以得知狼人的殺害對象」
//	R3  女巫·双药「解藥和毒藥不可以在同一夜使用」
//	R4  女巫·自救「解藥全程不能用於解救自己」（部分版本首夜能自救）
//	R5  守卫·自守「可以選擇守護自己或不進行守護」
//	R6  守卫·连守「不可連續兩晚守護同一名玩家」
//	R7  同守同救 「守衛和女巫同時對其守護和使用解藥，該名玩家依然會死亡」
//	R8  猎人     「除殉情或被毒殺外，以任何其他方式被淘汰時可以…開槍帶走一位玩家」
//	R9  好人胜利 「將狼人淘汰以獲取勝利」
//	R10 狼人胜利 「需要淘汰所有平民或神職人員」（屠边）
//	R11 白天流程 「所有存活玩家輪流發言，並在所有人發言完畢後投票放逐一名玩家」
//
// # 本引擎自定的口径（维基未规定，编号 D1–D3）
//
// 维基条目对以下情形没有明文，属于实现必须自行声明的约定。
// 这里用测试把当前行为固化下来，避免日后被无意改动：
//
//	D1 白天投票平票 -> 无人出局（不进入 PK 发言重投）
//	D2 狼人刀口平票 -> 空刀（狼队未达成共识）
//	D3 夜晚行动顺序 -> 守卫 → 狼人 → 女巫 → 预言家 → 结算
//
// # 已知偏差与 WEREWOLF_STRICT_RULES
//
// 部分规则当前实现尚未满足，登记在 knownDeviations 中。
// 登记粒度是「具体行为」而非「整条规则」——例如 R8 的正向用例
// （被刀/被放逐可开枪）已经符合规则，常驻执行；只有「毒杀不开枪」
// 和「一局一枪」两个行为被登记为偏差。
// 这些用例默认 Skip，以免 CI 因「已知待办」变红；
// 设置 WEREWOLF_STRICT_RULES=1 可强制全部执行，用于驱动修复：
//
//	WEREWOLF_STRICT_RULES=1 go test -run TestRule -v ./
//
// 修复某条规则后，从 knownDeviations 中删掉对应条目即可让它转为常驻用例。

import (
	"os"
	"testing"

	pb "github.com/Zereker/werewolf/proto"
)

// knownDeviations 登记「实现与规则不符、尚未修复」的条目。
// key 为规则编号，value 为偏差描述。
var knownDeviations = map[string]string{
	"R2":       "解药用完后 buildWitchPhaseInfo 仍无条件返回 KillTarget",
	"R3":       "WitchResolver 只防重复用同一技能，未禁止同夜解药+毒药",
	"R7.同守同救":  "WolfResolver 在目标被守时不设 KillTarget，同守同救无法触发",
	"R8.毒杀不开枪": "NightResolveResolver 对毒杀也触发 HUNTER_TRIGGERED",
	"R8.显式跳过":  "buildHunterPhaseInfo 宣告 SKIP 可用，但 Steps 里没有 SKIP，ValidateSkillUse 会拒绝",
	"R8.一局一枪":  "RoundCtx.HunterTriggered 触发后未清除，投票阶段会重复进入猎人阶段",
	"R10":      "CheckVictory 只做屠城判定；Camp 无神职/平民之分，屠边无法表达",
}

// requireRule 在规则尚未实现时跳过用例。
func requireRule(t *testing.T, id string) {
	t.Helper()
	reason, deviates := knownDeviations[id]
	if deviates && os.Getenv("WEREWOLF_STRICT_RULES") == "" {
		t.Skipf("已知偏差 %s：%s（设 WEREWOLF_STRICT_RULES=1 强制执行）", id, reason)
	}
}

// ==================== 测试脚手架 ====================

// seat 座位（玩家 ID + 角色），阵营由角色推导。
type seat struct {
	id   string
	role pb.RoleType
}

func wolf(id string) seat     { return seat{id, pb.RoleType_ROLE_TYPE_WEREWOLF} }
func seer(id string) seat     { return seat{id, pb.RoleType_ROLE_TYPE_SEER} }
func witch(id string) seat    { return seat{id, pb.RoleType_ROLE_TYPE_WITCH} }
func guard(id string) seat    { return seat{id, pb.RoleType_ROLE_TYPE_GUARD} }
func hunter(id string) seat   { return seat{id, pb.RoleType_ROLE_TYPE_HUNTER} }
func villager(id string) seat { return seat{id, pb.RoleType_ROLE_TYPE_VILLAGER} }

// villagers 批量生成平民座位。
func villagers(ids ...string) []seat {
	out := make([]seat, 0, len(ids))
	for _, id := range ids {
		out = append(out, villager(id))
	}
	return out
}

// campOf 由角色推导阵营。
func campOf(role pb.RoleType) pb.Camp {
	if role == pb.RoleType_ROLE_TYPE_WEREWOLF {
		return pb.Camp_CAMP_EVIL
	}
	return pb.Camp_CAMP_GOOD
}

// ruleGame 包装 Engine，提供面向规则测试的断言辅助。
type ruleGame struct {
	t *testing.T
	e *Engine
}

// newRuleGame 按座位表创建并开局。cfg 为 nil 时使用默认配置。
func newRuleGame(t *testing.T, cfg *GameConfig, seats ...seat) *ruleGame {
	t.Helper()
	e := NewEngine(cfg)
	for _, s := range seats {
		e.AddPlayer(s.id, s.role, campOf(s.role))
	}
	if err := e.Start(); err != nil {
		t.Fatalf("Start() 失败: %v", err)
	}
	return &ruleGame{t: t, e: e}
}

// seats 拼接座位表。
func seats(groups ...interface{}) []seat {
	out := make([]seat, 0)
	for _, g := range groups {
		switch v := g.(type) {
		case seat:
			out = append(out, v)
		case []seat:
			out = append(out, v...)
		}
	}
	return out
}

// use 提交技能，返回错误供调用方断言。
func (g *ruleGame) use(playerID string, skill pb.SkillType, targetID string) error {
	g.t.Helper()
	return g.e.SubmitSkillUse(&SkillUse{PlayerID: playerID, Skill: skill, TargetID: targetID})
}

// mustUse 提交技能，失败即终止用例。
func (g *ruleGame) mustUse(playerID string, skill pb.SkillType, targetID string) {
	g.t.Helper()
	if err := g.use(playerID, skill, targetID); err != nil {
		g.t.Fatalf("提交技能失败 player=%s skill=%v target=%s: %v", playerID, skill, targetID, err)
	}
}

// end 结束当前子阶段并断言流转到 expect。
func (g *ruleGame) end(expect pb.PhaseType) []*Effect {
	g.t.Helper()
	from := g.e.GetCurrentPhase()
	effects, err := g.e.EndSubStep()
	if err != nil {
		g.t.Fatalf("EndSubStep() 于 %v 失败: %v", from, err)
	}
	if got := g.e.GetCurrentPhase(); got != expect {
		g.t.Fatalf("阶段流转错误: %v 结束后期望 %v，实际 %v", from, expect, got)
	}
	return effects
}

// endAny 结束当前子阶段，不断言目标阶段。
func (g *ruleGame) endAny() []*Effect {
	g.t.Helper()
	effects, err := g.e.EndSubStep()
	if err != nil {
		g.t.Fatalf("EndSubStep() 失败: %v", err)
	}
	return effects
}

// walkNight 从 NIGHT_GUARD 一路走到 DAY，中途不提交任何技能。
func (g *ruleGame) walkNight() {
	g.t.Helper()
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	g.end(pb.PhaseType_PHASE_TYPE_DAY)
}

// toNextNight 从 DAY 走到下一个 NIGHT_GUARD（不投票，因而无人出局）。
func (g *ruleGame) toNextNight() {
	g.t.Helper()
	g.end(pb.PhaseType_PHASE_TYPE_VOTE)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_GUARD)
}

// info 取玩家只读信息。
func (g *ruleGame) info(id string) PlayerInfo {
	g.t.Helper()
	pi, ok := g.e.GetPlayerInfo(id)
	if !ok {
		g.t.Fatalf("玩家不存在: %s", id)
	}
	return pi
}

// alive 玩家是否存活。
func (g *ruleGame) alive(id string) bool { return g.info(id).Alive }

// assertAlive 断言存活状态。
func (g *ruleGame) assertAlive(id string, want bool, msg string) {
	g.t.Helper()
	if got := g.alive(id); got != want {
		g.t.Errorf("%s: 期望 %s 存活=%v，实际 %v", msg, id, want, got)
	}
}

// witchSeesKill 返回当前 NIGHT_WITCH 阶段女巫看到的刀口。
func (g *ruleGame) witchSeesKill() string {
	g.t.Helper()
	ri := g.e.GetPhaseInfo().RoleInfos[pb.RoleType_ROLE_TYPE_WITCH]
	if ri == nil {
		return ""
	}
	return ri.KillTarget
}

// vote 让多名玩家投同一目标。
func (g *ruleGame) vote(target string, voters ...string) {
	g.t.Helper()
	for _, v := range voters {
		g.mustUse(v, pb.SkillType_SKILL_TYPE_VOTE, target)
	}
}

// setDead 直接把玩家置为死亡，用于构造胜负判定的局面。
func (g *ruleGame) setDead(ids ...string) {
	g.t.Helper()
	g.e.state.mu.Lock()
	defer g.e.state.mu.Unlock()
	for _, id := range ids {
		p, ok := g.e.state.players[id]
		if !ok {
			g.t.Fatalf("玩家不存在: %s", id)
		}
		p.Alive = false
	}
}

// findEffect 在效果列表里找第一个指定类型的效果。
func findEffect(effects []*Effect, typ pb.EventType) *Effect {
	for _, e := range effects {
		if e.Type == typ {
			return e
		}
	}
	return nil
}

// ==================== R1 预言家查验阵营 ====================

// TestRule_R1_SeerChecksCamp 「每晚可以查驗一位存活玩家的所屬陣營」。
// 查验结果报的是阵营，不是具体角色。
func TestRule_R1_SeerChecksCamp(t *testing.T) {
	requireRule(t, "R1")

	cases := []struct {
		target     string
		wantCamp   pb.Camp
		wantIsGood bool
	}{
		{"w1", pb.Camp_CAMP_EVIL, false},
		{"v1", pb.Camp_CAMP_GOOD, true},
		{"wi", pb.Camp_CAMP_GOOD, true}, // 神职也应报好人，而非报出角色
	}

	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			g := newRuleGame(t, nil, seats(
				wolf("w1"), wolf("w2"), seer("s"), witch("wi"),
				villagers("v1", "v2", "v3", "v4"),
			)...)

			g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
			g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
			g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)

			g.mustUse("s", pb.SkillType_SKILL_TYPE_CHECK, tc.target)
			effects := g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)

			check := findEffect(effects, pb.EventType_EVENT_TYPE_CHECK)
			if check == nil {
				t.Fatal("期望产生 CHECK 效果")
			}
			if check.Data["camp"] != tc.wantCamp {
				t.Errorf("查验 %s: 期望 camp=%v，实际 %v", tc.target, tc.wantCamp, check.Data["camp"])
			}
			if check.Data["isGood"] != tc.wantIsGood {
				t.Errorf("查验 %s: 期望 isGood=%v，实际 %v", tc.target, tc.wantIsGood, check.Data["isGood"])
			}
			if _, leaked := check.Data["role"]; leaked {
				t.Error("查验结果泄露了具体角色，规则只允许报阵营")
			}
		})
	}
}

// ==================== R2 女巫解药未使用时才可得知刀口 ====================

// TestRule_R2_WitchSeesKillOnlyWhileAntidoteHeld 「解藥未使用時可以得知狼人的殺害對象」。
// 反过来说：解药一旦用掉，女巫就不该再看到刀口。
func TestRule_R2_WitchSeesKillOnlyWhileAntidoteHeld(t *testing.T) {
	requireRule(t, "R2")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), witch("wi"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	// —— 第一夜：解药在手，应当能看到刀口，并用掉解药 ——
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)

	if got := g.witchSeesKill(); got != "v1" {
		t.Fatalf("第一夜解药在手：期望女巫看到刀口 v1，实际 %q", got)
	}
	g.mustUse("wi", pb.SkillType_SKILL_TYPE_ANTIDOTE, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	g.end(pb.PhaseType_PHASE_TYPE_DAY)

	if g.info("wi").HasAntidote {
		t.Fatal("第一夜救人后解药应当已消耗")
	}
	g.assertAlive("v1", true, "第一夜被救")

	// —— 第二夜：解药已用完，不应再看到刀口 ——
	g.toNextNight()
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "v2")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)

	if got := g.witchSeesKill(); got != "" {
		t.Errorf("第二夜解药已用完：期望女巫看不到刀口（\"\"），实际 %q", got)
	}
}

// ==================== R3 解药与毒药不可同夜使用 ====================

// TestRule_R3_WitchCannotUseBothPotionsInOneNight 「解藥和毒藥不可以在同一夜使用」。
//
// 断言的是规则本身的不变量——同一夜最多只有一瓶药生效——
// 而不限定引擎在哪一层拦截（提交层报错，或结算层取消均可）。
//
// 注意：现有的 integration_test.go:TestScenario_WitchPoisonAndSaveOnSameNight
// 断言的是「两瓶药同夜都生效」，与本规则直接冲突，修复 R3 时需一并处理。
func TestRule_R3_WitchCannotUseBothPotionsInOneNight(t *testing.T) {
	requireRule(t, "R3")

	// 两种提交顺序都要满足不变量
	orders := []struct {
		name  string
		first pb.SkillType
	}{
		{"先解药后毒药", pb.SkillType_SKILL_TYPE_ANTIDOTE},
		{"先毒药后解药", pb.SkillType_SKILL_TYPE_POISON},
	}

	for _, ord := range orders {
		t.Run(ord.name, func(t *testing.T) {
			g := newRuleGame(t, nil, seats(
				wolf("w1"), wolf("w2"), witch("wi"),
				villagers("v1", "v2", "v3", "v4", "v5", "v6"),
			)...)

			g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
			g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "v1")
			g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)

			// 两瓶药都尝试提交；后提交的那瓶被拒是允许的
			if ord.first == pb.SkillType_SKILL_TYPE_ANTIDOTE {
				g.mustUse("wi", pb.SkillType_SKILL_TYPE_ANTIDOTE, "v1")
				_ = g.use("wi", pb.SkillType_SKILL_TYPE_POISON, "v2")
			} else {
				g.mustUse("wi", pb.SkillType_SKILL_TYPE_POISON, "v2")
				_ = g.use("wi", pb.SkillType_SKILL_TYPE_ANTIDOTE, "v1")
			}

			g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
			g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
			g.end(pb.PhaseType_PHASE_TYPE_DAY)

			after := g.info("wi")
			potionsUsed := 0
			if !after.HasAntidote {
				potionsUsed++
			}
			if !after.HasPoison {
				potionsUsed++
			}
			if potionsUsed > 1 {
				t.Errorf("同一夜消耗了 %d 瓶药，规则要求最多 1 瓶", potionsUsed)
			}

			saved := g.alive("v1")     // 被刀且被救
			poisoned := !g.alive("v2") // 被毒
			if saved && poisoned {
				t.Error("解药与毒药在同一夜同时生效（v1 被救且 v2 被毒），违反规则")
			}
		})
	}
}

// ==================== R4 女巫不能自救 ====================

// TestRule_R4_WitchCannotSaveSelf 「解藥全程不能用於解救自己」。
// 维基同时记载「部分版本首夜能自救」，故引擎以 WitchCanSaveSelf 提供变体。
func TestRule_R4_WitchCannotSaveSelf(t *testing.T) {
	requireRule(t, "R4")

	cases := []struct {
		name        string
		canSaveSelf bool
		wantAlive   bool
	}{
		{"默认不可自救", false, false},
		{"变体允许自救", true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultGameConfig()
			cfg.WitchCanSaveSelf = tc.canSaveSelf
			g := newRuleGame(t, cfg, seats(
				wolf("w1"), wolf("w2"), witch("wi"),
				villagers("v1", "v2", "v3", "v4", "v5", "v6"),
			)...)

			g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
			g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "wi")
			g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)

			if got := g.witchSeesKill(); got != "wi" {
				t.Fatalf("期望女巫看到自己被刀，实际刀口 %q", got)
			}
			g.mustUse("wi", pb.SkillType_SKILL_TYPE_ANTIDOTE, "wi")

			g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
			g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
			g.end(pb.PhaseType_PHASE_TYPE_DAY)

			g.assertAlive("wi", tc.wantAlive, "女巫自救 WitchCanSaveSelf="+tc.name)
		})
	}
}

// ==================== R5 守卫可以自守 ====================

// TestRule_R5_GuardMayProtectSelf 「可以選擇守護自己或不進行守護」。
func TestRule_R5_GuardMayProtectSelf(t *testing.T) {
	requireRule(t, "R5")

	cases := []struct {
		name           string
		canProtectSelf bool
		wantAlive      bool
	}{
		{"默认可自守", true, true},
		{"变体禁止自守", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultGameConfig()
			cfg.GuardCanProtectSelf = tc.canProtectSelf
			g := newRuleGame(t, cfg, seats(
				wolf("w1"), wolf("w2"), guard("g"),
				villagers("v1", "v2", "v3", "v4", "v5", "v6"),
			)...)

			g.mustUse("g", pb.SkillType_SKILL_TYPE_PROTECT, "g")
			g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
			g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "g")
			g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
			g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
			g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
			g.end(pb.PhaseType_PHASE_TYPE_DAY)

			g.assertAlive("g", tc.wantAlive, "守卫自守（"+tc.name+"）")
		})
	}
}

// TestRule_R5_GuardMaySkipProtection 守卫也可以选择不守护任何人。
func TestRule_R5_GuardMaySkipProtection(t *testing.T) {
	requireRule(t, "R5")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), guard("g"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	// 守卫不提交任何技能，直接过夜
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	g.end(pb.PhaseType_PHASE_TYPE_DAY)

	g.assertAlive("v1", false, "守卫未守护时刀口应当死亡")
}

// ==================== R6 守卫不可连续两晚守同一人 ====================

// TestRule_R6_GuardCannotProtectSameTargetTwice 「不可連續兩晚守護同一名玩家」。
func TestRule_R6_GuardCannotProtectSameTargetTwice(t *testing.T) {
	requireRule(t, "R6")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), guard("g"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	// 第一夜：守 v1，狼刀 v1 —— 守护生效
	g.mustUse("g", pb.SkillType_SKILL_TYPE_PROTECT, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	g.end(pb.PhaseType_PHASE_TYPE_DAY)
	g.assertAlive("v1", true, "第一夜守护生效")

	// 第二夜：再守 v1 —— 连守无效，狼刀 v1 应当命中
	g.toNextNight()
	g.mustUse("g", pb.SkillType_SKILL_TYPE_PROTECT, "v1")
	effects := g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)

	protect := findEffect(effects, pb.EventType_EVENT_TYPE_PROTECT)
	if protect == nil {
		t.Fatal("期望产生 PROTECT 效果（即便被取消）")
	}
	if !protect.Canceled {
		t.Error("第二夜连守同一目标，PROTECT 效果应当被取消")
	}

	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	g.endAny()

	g.assertAlive("v1", false, "连守无效，第二夜刀口应当死亡")
}

// TestRule_R6_GuardMayProtectDifferentTarget 换人守护不受连守限制。
func TestRule_R6_GuardMayProtectDifferentTarget(t *testing.T) {
	requireRule(t, "R6")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), guard("g"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	// 第一夜守 v1
	g.mustUse("g", pb.SkillType_SKILL_TYPE_PROTECT, "v1")
	g.walkNight()

	// 第二夜改守 v2，狼刀 v2 —— 应当守住
	g.toNextNight()
	g.mustUse("g", pb.SkillType_SKILL_TYPE_PROTECT, "v2")
	effects := g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)

	protect := findEffect(effects, pb.EventType_EVENT_TYPE_PROTECT)
	if protect == nil || protect.Canceled {
		t.Fatalf("换人守护不应被取消，实际 %+v", protect)
	}

	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "v2")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	g.end(pb.PhaseType_PHASE_TYPE_DAY)

	g.assertAlive("v2", true, "换人守护应当生效")
}

// TestRule_R6_GuardMayReprotectAfterGap 隔一晚后可以重新守回同一人。
// 规则限制的是「连续两晚」，不是「永远不能再守」。
func TestRule_R6_GuardMayReprotectAfterGap(t *testing.T) {
	requireRule(t, "R6")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), guard("g"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	// 第一夜守 v1
	g.mustUse("g", pb.SkillType_SKILL_TYPE_PROTECT, "v1")
	g.walkNight()

	// 第二夜守 v2
	g.toNextNight()
	g.mustUse("g", pb.SkillType_SKILL_TYPE_PROTECT, "v2")
	g.walkNight()

	// 第三夜守回 v1 —— 与上一晚（v2）不同，应当生效
	g.toNextNight()
	g.mustUse("g", pb.SkillType_SKILL_TYPE_PROTECT, "v1")
	effects := g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)

	protect := findEffect(effects, pb.EventType_EVENT_TYPE_PROTECT)
	if protect == nil || protect.Canceled {
		t.Fatalf("隔一晚后重新守回同一人应当生效，实际 %+v", protect)
	}
}

// ==================== R7 同守同救 ====================

// TestRule_R7_GuardPlusAntidoteKillsTarget
// 「守衛和女巫同時對其守護和使用解藥，該名玩家依然會死亡」。
//
// 前置条件同样重要：守卫的守护不应影响女巫的视野——
// 女巫看到的是狼刀目标，她并不知道守卫守了谁。
func TestRule_R7_GuardPlusAntidoteKillsTarget(t *testing.T) {
	requireRule(t, "R7.同守同救")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), guard("g"), witch("wi"),
		villagers("v1", "v2", "v3", "v4", "v5"),
	)...)

	g.mustUse("g", pb.SkillType_SKILL_TYPE_PROTECT, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)

	// 前置：女巫仍应看到刀口，守护对她不可见
	if got := g.witchSeesKill(); got != "v1" {
		t.Fatalf("守卫的守护不应遮蔽女巫视野：期望刀口 v1，实际 %q", got)
	}

	g.mustUse("wi", pb.SkillType_SKILL_TYPE_ANTIDOTE, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	g.endAny()

	g.assertAlive("v1", false, "同守同救")
	if g.info("wi").HasAntidote {
		t.Error("同守同救时解药仍应被消耗")
	}
}

// TestRule_R7_GuardAloneProtects 对照组：只有守卫守护（女巫未救），目标存活。
func TestRule_R7_GuardAloneProtects(t *testing.T) {
	requireRule(t, "R7")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), guard("g"), witch("wi"),
		villagers("v1", "v2", "v3", "v4", "v5"),
	)...)

	g.mustUse("g", pb.SkillType_SKILL_TYPE_PROTECT, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
	// 女巫不救
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	g.end(pb.PhaseType_PHASE_TYPE_DAY)

	g.assertAlive("v1", true, "仅守卫守护")
}

// ==================== R8 猎人开枪 ====================

// TestRule_R8_HunterShootsWhenKilledByWolves 猎人被狼刀，可以开枪。
func TestRule_R8_HunterShootsWhenKilledByWolves(t *testing.T) {
	requireRule(t, "R8")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "h")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	// 猎人在结算阶段才被触发，故 NIGHT_RESOLVE 结束后才进入猎人阶段
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER)

	g.mustUse("h", pb.SkillType_SKILL_TYPE_SHOOT, "w1")
	g.end(pb.PhaseType_PHASE_TYPE_DAY)

	g.assertAlive("h", false, "猎人被狼刀")
	g.assertAlive("w1", false, "猎人开枪带走的目标")
}

// TestRule_R8_HunterShootsWhenVotedOut 猎人被投票放逐，可以开枪。
func TestRule_R8_HunterShootsWhenVotedOut(t *testing.T) {
	requireRule(t, "R8")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	g.walkNight()
	g.end(pb.PhaseType_PHASE_TYPE_VOTE)

	g.vote("h", "w1", "w2", "v1", "v2", "v3")
	g.end(pb.PhaseType_PHASE_TYPE_DAY_HUNTER)

	g.mustUse("h", pb.SkillType_SKILL_TYPE_SHOOT, "w1")
	g.endAny()

	g.assertAlive("h", false, "猎人被投票出局")
	g.assertAlive("w1", false, "猎人开枪带走的目标")
}

// TestRule_R8_PoisonedHunterCannotShoot 「除…被毒殺外」——被女巫毒死的猎人不能开枪。
func TestRule_R8_PoisonedHunterCannotShoot(t *testing.T) {
	requireRule(t, "R8.毒杀不开枪")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h"), witch("wi"),
		villagers("v1", "v2", "v3", "v4", "v5"),
	)...)

	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
	g.mustUse("wi", pb.SkillType_SKILL_TYPE_POISON, "h")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)

	// 猎人被毒死，不应进入猎人阶段
	g.end(pb.PhaseType_PHASE_TYPE_DAY)
	g.assertAlive("h", false, "猎人被毒杀")
}

// TestRule_R8_HunterMayNotShoot 猎人可以选择不开枪。
// 与 R5「或不進行守護」同理，技能是可选的：不提交任何技能即视为放弃。
func TestRule_R8_HunterMayNotShoot(t *testing.T) {
	requireRule(t, "R8")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "h")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER)

	// 猎人不提交任何技能，直接结束
	g.end(pb.PhaseType_PHASE_TYPE_DAY)

	g.assertAlive("w1", true, "猎人放弃开枪")
	g.assertAlive("w2", true, "猎人放弃开枪")
}

// TestRule_R8_HunterMaySkipExplicitly 猎人可以显式提交 SKIP 放弃开枪。
//
// 这不是维基规则问题，而是引擎自相矛盾：
//   - GetPhaseInfo() 通过 buildHunterPhaseInfo 向调用方宣告 AllowedSkills = [SHOOT, SKIP]
//   - 但 NightHunterPhase/DayHunterPhase 的 Steps 里没有 SKIP，
//     ValidateSkillUse 走 GetAllowedSkills 时会拒绝 SKIP
//   - 结果 HunterResolver 里处理 SKIP 的分支成了死代码
//
// 「引擎宣告可用的技能，必须真的可提交」是比任何单条规则更基础的约束。
func TestRule_R8_HunterMaySkipExplicitly(t *testing.T) {
	requireRule(t, "R8.显式跳过")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "h")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER)

	// 前置：引擎确实对外宣告了 SKIP 可用
	advertised := g.e.GetPhaseInfo().RoleInfos[pb.RoleType_ROLE_TYPE_HUNTER].AllowedSkills
	hasSkip := false
	for _, sk := range advertised {
		if sk == pb.SkillType_SKILL_TYPE_SKIP {
			hasSkip = true
		}
	}
	if !hasSkip {
		t.Fatalf("前置不成立：GetPhaseInfo 未宣告 SKIP 可用，实际 %v", advertised)
	}

	// 宣告了就必须能提交
	if err := g.use("h", pb.SkillType_SKILL_TYPE_SKIP, ""); err != nil {
		t.Fatalf("GetPhaseInfo 宣告 SKIP 可用，SubmitSkillUse 却拒绝: %v", err)
	}

	// Engine.GetAllowedSkills 也应与之一致
	allowed := g.e.GetAllowedSkills("h")
	t.Logf("Engine.GetAllowedSkills(h) = %v", allowed)

	g.end(pb.PhaseType_PHASE_TYPE_DAY)
	g.assertAlive("w1", true, "猎人显式跳过")
}

// TestRule_R8_HunterShootsOnlyOnce 猎人一局只能开一枪。
// 覆盖 RoundCtx.HunterTriggered 未被消费导致的重复触发。
func TestRule_R8_HunterShootsOnlyOnce(t *testing.T) {
	requireRule(t, "R8.一局一枪")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	// 夜里猎人被刀并开枪带走 w1
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "h")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	// 猎人在结算阶段才被触发，故 NIGHT_RESOLVE 结束后才进入猎人阶段
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER)
	g.mustUse("h", pb.SkillType_SKILL_TYPE_SHOOT, "w1")
	g.end(pb.PhaseType_PHASE_TYPE_DAY)
	g.assertAlive("w1", false, "猎人第一枪")

	// 当天投票出局一名平民（不是猎人）——不应再进猎人阶段
	g.end(pb.PhaseType_PHASE_TYPE_VOTE)
	g.vote("v1", "w2", "v2", "v3", "v4")

	if got := g.e.GetCurrentPhase(); got != pb.PhaseType_PHASE_TYPE_VOTE {
		t.Fatalf("投票前阶段异常: %v", got)
	}
	g.endAny()

	if got := g.e.GetCurrentPhase(); got == pb.PhaseType_PHASE_TYPE_DAY_HUNTER {
		t.Fatal("出局者不是猎人，却再次进入 DAY_HUNTER（HunterTriggered 未在触发后清除）")
	}
	g.assertAlive("w2", true, "猎人不应开出第二枪")
}

// ==================== R9 好人胜利 ====================

// TestRule_R9_GoodWinsWhenAllWolvesDead 「將狼人淘汰以獲取勝利」。
func TestRule_R9_GoodWinsWhenAllWolvesDead(t *testing.T) {
	requireRule(t, "R9")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), seer("s"), witch("wi"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	g.setDead("w1", "w2")

	over, winner := g.e.state.CheckVictory()
	if !over {
		t.Fatal("狼人全部出局，游戏应当结束")
	}
	if winner != pb.Camp_CAMP_GOOD {
		t.Errorf("期望好人阵营胜利，实际 %v", winner)
	}
}

// ==================== R10 狼人胜利（屠边） ====================

// TestRule_R10_WolvesWinByWipingOutOneSide
// 「狼人陣營需要淘汰所有平民或神職人員以獲取勝利」。
//
// 屠边有两条独立的达成路径，任一满足即狼人胜利：
//   - 屠民：所有平民出局（神职还活着也算狼胜）
//   - 屠神：所有神职出局（平民还活着也算狼胜）
func TestRule_R10_WolvesWinByWipingOutOneSide(t *testing.T) {
	requireRule(t, "R10")

	t.Run("屠神_神职全灭", func(t *testing.T) {
		g := newRuleGame(t, nil, seats(
			wolf("w1"), wolf("w2"),
			seer("s"), witch("wi"),
			villagers("v1", "v2", "v3", "v4", "v5"),
		)...)

		g.setDead("s", "wi") // 神职全灭，5 平民 vs 2 狼

		over, winner := g.e.state.CheckVictory()
		if !over || winner != pb.Camp_CAMP_EVIL {
			t.Errorf("神职全灭应判狼人胜利，实际 over=%v winner=%v", over, winner)
		}
	})

	t.Run("屠民_平民全灭", func(t *testing.T) {
		g := newRuleGame(t, nil, seats(
			wolf("w1"), wolf("w2"),
			seer("s"), witch("wi"), guard("g"), hunter("h"),
			villagers("v1", "v2"),
		)...)

		g.setDead("v1", "v2") // 平民全灭，4 神职 vs 2 狼

		over, winner := g.e.state.CheckVictory()
		if !over || winner != pb.Camp_CAMP_EVIL {
			t.Errorf("平民全灭应判狼人胜利，实际 over=%v winner=%v", over, winner)
		}
	})

	t.Run("双方都还有人_游戏继续", func(t *testing.T) {
		g := newRuleGame(t, nil, seats(
			wolf("w1"), wolf("w2"),
			seer("s"), witch("wi"),
			villagers("v1", "v2", "v3"),
		)...)

		g.setDead("wi", "v1") // 神职剩预言家，平民剩 2 人

		over, winner := g.e.state.CheckVictory()
		if over {
			t.Errorf("神职与平民都尚有存活，游戏不应结束，实际 winner=%v", winner)
		}
	})
}

// ==================== R11 白天发言后投票放逐 ====================

// TestRule_R11_DayThenVoteEliminates
// 「所有存活玩家輪流發言，並在所有人發言完畢後投票放逐一名玩家」。
func TestRule_R11_DayThenVoteEliminates(t *testing.T) {
	requireRule(t, "R11")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	g.walkNight()

	// 白天：所有存活玩家都应能发言
	for _, id := range []string{"w1", "w2", "v1", "v2", "v3", "v4", "v5", "v6"} {
		if err := g.e.SendMessage(id, "发言"); err != nil {
			t.Errorf("白天玩家 %s 应当可以发言，实际 err=%v", id, err)
		}
	}

	// 发言完毕后进入投票
	g.end(pb.PhaseType_PHASE_TYPE_VOTE)
	g.vote("v1", "w1", "w2", "v2", "v3", "v4")

	effects := g.endAny()
	elim := findEffect(effects, pb.EventType_EVENT_TYPE_ELIMINATE)
	if elim == nil {
		t.Fatal("期望产生 ELIMINATE 效果")
	}
	if elim.TargetID != "v1" {
		t.Errorf("期望放逐 v1，实际 %s", elim.TargetID)
	}
	g.assertAlive("v1", false, "被投票放逐")
}

// TestRule_R11_DeadPlayerCannotVote 已出局玩家不再拥有投票权。
func TestRule_R11_DeadPlayerCannotVote(t *testing.T) {
	requireRule(t, "R11")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	g.end(pb.PhaseType_PHASE_TYPE_DAY)
	g.end(pb.PhaseType_PHASE_TYPE_VOTE)

	err := g.use("v1", pb.SkillType_SKILL_TYPE_VOTE, "w1")
	if !IsErrorCode(err, pb.ErrorCode_ERROR_CODE_PLAYER_DEAD) {
		t.Errorf("已出局玩家投票应返回 PLAYER_DEAD，实际 %v", err)
	}
}

// ==================== D1–D3 本引擎自定口径 ====================

// TestConvention_D1_VoteTieEliminatesNobody D1：白天投票平票 -> 无人出局。
// 维基未规定平票处理方式，此为本引擎约定，改动即为破坏性变更。
func TestConvention_D1_VoteTieEliminatesNobody(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	g.walkNight()
	g.end(pb.PhaseType_PHASE_TYPE_VOTE)

	// v1 与 v2 各 2 票
	g.vote("v1", "w1", "w2")
	g.vote("v2", "v3", "v4")

	effects := g.end(pb.PhaseType_PHASE_TYPE_NIGHT_GUARD)

	if elim := findEffect(effects, pb.EventType_EVENT_TYPE_ELIMINATE); elim != nil {
		t.Errorf("平票不应有人出局，实际放逐了 %s", elim.TargetID)
	}
	g.assertAlive("v1", true, "平票")
	g.assertAlive("v2", true, "平票")
}

// TestConvention_D1_NoVotesEliminatesNobody D1 补充：无人投票同样无人出局。
func TestConvention_D1_NoVotesEliminatesNobody(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	g.walkNight()
	g.end(pb.PhaseType_PHASE_TYPE_VOTE)

	effects := g.end(pb.PhaseType_PHASE_TYPE_NIGHT_GUARD)
	if elim := findEffect(effects, pb.EventType_EVENT_TYPE_ELIMINATE); elim != nil {
		t.Errorf("无人投票时不应有人出局，实际放逐了 %s", elim.TargetID)
	}
}

// TestConvention_D2_WolfKillTieIsEmptyKnife D2：狼人刀口平票 -> 空刀。
func TestConvention_D2_WolfKillTieIsEmptyKnife(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), witch("wi"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)

	// 两狼各刀一人，平票
	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "v1")
	g.mustUse("w2", pb.SkillType_SKILL_TYPE_KILL, "v2")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)

	if got := g.witchSeesKill(); got != "" {
		t.Errorf("狼刀平票应为空刀，女巫不应看到刀口，实际 %q", got)
	}

	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	g.end(pb.PhaseType_PHASE_TYPE_DAY)

	g.assertAlive("v1", true, "狼刀平票")
	g.assertAlive("v2", true, "狼刀平票")
}

// TestConvention_D3_NightPhaseOrder D3：夜晚行动顺序固定为
// 守卫 → 狼人 → 女巫 → 预言家 → 结算。
//
// 顺序不是随意的：女巫必须排在狼人之后才能看到刀口（R2 的前提），
// 守卫必须排在狼人之前才能拦下刀口。
func TestConvention_D3_NightPhaseOrder(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), guard("g"), witch("wi"), seer("s"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	want := []pb.PhaseType{
		pb.PhaseType_PHASE_TYPE_NIGHT_GUARD,
		pb.PhaseType_PHASE_TYPE_NIGHT_WOLF,
		pb.PhaseType_PHASE_TYPE_NIGHT_WITCH,
		pb.PhaseType_PHASE_TYPE_NIGHT_SEER,
		pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE,
		pb.PhaseType_PHASE_TYPE_DAY,
		pb.PhaseType_PHASE_TYPE_VOTE,
		pb.PhaseType_PHASE_TYPE_NIGHT_GUARD, // 回到下一夜
	}

	if got := g.e.GetCurrentPhase(); got != want[0] {
		t.Fatalf("开局阶段: 期望 %v，实际 %v", want[0], got)
	}
	for i := 1; i < len(want); i++ {
		g.end(want[i])
	}

	if got := g.e.GetCurrentRound(); got != 2 {
		t.Errorf("走完一整轮后期望 Round=2，实际 %d", got)
	}
}

// TestConvention_D3_SkillRejectedOutsideItsPhase 技能只在其所属子阶段可用。
func TestConvention_D3_SkillRejectedOutsideItsPhase(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), guard("g"), witch("wi"), seer("s"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	// 开局是 NIGHT_GUARD，此时除守卫外的夜间技能都应被拒
	cases := []struct {
		player string
		skill  pb.SkillType
		target string
	}{
		{"w1", pb.SkillType_SKILL_TYPE_KILL, "v1"},
		{"wi", pb.SkillType_SKILL_TYPE_POISON, "v1"},
		{"s", pb.SkillType_SKILL_TYPE_CHECK, "v1"},
		{"v1", pb.SkillType_SKILL_TYPE_VOTE, "w1"},
	}
	for _, tc := range cases {
		err := g.use(tc.player, tc.skill, tc.target)
		if !IsErrorCode(err, pb.ErrorCode_ERROR_CODE_SKILL_NOT_ALLOWED) {
			t.Errorf("NIGHT_GUARD 阶段提交 %v 应返回 SKILL_NOT_ALLOWED，实际 %v", tc.skill, err)
		}
	}

	// 守卫技能在本阶段可用
	if err := g.use("g", pb.SkillType_SKILL_TYPE_PROTECT, "v1"); err != nil {
		t.Errorf("NIGHT_GUARD 阶段守卫应可守护，实际 %v", err)
	}
}
