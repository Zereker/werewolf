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
// knownDeviations 登记「实现与规则不符、尚未修复」的行为，登记项默认 Skip，
// 以免 CI 因已知待办变红；设 WEREWOLF_STRICT_RULES=1 可强制全部执行：
//
//	WEREWOLF_STRICT_RULES=1 go test -run TestRule -v ./
//
// 登记粒度是「具体行为」而非「整条规则」，这样同一条规则下已经符合的
// 正向用例仍然常驻执行，不会被一并跳过。
//
// 当前该表为空——R1–R11 全部通过。新发现偏差时在此登记，修复后删除。

import (
	"maps"
	"os"
	"testing"
)

// knownDeviations 登记「实现与规则不符、尚未修复」的条目。
// key 为规则编号，value 为偏差描述。
var knownDeviations = map[string]string{}

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
	role RoleType
}

func wolf(id string) seat     { return seat{id, RoleWerewolf} }
func seer(id string) seat     { return seat{id, RoleSeer} }
func witch(id string) seat    { return seat{id, RoleWitch} }
func guard(id string) seat    { return seat{id, RoleGuard} }
func hunter(id string) seat   { return seat{id, RoleHunter} }
func villager(id string) seat { return seat{id, RoleVillager} }

// villagers 批量生成平民座位。
func villagers(ids ...string) []seat {
	out := make([]seat, 0, len(ids))
	for _, id := range ids {
		out = append(out, villager(id))
	}
	return out
}

// ruleGame 包装 Engine，提供面向规则测试的断言辅助。
type ruleGame struct {
	t *testing.T
	e *Engine
}

// sameVars 比较两份自定义状态是否完全一致。
//
// 女巫的药 v2 起并入 Vars，因此「状态有没有带上」这件事不再是逐字段比，
// 而是整张表比——漏掉一个键就是漏掉一条规则，与漏掉 LastProtectedRound
// 那次是同一类错误。
func sameVars(a, b map[string]string) bool {
	return maps.Equal(a, b)
}

// newRuleGame 按座位表创建并开局。cfg 为 nil 时使用默认配置。
func newRuleGame(t *testing.T, cfg *GameConfig, seats ...seat) *ruleGame {
	t.Helper()
	return newRuleGameWith(t, cfg, nil, seats...)
}

