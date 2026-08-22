package werewolf

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Zereker/werewolf/engine"
	"math/rand"
	"testing"
)

// TestFuzz_Invariants 随机对局，检查一批全局不变量。
//
// 逐条写的用例只能覆盖想得到的局面。这个测试反过来做：让引擎跑成千上万局
// 随机对局，每一步都去核对那些「无论如何都必须成立」的性质。
// 之前几轮 review 查出来的问题——回放分叉、快照与原引擎不同步、
// 出局的女巫看到刀口、被否决的效果广播给全场——恰好都属于
// 「某个特定局面下才现形」，也恰好都能被这里的不变量抓住。
//
// # 随机的是配置，不只是打法
//
// 这个测试最初只随机「怎么打」，板子与规则配置写死成默认值。但已经查出的
// 三条会改变对局结果的问题里，有两条恰恰出在**自定义配置**上：回合边界
// 写死了守卫阶段（阶段环里没有它就永不重置回合上下文）、恢复出来的引擎
// 注册不了自定义解析器。默认配置那条路早就被踩实了，出事的一直是旁边那些。
//
// 所以板子、6 个规则开关、胜负判定方式、起始阶段与阶段环现在也一起随机。
// 不变量一条都不用改——它们本来就该在任何合法配置下成立。
//
// 种子固定，因此失败可以复现：日志里会带上 seed 与 step。
// games 调大即可加大搜索强度（3000 局约 20 秒）。
func TestFuzz_Invariants(t *testing.T) {
	const games = 200
	stats := map[string]int{}

	for seed := 0; seed < games; seed++ {
		rng := rand.New(rand.NewSource(int64(seed)))
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("seed=%d PANIC: %v", seed, r)
				}
			}()
			for _, k := range playRandom(t, seed, rng) {
				stats[k]++
			}
		}()
	}
	for _, k := range []string{"结束", "屠边", "屠城", "有守卫阶段", "无守卫阶段", "含自定义角色", "只用内置角色"} {
		t.Logf("  %-12s %d", k, stats[k])
	}
	if n := stats["400 步未结束"]; n > 0 {
		t.Errorf("有 %d 局在 400 步内没有结束", n)
	}
	// 随机化一旦退化（比如某个分支永远走不到），这个测试会安静地
	// 变成只跑默认配置——那正是它要修的毛病，所以显式挡一道
	for _, k := range []string{"屠边", "屠城", "有守卫阶段", "无守卫阶段", "含自定义角色", "只用内置角色"} {
		if stats[k] == 0 {
			t.Errorf("随机化没有覆盖到「%s」，搜索空间退化了", k)
		}
	}
}

// randomConfig 随机出一套合法配置。
//
// 只随机「使用者真的可能这么配」的维度，且必须自洽——Validate 过不了的
// 配置属于另一类测试（config_test.go 里逐条盯着），在这里只会掩盖真问题。
func randomConfig(rng *rand.Rand) (*GameConfig, Rules) {
	cfg := DefaultGameConfig()
	rules := DefaultRules()

	// 6 个规则开关，各自独立
	rules.WitchCanSaveSelf = rng.Intn(2) == 0
	rules.WitchCanUseBothPotions = rng.Intn(2) == 0
	rules.GuardCanProtectSelf = rng.Intn(2) == 0
	rules.GuardCanRepeat = rng.Intn(2) == 0
	rules.SameGuardKillIsEmpty = rng.Intn(2) == 0
	rules.GuardSaveTogetherDies = rng.Intn(2) == 0

	if rng.Intn(2) == 0 {
		rules.VictoryMode = VictoryModeTownWipe
	}

	// 阶段环：三成的局面把守卫阶段整个摘掉。
	//
	// 没有守卫的板子本来就该这么配，而回合边界此前写死成 NIGHT_GUARD，
	// 摘掉它之后回合数永远停在 1、回合上下文永不重置——女巫用掉的那瓶
	// 解药会一夜又一夜地把同一个人救回来。这条路必须有人走。
	if rng.Intn(10) < 3 {
		cfg.StartPhase = PhaseNightWolf
		cfg.Phases[PhaseVote].NextPhase = PhaseNightWolf
		cfg.Phases[PhaseDayHunter].NextPhase = PhaseNightWolf
		delete(cfg.Phases, PhaseNightGuard)
		// 「每夜从干净的局面开始」跟着搬到新的第一个夜间阶段——
		// 这个声明是板子的一部分，改板子就要跟着改，和 NextPhase 一样。
		cfg.Phases[PhaseNightWolf].ClearsRoundVars = true
	}

	if err := cfg.Validate(); err != nil {
		panic("随机出的配置本身不合法: " + err.Error())
	}
	if err := rules.Validate(); err != nil {
		panic("随机出的规则本身不合法: " + err.Error())
	}
	return cfg, rules
}

