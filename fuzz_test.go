package werewolf

import (
	"encoding/json"
	"fmt"
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
	for _, k := range []string{"结束", "屠边", "屠城", "有守卫阶段", "无守卫阶段"} {
		t.Logf("  %-12s %d", k, stats[k])
	}
	if n := stats["400 步未结束"]; n > 0 {
		t.Errorf("有 %d 局在 400 步内没有结束", n)
	}
	// 随机化一旦退化（比如某个分支永远走不到），这个测试会安静地
	// 变成只跑默认配置——那正是它要修的毛病，所以显式挡一道
	for _, k := range []string{"屠边", "屠城", "有守卫阶段", "无守卫阶段"} {
		if stats[k] == 0 {
			t.Errorf("随机化没有覆盖到「%s」，搜索空间退化了", k)
		}
	}
}

// randomConfig 随机出一套合法配置。
//
// 只随机「使用者真的可能这么配」的维度，且必须自洽——Validate 过不了的
// 配置属于另一类测试（config_test.go 里逐条盯着），在这里只会掩盖真问题。
func randomConfig(rng *rand.Rand) *GameConfig {
	cfg := DefaultGameConfig()

	// 6 个规则开关，各自独立
	cfg.WitchCanSaveSelf = rng.Intn(2) == 0
	cfg.WitchCanUseBothPotions = rng.Intn(2) == 0
	cfg.GuardCanProtectSelf = rng.Intn(2) == 0
	cfg.GuardCanRepeat = rng.Intn(2) == 0
	cfg.SameGuardKillIsEmpty = rng.Intn(2) == 0
	cfg.GuardSaveTogetherDies = rng.Intn(2) == 0

	if rng.Intn(2) == 0 {
		cfg.VictoryMode = VictoryModeTownWipe
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
	}

	if err := cfg.Validate(); err != nil {
		panic("随机出的配置本身不合法: " + err.Error())
	}
	return cfg
}

// randomBoard 随机出一副合法的板子：至少一狼、至少一好人。
func randomBoard(rng *rand.Rand, withGuard bool) []RoleType {
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

	rng.Shuffle(len(roles), func(i, j int) { roles[i], roles[j] = roles[j], roles[i] })
	return roles
}