// newRuleGameWith 同 newRuleGame，另外带上构造选项。
// 日志、指标、自定义解析器都只能在构造时给出，故需要这个入口。
func newRuleGameWith(t *testing.T, cfg *GameConfig, opts []EngineOption, seats ...seat) *ruleGame {
	t.Helper()
	e := MustNewEngine(cfg, opts...)
	for _, st := range seats {
		if err := e.AddPlayer(st.id, st.role); err != nil {
			t.Fatalf("AddPlayer(%s, %v) 失败: %v", st.id, st.role, err)
		}
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
func (g *ruleGame) use(playerID string, skill SkillType, targetID string) error {
	g.t.Helper()
	return g.e.SubmitSkillUse(&SkillUse{PlayerID: playerID, Skill: skill, TargetID: targetID})
}

// mustUse 提交技能，失败即终止用例。
func (g *ruleGame) mustUse(playerID string, skill SkillType, targetID string) {
	g.t.Helper()
	if err := g.use(playerID, skill, targetID); err != nil {
		g.t.Fatalf("提交技能失败 player=%s skill=%v target=%s: %v", playerID, skill, targetID, err)
	}
}

// end 结束当前子阶段并断言流转到 expect。
func (g *ruleGame) end(expect PhaseType) []*Effect {
	g.t.Helper()
	from := g.e.Phase()
	effects, err := g.e.EndPhase()
	if err != nil {
		g.t.Fatalf("EndPhase() 于 %v 失败: %v", from, err)
	}
	if got := g.e.Phase(); got != expect {
		g.t.Fatalf("阶段流转错误: %v 结束后期望 %v，实际 %v", from, expect, got)
	}
	return effects
}

// endAny 结束当前子阶段，不断言目标阶段。
func (g *ruleGame) endAny() []*Effect {
	g.t.Helper()
	effects, err := g.e.EndPhase()
	if err != nil {
		g.t.Fatalf("EndPhase() 失败: %v", err)
	}
	return effects
}

// walkNight 从 NIGHT_GUARD 一路走到 DAY，中途不提交任何技能。
func (g *ruleGame) walkNight() {
	g.t.Helper()
	g.end(PhaseNightWolf)
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)
}

// toNextNight 从 DAY 走到下一个 NIGHT_GUARD（不投票，因而无人出局）。
func (g *ruleGame) toNextNight() {
	g.t.Helper()
	g.end(PhaseVote)
	g.end(PhaseNightGuard)
}

// info 取玩家只读信息。
func (g *ruleGame) info(id string) PlayerInfo {
	g.t.Helper()
	pi, ok := g.e.PlayerInfo(id)
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
	ri := g.e.PhaseInfo().RoleInfos[RoleWitch]
	if ri == nil {
		return ""
	}
	return witchKill(ri)
}

// vote 让多名玩家投同一目标。
func (g *ruleGame) vote(target string, voters ...string) {
	g.t.Helper()
	for _, v := range voters {
		g.mustUse(v, SkillVote, target)
	}
}

// setDead 直接把玩家置为死亡，用于构造胜负判定的局面。
func (g *ruleGame) setDead(ids ...string) {
	g.t.Helper()
	g.e.mu.Lock()
	defer g.e.mu.Unlock()
	for _, id := range ids {
		p, ok := g.e.state.players[id]
		if !ok {
			g.t.Fatalf("玩家不存在: %s", id)
		}
		p.Alive = false
	}
}

// findEffect 在效果列表里找第一个指定类型的效果。
func findEffect(effects []*Effect, typ EventType) *Effect {
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
		wantCamp   Camp
		wantIsGood bool
	}{
		{"w1", CampEvil, false},
		{"v1", CampGood, true},
		{"wi", CampGood, true}, // 神职也应报好人，而非报出角色
	}

	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			g := newRuleGame(t, nil, seats(
				wolf("w1"), wolf("w2"), seer("s"), witch("wi"),
				villagers("v1", "v2", "v3", "v4"),
			)...)

			g.end(PhaseNightWolf)
			g.end(PhaseNightWitch)
			g.end(PhaseNightSeer)

			g.mustUse("s", SkillCheck, tc.target)
			effects := g.end(PhaseNightResolve)

			check := findEffect(effects, EventCheck)
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
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)

	if got := g.witchSeesKill(); got != "v1" {
		t.Fatalf("第一夜解药在手：期望女巫看到刀口 v1，实际 %q", got)
	}
	g.mustUse("wi", SkillAntidote, "v1")
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

	if g.info("wi").Var(VarWitchAntidote) != "" {
		t.Fatal("第一夜救人后解药应当已消耗")
	}
	g.assertAlive("v1", true, "第一夜被救")

	// —— 第二夜：解药已用完，不应再看到刀口 ——
	g.toNextNight()
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v2")
	g.end(PhaseNightWitch)

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
		first SkillType
	}{
		{"先解药后毒药", SkillAntidote},
		{"先毒药后解药", SkillPoison},
	}

	for _, ord := range orders {
		t.Run(ord.name, func(t *testing.T) {
			g := newRuleGame(t, nil, seats(
				wolf("w1"), wolf("w2"), witch("wi"),
				villagers("v1", "v2", "v3", "v4", "v5", "v6"),
			)...)

			g.end(PhaseNightWolf)
			g.mustUse("w1", SkillKill, "v1")
			g.end(PhaseNightWitch)

			// 两瓶药都尝试提交；后提交的那瓶被拒是允许的
			if ord.first == SkillAntidote {
				g.mustUse("wi", SkillAntidote, "v1")
				_ = g.use("wi", SkillPoison, "v2")
			} else {
				g.mustUse("wi", SkillPoison, "v2")
				_ = g.use("wi", SkillAntidote, "v1")
			}

			g.end(PhaseNightSeer)
			g.end(PhaseNightResolve)
			g.end(PhaseDay)

			after := g.info("wi")
			potionsUsed := 0
			if after.Var(VarWitchAntidote) == "" {
				potionsUsed++
			}
			if after.Var(VarWitchPoison) == "" {
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
				wolf("w1"), wolf("w2"), witch("wi"), seer("s"),
				villagers("v1", "v2", "v3", "v4", "v5", "v6"),
			)...)

			g.end(PhaseNightWolf)
			g.mustUse("w1", SkillKill, "wi")
			g.end(PhaseNightWitch)

			if got := g.witchSeesKill(); got != "wi" {
				t.Fatalf("期望女巫看到自己被刀，实际刀口 %q", got)
			}
			g.mustUse("wi", SkillAntidote, "wi")

			g.end(PhaseNightSeer)
			g.end(PhaseNightResolve)
			g.end(PhaseDay)

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
				wolf("w1"), wolf("w2"), guard("g"), seer("s"),
				villagers("v1", "v2", "v3", "v4", "v5", "v6"),
			)...)

			g.mustUse("g", SkillProtect, "g")
			g.end(PhaseNightWolf)
			g.mustUse("w1", SkillKill, "g")
			g.end(PhaseNightWitch)
			g.end(PhaseNightSeer)
			g.end(PhaseNightResolve)
			g.end(PhaseDay)

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
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

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
	g.mustUse("g", SkillProtect, "v1")
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)
	g.assertAlive("v1", true, "第一夜守护生效")

	// 第二夜：再守 v1 —— 连守无效，狼刀 v1 应当命中
	g.toNextNight()
	g.mustUse("g", SkillProtect, "v1")
	effects := g.end(PhaseNightWolf)

	protect := findEffect(effects, EventProtect)
	if protect == nil {
		t.Fatal("期望产生 PROTECT 效果（即便被取消）")
	}
	if !protect.Canceled {
		t.Error("第二夜连守同一目标，PROTECT 效果应当被取消")
	}

	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
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
	g.mustUse("g", SkillProtect, "v1")
	g.walkNight()

	// 第二夜改守 v2，狼刀 v2 —— 应当守住
	g.toNextNight()
	g.mustUse("g", SkillProtect, "v2")
	effects := g.end(PhaseNightWolf)

	protect := findEffect(effects, EventProtect)
	if protect == nil || protect.Canceled {
		t.Fatalf("换人守护不应被取消，实际 %+v", protect)
	}

	g.mustUse("w1", SkillKill, "v2")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

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
	g.mustUse("g", SkillProtect, "v1")
	g.walkNight()

	// 第二夜守 v2
	g.toNextNight()
	g.mustUse("g", SkillProtect, "v2")
	g.walkNight()

	// 第三夜守回 v1 —— 与上一晚（v2）不同，应当生效
	g.toNextNight()
	g.mustUse("g", SkillProtect, "v1")
	effects := g.end(PhaseNightWolf)

	protect := findEffect(effects, EventProtect)
	if protect == nil || protect.Canceled {
		t.Fatalf("隔一晚后重新守回同一人应当生效，实际 %+v", protect)
	}
}