// randomBoard 随机出一副能真的开局的板子。
//
// 「合法」不只是「至少一狼、至少一好人」：屠城模式下狼人不比好人少的
// 板子第一次结算就是狼人胜，Start 会直接拒掉（见
// TestEngine_Start_RejectsInvalidBoard）。补平人数是刻意的——不补的话
// 5000 局里有 8% 根本没开局，而统计里看不出来，「跑了 5000 局」就成了
// 一句不准确的话。
//
// extraEvil 是板子之外还会加进来的狼（狼王），一并计进人数。
func randomBoard(rng *rand.Rand, withGuard bool, extraEvil int) []RoleType {
	gods := []RoleType{RoleSeer, RoleWitch, RoleHunter}
	if withGuard {
		gods = append(gods, RoleGuard)
	}

	roles := make([]RoleType, 0, 12)
	for i := 0; i < 1+rng.Intn(3); i++ { // 1~3 狼
		roles = append(roles, RoleWerewolf)
	}
	for _, g := range gods { // 每个神职各有一半概率上场
		if rng.Intn(2) == 0 {
			roles = append(roles, g)
		}
	}
	for i := 0; i < 1+rng.Intn(5); i++ { // 1~5 民，保证好人不为空
		roles = append(roles, RoleVillager)
	}

	// 补平：好人必须严格多于狼人，否则屠城模式下开局即判负
	evil := extraEvil
	for _, r := range roles {
		if r == RoleWerewolf {
			evil++
		}
	}
	for len(roles)+extraEvil-evil <= evil {
		roles = append(roles, RoleVillager)
	}

	rng.Shuffle(len(roles), func(i, j int) { roles[i], roles[j] = roles[j], roles[i] })
	return roles
}

// withWolfKing 往配置里塞一个第三方角色：狼王，出局时可以带走一个人。
//
// 复用 extension_test.go 里那套狼王——随机对局跑的就是扩展契约本身的
// 那份可执行说明，而不是另造一个只在这里成立的假扩展。
//
// 随机对局此前只走内置配置的组合，扩展那扇门一次都没进过——而三条 P0
// 里那条「恢复出来的引擎注册不了自定义解析器」恰恰在门后，覆盖它的
// 一直只有 extension_test.go 里手写的一条路径。
//
// 返回配置需要的构造选项：解析器只能在构造时给出，恢复与回放同理。
func withWolfKing(cfg *GameConfig, rules Rules) []EngineOption {
	cfg.Phases[phaseWolfKing] = &PhaseConfig{
		Type:      phaseWolfKing,
		Steps:     []PhaseStep{{Role: roleWolfKing, Skill: skillWolfClaw}},
		NextPhase: cfg.StartPhase,
	}
	return []EngineOption{
		engine.WithRoleSetup(roleWolfKing, engine.RoleSetupFunc(wolfKingSetup)),
		engine.WithResolver(phaseWolfKing, &wolfKingResolver{}),
		engine.WithResolver(PhaseVote, &voteWithWolfKing{inner: NewVoteResolver()}),
		engine.WithResolver(PhaseNightResolve, &nightResolveWithWolfKing{inner: NewNightResolveResolver(rules)}),
	}
}

// nightResolveWithWolfKing 夜里被刀的狼王同样可以用爪子。
//
// 与 voteWithWolfKing 同构，只是接的是另一条死亡通道——第三方要覆盖
// 所有死法，就得每条通道各包一层。
type nightResolveWithWolfKing struct{ inner engine.Resolver }