// playRandom 跑一局，返回若干个用于统计的标签。
func playRandom(t *testing.T, seed int, rng *rand.Rand) []string {
	cfg := randomConfig(rng)
	_, hasGuardPhase := cfg.Phases[PhaseNightGuard]

	tags := []string{"有守卫阶段", "屠边"}
	if !hasGuardPhase {
		tags[0] = "无守卫阶段"
	}
	if cfg.VictoryMode == VictoryModeTownWipe {
		tags[1] = "屠城"
	}

	e := MustNewEngine(cfg)
	roles := randomBoard(rng, hasGuardPhase)

	ids := make([]string, 0, len(roles))
	identity := map[string][2]int32{}
	for i, r := range roles {
		id := fmt.Sprintf("p%02d", i)
		if err := e.AddPlayer(id, r); err != nil {
			t.Fatalf("seed=%d AddPlayer: %v", seed, err)
		}
		ids = append(ids, id)
		identity[id] = [2]int32{int32(r), int32(CampOf(r))}
	}
	if err := e.Start(); err != nil {
		t.Fatalf("seed=%d Start: %v", seed, err)
	}

	dead := map[string]bool{}
	lastRound := e.Round()
	cycles := 1 // 已经进入起始阶段一次
	for step := 0; step < 400; step++ {
		if e.IsGameOver() {
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
			_ = e.SubmitSkillUse(&SkillUse{PlayerID: id, Skill: skill, TargetID: target})
		}

		// 不变量 A：PlayerView 与 AllowedSkills 一致
		for _, id := range ids {
			if a, b := len(e.AllowedSkills(id)), len(e.PlayerView(id).AllowedSkills); a != b {
				t.Fatalf("seed=%d step=%d %s: AllowedSkills=%d PlayerView=%d", seed, step, id, a, b)
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
					t.Fatalf("seed=%d step=%d PhaseInfo 的 %v 名单顺序不稳定: %s vs %s",
						seed, step, role, want[role], got)
				}
			}
		}

		// 不变量 B：快照往返后继续推进，结果必须一致
		snap := e.Snapshot()
		raw, _ := json.Marshal(snap)
		var round Snapshot
		_ = json.Unmarshal(raw, &round)
		clone, errR := RestoreEngine(cfg, &round)
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
		if e.Phase() != clone.Phase() || e.Round() != clone.Round() {
			t.Fatalf("seed=%d step=%d 快照分叉: 原=%v/%d 副本=%v/%d",
				seed, step, e.Phase(), e.Round(), clone.Phase(), clone.Round())
		}

		// 不变量 H：回合数单调不减，且绕回起始阶段就必须是新的一回合。
		//
		// 「回合边界」此前写死成守卫阶段，阶段环里没有它的时候回合数
		// 永远停在 1。光看这一条还不够——回合数不动本身不刺眼，
		// 真正的后果由下面的 I 抓。
		if e.Round() < lastRound {
			t.Fatalf("seed=%d step=%d 回合数倒退: %d -> %d", seed, step, lastRound, e.Round())
		}
		if e.Phase() == cfg.startPhase() {
			cycles++
		}
		if e.Round() < cycles {
			t.Fatalf("seed=%d step=%d 第 %d 次绕回起始阶段，回合数却只有 %d",
				seed, step, cycles, e.Round())
		}

		// 不变量 I：进入新的一回合时，回合上下文必须是干净的。
		//
		// 被守、被救、被毒、刀口都是「本回合有效」的记录。不清的话
		// 女巫用掉的那瓶解药会一夜又一夜地把同一个人救回来——
		// 一次性道具变成了永久道具，规则当场失效。
		if e.Round() > lastRound {
			if rc := e.RoundContext(); rc != nil {
				if rc.KillTarget != "" || len(rc.ProtectedPlayers) > 0 ||
					len(rc.SavedPlayers) > 0 || len(rc.PoisonedPlayers) > 0 {
					t.Fatalf("seed=%d step=%d 进入第 %d 回合，上一回合的记录还在: %+v",
						seed, step, e.Round(), rc)
				}
			}
		}
		lastRound = e.Round()

		// 不变量 C：死人不复活；身份不变
		for _, id := range ids {
			p, _ := e.PlayerInfo(id)
			if dead[id] && p.Alive {
				t.Fatalf("seed=%d step=%d %s 复活了", seed, step, id)
			}
			if !p.Alive {
				dead[id] = true
			}
			if got := ([2]int32{int32(p.Role), int32(p.Camp)}); got != identity[id] {
				t.Fatalf("seed=%d step=%d %s 身份变了", seed, step, id)
			}
		}

		// 不变量 E：任何玩家的视图里，不该出现他无权知道的身份
		for _, id := range ids {
			v := e.PlayerView(id)
			self, _ := e.PlayerInfo(id)
			for _, p := range v.Players {
				if p.Role == RoleUnspecified || p.ID == id {
					continue
				}
				// 只允许看到狼队友的身份
				other, _ := e.PlayerInfo(p.ID)
				if self.Camp != CampEvil || other.Camp != CampEvil {
					t.Fatalf("seed=%d step=%d %s(%v) 看到了 %s(%v) 的身份",
						seed, step, id, self.Camp, p.ID, other.Camp)
				}
			}
			// 刀口只有活着且解药在手的女巫能看到
			if v.KillTarget != "" {
				if !self.Alive || self.Role != RoleWitch || !self.HasAntidote {
					t.Fatalf("seed=%d step=%d %s 不该看到刀口 %q", seed, step, id, v.KillTarget)
				}
			}
			// 好人不该有狼队友
			if self.Camp != CampEvil && len(v.Teammates) > 0 {
				t.Fatalf("seed=%d step=%d 好人 %s 拿到了队友 %v", seed, step, id, v.Teammates)
			}
		}

		// 不变量 F：AudienceOf 给出的受众必须都在场上；私密/被否决的效果不得外扩
		for _, ef := range lastEffects {
			aud, known := e.AudienceOf(ef)
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
		replayed, err := ReplayEngine(cfg, e.EffectLog())
		if err != nil {
			t.Fatalf("seed=%d step=%d Replay: %v", seed, step, err)
		}
		if replayed.Phase() != e.Phase() || replayed.Round() != e.Round() {
			t.Fatalf("seed=%d step=%d 回放分叉: 原=%v/%d 回放=%v/%d",
				seed, step, e.Phase(), e.Round(), replayed.Phase(), replayed.Round())
		}
		for _, id := range ids {
			a, _ := e.PlayerInfo(id)
			b, _ := replayed.PlayerInfo(id)
			if a.Alive != b.Alive || a.HasAntidote != b.HasAntidote || a.HasPoison != b.HasPoison {
				t.Fatalf("seed=%d step=%d 回放的 %s 状态不同: %+v vs %+v", seed, step, id, a, b)
			}
		}
	}
	return append(tags, "400 步未结束")
}