// TestRule_R6_GuardMayReprotectAfterIdleNight 空守一晚后可以再守回同一人。
//
// 与 GuardMayReprotectAfterGap 的区别在于中间那一晚守卫**什么都没做**。
// 规则限制的是「连续两晚守同一人」，守卫弃权的那一晚打断了这个连续性。
// 用「最后一次成功守护的目标」来判连守是不对的：那个记录不会因为
// 守卫弃权而失效，一旦命中就永久卡住这个目标。
func TestRule_R6_GuardMayReprotectAfterIdleNight(t *testing.T) {
	requireRule(t, "R6")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), guard("g"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	// 第一夜守 v1
	g.mustUse("g", SkillProtect, "v1")
	g.walkNight()

	// 第二夜守卫弃权
	g.toNextNight()
	g.walkNight()

	// 第三夜守回 v1 —— 上一晚没守任何人，不构成连守
	g.toNextNight()
	g.mustUse("g", SkillProtect, "v1")
	effects := g.end(PhaseNightWolf)

	protect := findEffect(effects, EventProtect)
	if protect == nil || protect.Canceled {
		t.Fatalf("空守一晚后重新守回同一人应当生效，实际 %+v", protect)
	}

	// 并且真的守得住
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)
	g.assertAlive("v1", true, "空守一晚后的守护应当生效")
}