func (r *nightResolveWithWolfKing) Resolve(uses []*SkillUse, view GameView) []*Effect {
	effects := r.inner.Resolve(uses, view)
	for _, ef := range effects[:len(effects):len(effects)] {
		if ef.Canceled || ef.TargetID == "" {
			continue
		}
		if ef.Type != EventKill && ef.Type != EventPoison {
			continue
		}
		if p, ok := view.Player(ef.TargetID); ok && p.Role == roleWolfKing {
			effects = append(effects, engine.NewDetourEffect(ef.TargetID, phaseWolfKing))
		}
	}
	return effects
}

func playRandom(t *testing.T, seed int, rng *rand.Rand) []string {
	cfg, rules := randomConfig(rng)
	_, hasGuardPhase := cfg.Phases[PhaseNightGuard]

	tags := []string{"有守卫阶段", "屠边", "只用内置角色"}
	if !hasGuardPhase {
		tags[0] = "无守卫阶段"
	}
	if rules.VictoryMode == VictoryModeTownWipe {
		tags[1] = "屠城"
	}

	// 三成的局面带一个第三方角色进来
	var opts []EngineOption
	withCustom := rng.Intn(10) < 3
	if withCustom {
		tags[2] = "含自定义角色"
		opts = withWolfKing(cfg, rules)
	}

	e, errNew := NewWith(cfg, rules, opts...)
	if errNew != nil {
		t.Fatalf("seed=%d NewWith: %v", seed, errNew)
	}
	extraEvil := 0
	if withCustom {
		extraEvil = 1 // 狼王也是狼
	}
	roles := randomBoard(rng, hasGuardPhase, extraEvil)
	if withCustom {
		roles = append(roles, roleWolfKing)
		rng.Shuffle(len(roles), func(i, j int) { roles[i], roles[j] = roles[j], roles[i] })
	}

	ids := make([]string, 0, len(roles))
	type who struct {
		role RoleType
		camp Camp
	}
	identity := map[string]who{}
	for i, r := range roles {
		id := fmt.Sprintf("p%02d", i)
		// 阵营与类别写在角色自己的 setup 里，入座不需要再给一遍——
		// 狼王的那一份见 wolfKingSetup
		if err := e.AddPlayer(id, r); err != nil {
			t.Fatalf("seed=%d AddPlayer: %v", seed, err)
		}
		ids = append(ids, id)
		p, _ := e.PlayerInfo(id)
		identity[id] = who{role: r, camp: campOf(p)}
	}
	if err := e.Start(); err != nil {
		// 开局就已分出胜负的板子（屠城模式下狼人不比好人少）会被 Start
		// 拒掉。这不是缺陷，是那条校验在起作用——但要记一笔，否则
		// 「跑了 5000 局」里有多少局根本没开局就说不清了。
		if !errors.Is(err, engine.ErrInvalidBoard) {
			t.Fatalf("seed=%d Start: %v", seed, err)
		}
		return append(tags, "开局即判负，未成局")
	}

	dead := map[string]bool{}
	lastRound := e.Status().Round
	lastPhase := e.Status().Phase
	for step := 0; step < 400; step++ {
		if e.Status().Over {
			return append(tags, "结束")
		}

		// 每个存活玩家随机提交一个当前允许的技能
		for _, id := range ids {
			skills := e.AllowedSkills(id)
			if len(skills) == 0 || rng.Intn(4) == 0 {
				continue
			}
			skill := skills[rng.Intn(len(skills))]
			target := ids[rng.Intn(len(ids))]
			_ = e.SubmitSkillUse(&SkillUse{PlayerID: id, Skill: skill, Targets: []string{target}})
		}

		// 不变量 A：PlayerView 与 AllowedSkills 一致
		for _, id := range ids {
			if a, b := len(e.AllowedSkills(id)), len(e.PlayerView(id).AllowedSkills); a != b {
				t.Fatalf("seed=%d step=%d %s: AllowedSkills=%d engine.PlayerView=%d", seed, step, id, a, b)
			}
		}

		// 不变量 G：同一个局面反复查询，名单顺序必须稳定
		want := map[RoleType]string{}
		for role, ri := range e.PhaseInfo().RoleInfos {
			want[role] = fmt.Sprint(ri.PlayerIDs)
		}
		for i := 0; i < 3; i++ {
			for role, ri := range e.PhaseInfo().RoleInfos {
				if got := fmt.Sprint(ri.PlayerIDs); got != want[role] {
					t.Fatalf("seed=%d step=%d engine.PhaseInfo 的 %v 名单顺序不稳定: %s vs %s",
						seed, step, role, want[role], got)
				}
			}
		}

		// 不变量 B：快照往返后继续推进，结果必须一致
		snap := e.Snapshot()
		raw, _ := json.Marshal(snap)
		var round Snapshot
		_ = json.Unmarshal(raw, &round)
		clone, errR := Restore(cfg, rules, &round, opts...)
		if errR != nil {
			t.Fatalf("seed=%d step=%d Restore: %v", seed, step, errR)
		}

		lastEffects, err := e.EndPhase()
		if err != nil {
			t.Fatalf("seed=%d step=%d EndPhase: %v", seed, step, err)
		}
		if _, err := clone.EndPhase(); err != nil {
			t.Fatalf("seed=%d step=%d clone EndPhase: %v", seed, step, err)
		}
		if e.Status().Phase != clone.Status().Phase || e.Status().Round != clone.Status().Round {
			t.Fatalf("seed=%d step=%d 快照分叉: 原=%v/%d 副本=%v/%d",
				seed, step, e.Status().Phase, e.Status().Round, clone.Status().Phase, clone.Status().Round)
		}

		// 光比阶段与回合不够：快照漏掉一个字段，两边照样能同步地
		// 走完一整局，只是规则判定不一样了——LastProtectedRound 就是这么
		// 漏了一整轮的（存一次档，连守限制当场失效）。
		// 逐字节比对两边导出的快照，才真的能挡住「漏字段」这一类。
		a, _ := json.Marshal(e.Snapshot())
		b, _ := json.Marshal(clone.Snapshot())
		if string(a) != string(b) {
			t.Fatalf("seed=%d step=%d 快照往返后状态不一致:\n  原  %s\n  副本 %s",
				seed, step, a, b)
		}

		// 不变量 H：回合数单调不减，且只在声明了 EndsRound 的阶段之后前进。
		//
		// 这一条此前写的是「绕回起始阶段就必须是新的一回合」——那编码的是
		// 内核自己猜回合边界的旧设计。现在边界由板子声明（PhaseConfig.
		// EndsRound），于是能断言更强的东西：**回合数不该在别的地方偷偷跳**。
		//
		// 光看回合数还不够——它不动本身不刺眼，真正的后果由下面的 I 抓。
		if e.Status().Round < lastRound {
			t.Fatalf("seed=%d step=%d 回合数倒退: %d -> %d", seed, step, lastRound, e.Status().Round)
		}
		if e.Status().Round > lastRound {
			if pc := cfg.Phases[lastPhase]; pc == nil || !pc.EndsRound {
				t.Fatalf("seed=%d step=%d 回合数从 %d 跳到 %d，但刚结算完的 %v 没有声明 EndsRound",
					seed, step, lastRound, e.Status().Round, lastPhase)
			}
		}

		// 不变量 I：进入新的一回合时，回合上下文必须是干净的。
		//
		// 被守、被救、被毒、刀口都是「本回合有效」的记录。不清的话
		// 女巫用掉的那瓶解药会一夜又一夜地把同一个人救回来——
		// 一次性道具变成了永久道具，规则当场失效。
		if e.Status().Round > lastRound {
			if rc := e.RoundContext(); rc != nil && len(rc.Vars) > 0 {
				t.Fatalf("seed=%d step=%d 进入第 %d 回合，上一回合的记录还在: %v",
					seed, step, e.Status().Round, rc.Vars)
			}
			for _, id := range ids {
				if p, ok := e.PlayerInfo(id); ok && len(p.RoundVars) > 0 {
					t.Fatalf("seed=%d step=%d 进入第 %d 回合，%s 身上的标记还在: %v",
						seed, step, e.Status().Round, id, p.RoundVars)
				}
			}
		}
		lastRound, lastPhase = e.Status().Round, e.Status().Phase

		// 不变量 C：死人不复活；身份不变
		for _, id := range ids {
			p, _ := e.PlayerInfo(id)
			if dead[id] && p.Alive {
				t.Fatalf("seed=%d step=%d %s 复活了", seed, step, id)
			}
			if !p.Alive {
				dead[id] = true
			}
			if got := (who{role: p.Role, camp: campOf(p)}); got != identity[id] {
				t.Fatalf("seed=%d step=%d %s 身份变了", seed, step, id)
			}
		}

		// 不变量 E：任何玩家的视图里，不该出现他无权知道的身份
		for _, id := range ids {
			v := e.PlayerView(id)
			self, _ := e.PlayerInfo(id)
			for _, p := range v.Players {
				if p.Role == engine.RoleUnspecified || p.ID == id {
					continue
				}
				// 只允许看到狼队友的身份
				other, _ := e.PlayerInfo(p.ID)
				if campOf(self) != CampEvil || campOf(other) != CampEvil {
					t.Fatalf("seed=%d step=%d %s(%v) 看到了 %s(%v) 的身份",
						seed, step, id, campOf(self), p.ID, campOf(other))
				}
			}
			// 刀口只有活着且解药在手的女巫能看到
			if v.RoleInfo[RoleInfoKillTarget] != "" {
				if !self.Alive || self.Role != RoleWitch || self.Vars[VarWitchAntidote] == "" {
					t.Fatalf("seed=%d step=%d %s 不该看到刀口 %q", seed, step, id, v.RoleInfo[RoleInfoKillTarget])
				}
			}
			// 好人不该有狼队友
			if campOf(self) != CampEvil && len(v.Teammates) > 0 {
				t.Fatalf("seed=%d step=%d 好人 %s 拿到了队友 %v", seed, step, id, v.Teammates)
			}
		}

		// 不变量 F：AudienceOf 给出的受众必须都在场上；私密/被否决的效果不得外扩
		for _, ef := range lastEffects {
			aud, known := e.AudienceOf(ef.ToEvent())
			if !known {
				t.Fatalf("seed=%d step=%d 引擎不认得自己产出的效果 %v", seed, step, ef.Type)
			}
			for _, rid := range aud {
				if _, ok := e.PlayerInfo(rid); !ok {
					t.Fatalf("seed=%d step=%d 受众里有不在场上的 %q", seed, step, rid)
				}
			}
			if ef.Canceled && len(aud) > 1 {
				t.Fatalf("seed=%d step=%d 被否决的效果发给了 %v", seed, step, aud)
			}
			switch ef.Type {
			case EventCheck, EventProtect,
				EventSave:
				if len(aud) > 1 {
					t.Fatalf("seed=%d step=%d 私密效果 %v 发给了 %v", seed, step, ef.Type, aud)
				}
			}
		}

		// 不变量 D：效果流回放必须与原引擎同步
		replayed, err := Replay(cfg, rules, e.EffectLog(), opts...)
		if err != nil {
			t.Fatalf("seed=%d step=%d Replay: %v", seed, step, err)
		}
		if replayed.Status().Phase != e.Status().Phase || replayed.Status().Round != e.Status().Round {
			t.Fatalf("seed=%d step=%d 回放分叉: 原=%v/%d 回放=%v/%d",
				seed, step, e.Status().Phase, e.Status().Round, replayed.Status().Phase, replayed.Status().Round)
		}
		for _, id := range ids {
			a, _ := e.PlayerInfo(id)
			b, _ := replayed.PlayerInfo(id)
			if a.Alive != b.Alive || !sameVars(a.Vars, b.Vars) {
				t.Fatalf("seed=%d step=%d 回放的 %s 状态不同: %+v vs %+v", seed, step, id, a, b)
			}
		}
	}
	return append(tags, "400 步未结束")
}