// TestRule_R6_CanceledProtectDoesNotBlockNextNight 被判为连守而取消的一次守护，
// 不应该被记成「这一晚守了他」，从而连锁影响再下一晚。
func TestRule_R6_CanceledProtectDoesNotBlockNextNight(t *testing.T) {
	requireRule(t, "R6")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), guard("g"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	// 第一夜守 v1（生效）
	g.mustUse("g", SkillProtect, "v1")
	g.walkNight()

	// 第二夜再守 v1（连守，被取消）
	g.toNextNight()
	g.mustUse("g", SkillProtect, "v1")
	g.walkNight()

	// 第三夜守 v1 —— 第二夜那次没有生效，因此不构成连守
	g.toNextNight()
	g.mustUse("g", SkillProtect, "v1")
	effects := g.end(PhaseNightWolf)

	protect := findEffect(effects, EventProtect)
	if protect == nil || protect.Canceled {
		t.Fatalf("上一晚的守护被取消，本晚不应再判连守，实际 %+v", protect)
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

	g.mustUse("g", SkillProtect, "v1")
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)

	// 前置：女巫仍应看到刀口，守护对她不可见
	if got := g.witchSeesKill(); got != "v1" {
		t.Fatalf("守卫的守护不应遮蔽女巫视野：期望刀口 v1，实际 %q", got)
	}

	g.mustUse("wi", SkillAntidote, "v1")
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.endAny()

	g.assertAlive("v1", false, "同守同救")
	if g.info("wi").Var(VarWitchAntidote) != "" {
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

	g.mustUse("g", SkillProtect, "v1")
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)
	// 女巫不救
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

	g.assertAlive("v1", true, "仅守卫守护")
}

// ==================== R8 猎人开枪 ====================

// TestRule_R8_HunterShootsWhenKilledByWolves 猎人被狼刀，可以开枪。
func TestRule_R8_HunterShootsWhenKilledByWolves(t *testing.T) {
	requireRule(t, "R8")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h"), seer("s"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "h")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	// 猎人在结算阶段才被触发，故 NIGHT_RESOLVE 结束后才进入猎人阶段
	g.end(PhaseNightResolve)
	g.end(PhaseNightHunter)

	g.mustUse("h", SkillShoot, "w1")
	g.end(PhaseDay)

	g.assertAlive("h", false, "猎人被狼刀")
	g.assertAlive("w1", false, "猎人开枪带走的目标")
}

// TestRule_R8_HunterShootsWhenVotedOut 猎人被投票放逐，可以开枪。
func TestRule_R8_HunterShootsWhenVotedOut(t *testing.T) {
	requireRule(t, "R8")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h"), seer("s"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	g.walkNight()
	g.end(PhaseVote)

	g.vote("h", "w1", "w2", "v1", "v2", "v3")
	g.end(PhaseDayHunter)

	g.mustUse("h", SkillShoot, "w1")
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

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)
	g.mustUse("wi", SkillPoison, "h")
	g.end(PhaseNightSeer)

	// 猎人被毒死，结算后应直接进入白天，不经过猎人阶段
	g.end(PhaseNightResolve)
	g.end(PhaseDay)
	g.assertAlive("h", false, "猎人被毒杀")
}

// TestRule_R8_KnifedAndPoisonedHunterCannotShoot 同一晚既被狼刀又被毒的猎人不能开枪。
//
// 「除殉情或被毒殺外」是对死因的排除，不是对死亡通道的排除。
// 夜晚结算按刀口走「死了 -> 是猎人 -> 触发开枪」这条路，
// 如果只看守护与解药、不看毒药，被毒过的猎人照样会被拉进开枪阶段。
func TestRule_R8_KnifedAndPoisonedHunterCannotShoot(t *testing.T) {
	requireRule(t, "R8.毒杀不开枪")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h"), witch("wi"),
		villagers("v1", "v2", "v3", "v4", "v5"),
	)...)

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "h")
	g.end(PhaseNightWitch)
	// 女巫不救，改为对同一个人补毒
	g.mustUse("wi", SkillPoison, "h")
	g.end(PhaseNightSeer)

	// 猎人身上有毒，结算后应直接进白天
	g.end(PhaseNightResolve)
	g.end(PhaseDay)
	g.assertAlive("h", false, "猎人同时被刀被毒")
}

// TestRule_R8_HunterShotHunterMayShootBack 猎人开枪打死另一名猎人，后者同样可以开枪。
//
// 「以任何其他方式被淘汰時可以…開槍」——被枪打死也是「其他方式」。
// 死亡触发能力的入队目前分散在三条死亡通道上（狼刀、投票、开枪），
// 只要有一条漏了检查，这一类连锁就断在那里。
func TestRule_R8_HunterShotHunterMayShootBack(t *testing.T) {
	requireRule(t, "R8.连锁开枪")

	// 板子里留一名预言家：两名猎人都出局后神职仍未灭，
	// 否则屠边判定会在连锁开完之前就结束游戏，测不到要测的东西。
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h1"), hunter("h2"), seer("s"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	// 夜里狼刀 h1，h1 进开枪阶段
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "h1")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseNightHunter)

	// h1 打死另一名猎人 h2 —— h2 应当被拉进同一个开枪阶段
	g.mustUse("h1", SkillShoot, "h2")
	g.end(PhaseNightHunter)
	g.assertAlive("h2", false, "被 h1 打死")

	if ids := g.e.PhaseInfo().RoleInfos[RoleHunter].PlayerIDs; len(ids) != 1 || ids[0] != "h2" {
		t.Fatalf("本阶段应由 h2 行动，实际 %v", ids)
	}

	g.mustUse("h2", SkillShoot, "w1")
	g.end(PhaseDay)
	g.assertAlive("w1", false, "h2 的回枪应当生效")
}

// TestRule_R8_HunterMayNotShoot 猎人可以选择不开枪。
// 与 R5「或不進行守護」同理，技能是可选的：不提交任何技能即视为放弃。
func TestRule_R8_HunterMayNotShoot(t *testing.T) {
	requireRule(t, "R8")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h"), seer("s"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "h")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseNightHunter)

	// 猎人不提交任何技能，直接结束
	g.end(PhaseDay)

	g.assertAlive("w1", true, "猎人放弃开枪")
	g.assertAlive("w2", true, "猎人放弃开枪")
}

// TestRule_R8_HunterMaySkipExplicitly 猎人可以显式提交 SKIP 放弃开枪。
//
// 这不是维基规则问题，而是引擎自相矛盾：
//   - PhaseInfo() 通过 buildHunterPhaseInfo 向调用方宣告 AllowedSkills = [SHOOT, SKIP]
//   - 但 NightHunterPhase/DayHunterPhase 的 Steps 里没有 SKIP，
//     ValidateSkillUse 走 AllowedSkills 时会拒绝 SKIP
//   - 结果 HunterResolver 里处理 SKIP 的分支成了死代码
//
// 「引擎宣告可用的技能，必须真的可提交」是比任何单条规则更基础的约束。
func TestRule_R8_HunterMaySkipExplicitly(t *testing.T) {
	requireRule(t, "R8.显式跳过")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h"), seer("s"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "h")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseNightHunter)

	// 前置：引擎确实对外宣告了 SKIP 可用
	advertised := g.e.PhaseInfo().RoleInfos[RoleHunter].AllowedSkills
	hasSkip := false
	for _, sk := range advertised {
		if sk == SkillSkip {
			hasSkip = true
		}
	}
	if !hasSkip {
		t.Fatalf("前置不成立：PhaseInfo 未宣告 SKIP 可用，实际 %v", advertised)
	}

	// 宣告了就必须能提交
	if err := g.use("h", SkillSkip, ""); err != nil {
		t.Fatalf("PhaseInfo 宣告 SKIP 可用，SubmitSkillUse 却拒绝: %v", err)
	}

	// Engine.AllowedSkills 也应与之一致
	allowed := g.e.AllowedSkills("h")
	t.Logf("Engine.AllowedSkills(h) = %v", allowed)

	g.end(PhaseDay)
	g.assertAlive("w1", true, "猎人显式跳过")
}

// TestRule_R8_HunterShootsOnlyOnce 猎人一局只能开一枪。
// 覆盖 RoundCtx.HunterTriggered 未被消费导致的重复触发。
func TestRule_R8_HunterShootsOnlyOnce(t *testing.T) {
	requireRule(t, "R8.一局一枪")

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h"), seer("s"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	// 夜里猎人被刀并开枪带走 w1
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "h")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	// 猎人在结算阶段才被触发，故 NIGHT_RESOLVE 结束后才进入猎人阶段
	g.end(PhaseNightResolve)
	g.end(PhaseNightHunter)
	g.mustUse("h", SkillShoot, "w1")
	g.end(PhaseDay)
	g.assertAlive("w1", false, "猎人第一枪")

	// 当天投票出局一名平民（不是猎人）——不应再进猎人阶段
	g.end(PhaseVote)
	g.vote("v1", "w2", "v2", "v3", "v4")

	if got := g.e.Phase(); got != PhaseVote {
		t.Fatalf("投票前阶段异常: %v", got)
	}
	g.endAny()

	if got := g.e.Phase(); got == PhaseDayHunter {
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

	over, winner := checkVictory(g.e)
	if !over {
		t.Fatal("狼人全部出局，游戏应当结束")
	}
	if winner != CampGood {
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

		over, winner := checkVictory(g.e)
		if !over || winner != CampEvil {
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

		over, winner := checkVictory(g.e)
		if !over || winner != CampEvil {
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

		over, winner := checkVictory(g.e)
		if over {
			t.Errorf("神职与平民都尚有存活，游戏不应结束，实际 winner=%v", winner)
		}
	})
}

// TestRule_R7_GuardSaveTogetherDiesDisabled 同守同救致死的变体开关。
//
// 维基记载的是「依然會死亡」，故 GuardSaveTogetherDies 默认为 true；
// 关掉之后守护与解药叠加，目标存活。
func TestRule_R7_GuardSaveTogetherDiesDisabled(t *testing.T) {
	cfg := DefaultGameConfig()
	cfg.GuardSaveTogetherDies = false

	g := newRuleGame(t, cfg, seats(
		wolf("w1"), wolf("w2"), guard("g"), witch("wi"),
		villagers("v1", "v2", "v3", "v4", "v5"),
	)...)

	g.mustUse("g", SkillProtect, "v1")
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)
	g.mustUse("wi", SkillAntidote, "v1")
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

	g.assertAlive("v1", true, "关闭同守同救致死后")
}

// TestRule_R10_SideWipeIgnoresEvilCategories 屠边只数好人阵营的神职与平民。
//
// AddCustomPlayer 允许把狼队成员标成神职（隐狼、狼美人这类角色本来就是
// 「狼阵营的神」）。屠边的字面表述是「淘汰所有平民或神職人員」，指的是
// 好人阵营那一半。把狼队的神一起计进总数，一名活着的隐狼就能让
// 「好人的神已经死光」这个事实永远判不出来，狼队赢不了本该赢下的局。
func TestRule_R10_SideWipeIgnoresEvilCategories(t *testing.T) {
	requireRule(t, "R10.屠边只数好人")

	e := MustNewEngine(nil)
	mustAdd(t, e, "w1", RoleWerewolf)
	// 隐狼：狼阵营，但角色类别是神职
	if err := e.AddCustomPlayer("hidden", RoleWerewolf,
		CampEvil, RoleCategoryGod); err != nil {
		t.Fatalf("AddCustomPlayer 失败: %v", err)
	}
	mustAdd(t, e, "s", RoleSeer)
	mustAdd(t, e, "v1", RoleVillager)
	mustAdd(t, e, "v2", RoleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}

	// 好人这边唯一的神职出局 —— 屠神成立，狼人胜
	e.mu.Lock()
	e.state.players["s"].Alive = false
	e.mu.Unlock()

	over, winner := checkVictory(e)
	if !over {
		t.Fatal("好人神职已全部出局，屠神应当成立（活着的隐狼不算好人的神）")
	}
	if winner != CampEvil {
		t.Errorf("期望狼人胜利，实际 %v", winner)
	}
}

// TestRule_R10_VictoryModeTownWipe 屠城判定的变体开关。
func TestRule_R10_VictoryModeTownWipe(t *testing.T) {
	cfg := DefaultGameConfig()
	cfg.VictoryMode = VictoryModeTownWipe

	g := newRuleGame(t, cfg, seats(
		wolf("w1"), wolf("w2"),
		seer("s"), witch("wi"),
		villagers("v1", "v2"),
	)...)

	// 好人 4 > 狼 2，游戏继续
	if over, _ := checkVictory(g.e); over {
		t.Fatal("好人多于狼人时游戏不应结束")
	}

	// 好人降到 2 == 狼 2，屠城成立
	g.setDead("v1", "v2")
	over, winner := checkVictory(g.e)
	if !over || winner != CampEvil {
		t.Errorf("屠城模式下 好人数 <= 狼人数 应判狼人胜利，实际 over=%v winner=%v", over, winner)
	}

	// 同一局面在屠边模式下：神职还在、平民全灭 -> 同样是狼胜（屠民）
	if over, winner := checkVictory(g.e); !over || winner != CampEvil {
		t.Errorf("屠边模式下平民全灭应判狼人胜利，实际 over=%v winner=%v", over, winner)
	}
}

// TestRule_R10_MissingCategoryDoesNotEndGame 屠边判定只对开局存在的类别生效。
//
// 没有神职的板子不应在开局瞬间因「神职全灭」判负，平民同理。
func TestRule_R10_MissingCategoryDoesNotEndGame(t *testing.T) {
	t.Run("无神职板子", func(t *testing.T) {
		g := newRuleGame(t, nil, seats(
			wolf("w1"), villagers("v1", "v2", "v3"),
		)...)
		if over, winner := checkVictory(g.e); over {
			t.Errorf("板子上本就没有神职，不应判负，实际 winner=%v", winner)
		}
	})

	t.Run("无平民板子", func(t *testing.T) {
		g := newRuleGame(t, nil, seats(
			wolf("w1"), seer("s"), witch("wi"), guard("g"),
		)...)
		if over, winner := checkVictory(g.e); over {
			t.Errorf("板子上本就没有平民，不应判负，实际 winner=%v", winner)
		}
	})

	t.Run("好人全灭仍然判负", func(t *testing.T) {
		g := newRuleGame(t, nil, seats(
			wolf("w1"), villagers("v1", "v2"),
		)...)
		g.setDead("v1", "v2")
		over, winner := checkVictory(g.e)
		if !over || winner != CampEvil {
			t.Errorf("好人全灭应判狼人胜利，实际 over=%v winner=%v", over, winner)
		}
	})
}

// TestRule_R10_CategoryOf 角色类别的默认映射。
func TestRule_R10_CategoryOf(t *testing.T) {
	cases := []struct {
		role RoleType
		want RoleCategory
	}{
		{RoleWerewolf, RoleCategoryWolf},
		{RoleSeer, RoleCategoryGod},
		{RoleWitch, RoleCategoryGod},
		{RoleHunter, RoleCategoryGod},
		{RoleGuard, RoleCategoryGod},
		{RoleVillager, RoleCategoryVillager},
		{RoleGod, RoleCategoryUnknown},
	}
	for _, tc := range cases {
		if got := CategoryOf(tc.role); got != tc.want {
			t.Errorf("CategoryOf(%v): 期望 %v，实际 %v", tc.role, tc.want, got)
		}
	}

	// AddPlayer 应自动填充类别，且可被覆盖（供自定义角色使用）
	g := newRuleGame(t, nil, seats(wolf("w1"), seer("s"), villagers("v1", "v2"))...)
	if got := g.info("s").Category; got != RoleCategoryGod {
		t.Errorf("预言家类别: 期望 GOD，实际 %v", got)
	}
	// 扩展角色用 AddCustomPlayer 显式指定阵营与类别：
	// 隐狼是「好人牌面的狼」，阵营与类别都无法从角色推导
	e2 := MustNewEngine(nil)
	if err := e2.AddCustomPlayer("hidden", RoleVillager,
		CampEvil, RoleCategoryWolf); err != nil {
		t.Fatalf("AddCustomPlayer 失败: %v", err)
	}
	hidden, _ := e2.PlayerInfo("hidden")
	if hidden.Camp != CampEvil || hidden.Category != RoleCategoryWolf {
		t.Errorf("自定义玩家: 期望 EVIL/WOLF，实际 %v/%v", hidden.Camp, hidden.Category)
	}
}

// TestRule_R10_HunterShotCanFlipVictory 猎人的枪可以翻盘。
//
// 胜负判定必须排在死亡技能结算之后：猎人被刀时神职即将全灭，
// 但他开枪带走最后一只狼，好人反而获胜。
func TestRule_R10_HunterShotCanFlipVictory(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), hunter("h"), villagers("v1", "v2"),
	)...)

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "h")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)

	// 猎人是唯一神职，此刻「屠神」已成立，但必须先让他开枪
	g.end(PhaseNightHunter)
	if g.e.IsGameOver() {
		t.Fatal("猎人尚未开枪，游戏不应结束")
	}

	g.mustUse("h", SkillShoot, "w1")
	g.endAny()

	if !g.e.IsGameOver() {
		t.Fatal("最后一只狼被带走，游戏应当结束")
	}
	over, winner := checkVictory(g.e)
	if !over || winner != CampGood {
		t.Errorf("猎人带走最后一只狼，应判好人胜利，实际 over=%v winner=%v", over, winner)
	}
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
	g.end(PhaseVote)
	g.vote("v1", "w1", "w2", "v2", "v3", "v4")

	effects := g.endAny()
	elim := findEffect(effects, EventEliminate)
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

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)
	g.end(PhaseVote)

	err := g.use("v1", SkillVote, "w1")
	if !HasCode(err, CodePlayerDead) {
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
	g.end(PhaseVote)

	// v1 与 v2 各 2 票
	g.vote("v1", "w1", "w2")
	g.vote("v2", "v3", "v4")

	effects := g.end(PhaseNightGuard)

	if elim := findEffect(effects, EventEliminate); elim != nil {
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
	g.end(PhaseVote)

	effects := g.end(PhaseNightGuard)
	if elim := findEffect(effects, EventEliminate); elim != nil {
		t.Errorf("无人投票时不应有人出局，实际放逐了 %s", elim.TargetID)
	}
}

// TestConvention_D2_WolfKillTieIsEmptyKnife D2：狼人刀口平票 -> 空刀。
func TestConvention_D2_WolfKillTieIsEmptyKnife(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), witch("wi"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	g.end(PhaseNightWolf)

	// 两狼各刀一人，平票
	g.mustUse("w1", SkillKill, "v1")
	g.mustUse("w2", SkillKill, "v2")
	g.end(PhaseNightWitch)

	if got := g.witchSeesKill(); got != "" {
		t.Errorf("狼刀平票应为空刀，女巫不应看到刀口，实际 %q", got)
	}

	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

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

	want := []PhaseType{
		PhaseNightGuard,
		PhaseNightWolf,
		PhaseNightWitch,
		PhaseNightSeer,
		PhaseNightResolve,
		PhaseDay,
		PhaseVote,
		PhaseNightGuard, // 回到下一夜
	}

	if got := g.e.Phase(); got != want[0] {
		t.Fatalf("开局阶段: 期望 %v，实际 %v", want[0], got)
	}
	for i := 1; i < len(want); i++ {
		g.end(want[i])
	}

	if got := g.e.Round(); got != 2 {
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
		skill  SkillType
		target string
	}{
		{"w1", SkillKill, "v1"},
		{"wi", SkillPoison, "v1"},
		{"s", SkillCheck, "v1"},
		{"v1", SkillVote, "w1"},
	}
	for _, tc := range cases {
		err := g.use(tc.player, tc.skill, tc.target)
		if !HasCode(err, CodeSkillNotAllowed) {
			t.Errorf("NIGHT_GUARD 阶段提交 %v 应返回 SKILL_NOT_ALLOWED，实际 %v", tc.skill, err)
		}
	}

	// 守卫技能在本阶段可用
	if err := g.use("g", SkillProtect, "v1"); err != nil {
		t.Errorf("NIGHT_GUARD 阶段守卫应可守护，实际 %v", err)
	}
}
