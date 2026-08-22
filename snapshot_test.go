package werewolf

import (
	"encoding/json"
	"errors"
	"github.com/Zereker/werewolf/engine"
	"reflect"
	"testing"
)

// buildMidGameEngine 造一个局面复杂的引擎：第二夜、守卫守过人、
// 女巫用过解药、有玩家已出局、当前阶段还有未结算的技能。
func buildMidGameEngine(t *testing.T) *Engine {
	t.Helper()

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"),
		seer("s"), witch("wi"), guard("g"), hunter("h"),
		villagers("v1", "v2", "v3"),
	)...)

	// 第一夜：守卫守 v1，狼刀 v1，女巫解药救回
	g.mustUse("g", SkillProtect, "v2")
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)
	g.mustUse("wi", SkillAntidote, "v1")
	g.end(PhaseNightSeer)
	g.mustUse("s", SkillCheck, "w1")
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

	// 白天投票放逐 v3
	g.end(PhaseVote)
	g.vote("v3", "w1", "w2", "v1", "v2", "s")
	g.end(PhaseNightGuard)

	// 第二夜：守卫已提交技能，但本阶段尚未结算
	g.mustUse("g", SkillProtect, "s")

	return g.e
}

func TestSnapshot_RoundTripThroughJSON(t *testing.T) {
	eng := buildMidGameEngine(t)
	snap := eng.Snapshot()

	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	var decoded Snapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	restored, err := Restore(nil, DefaultRules(), &decoded)
	if err != nil {
		t.Fatalf("恢复失败: %v", err)
	}

	if !reflect.DeepEqual(snap, restored.Snapshot()) {
		t.Errorf("往返后快照不一致\n原始: %+v\n恢复: %+v", snap, restored.Snapshot())
	}
}

// TestSnapshot_RestoredEngineContinuesIdentically 恢复出来的引擎必须与
// 原引擎在后续推进中给出完全一致的结果——这是持久化真正要保证的东西。
func TestSnapshot_RestoredEngineContinuesIdentically(t *testing.T) {
	original := buildMidGameEngine(t)

	restored, err := Restore(nil, DefaultRules(), original.Snapshot())
	if err != nil {
		t.Fatalf("恢复失败: %v", err)
	}

	// 两个引擎并行走完第二夜，每一步都比对
	steps := []struct {
		player string
		skill  SkillType
		target string
	}{
		{"", engine.SkillUnspecified, ""}, // NIGHT_GUARD 结算（技能已在快照里）
		{"w1", SkillKill, "s"},
		{"wi", SkillPoison, "w2"},
		{"s", SkillCheck, "w2"},
		{"", engine.SkillUnspecified, ""}, // NIGHT_RESOLVE
	}

	for i, st := range steps {
		if st.player != "" {
			for _, e := range []*Engine{original, restored} {
				if err := e.SubmitSkillUse(&SkillUse{
					PlayerID: st.player, Skill: st.skill, TargetID: st.target,
				}); err != nil {
					t.Fatalf("第 %d 步提交失败: %v", i, err)
				}
			}
		}

		effOrig, err1 := original.EndPhase()
		effRest, err2 := restored.EndPhase()
		if err1 != nil || err2 != nil {
			t.Fatalf("第 %d 步 EndPhase: 原始 %v / 恢复 %v", i, err1, err2)
		}
		if len(effOrig) != len(effRest) {
			t.Fatalf("第 %d 步效果数量不一致: 原始 %d / 恢复 %d", i, len(effOrig), len(effRest))
		}
		for j := range effOrig {
			a, b := effOrig[j], effRest[j]
			if a.Type != b.Type || a.SourceID != b.SourceID ||
				a.TargetID != b.TargetID || a.Canceled != b.Canceled {
				t.Errorf("第 %d 步第 %d 个效果不一致:\n 原始 %+v\n 恢复 %+v", i, j, a, b)
			}
		}

		if !reflect.DeepEqual(original.Snapshot(), restored.Snapshot()) {
			t.Fatalf("第 %d 步之后两个引擎的状态已经分叉", i)
		}
	}
}

func TestSnapshot_PreservesDetailedState(t *testing.T) {
	eng := buildMidGameEngine(t)
	restored, err := Restore(nil, DefaultRules(), eng.Snapshot())
	if err != nil {
		t.Fatalf("恢复失败: %v", err)
	}

	t.Run("女巫药剂", func(t *testing.T) {
		wi, _ := restored.PlayerInfo("wi")
		if wi.Var(VarWitchAntidote) != "" {
			t.Error("第一夜用掉的解药不应恢复回来")
		}
		if wi.Var(VarWitchPoison) == "" {
			t.Error("未使用的毒药应当保留")
		}
	})

	t.Run("守卫上回合目标", func(t *testing.T) {
		// 第一夜守的是 v2，恢复后连守限制必须仍然生效
		p, ok := restored.PlayerInfo("g")
		if !ok {
			t.Fatal("守卫丢失")
		}
		if got := p.Var(PlayerVarLastProtectedTarget); got != "v2" {
			t.Errorf("守卫上回合目标: 期望 v2，实际 %q", got)
		}
	})

	t.Run("出局玩家", func(t *testing.T) {
		if mustAlive(t, restored, "v3") {
			t.Error("被放逐的 v3 不应复活")
		}
		if !mustAlive(t, restored, "v1") {
			t.Error("被救回的 v1 应当存活")
		}
	})

	t.Run("阶段与回合", func(t *testing.T) {
		if restored.Phase() != PhaseNightGuard {
			t.Errorf("阶段: 期望 NIGHT_GUARD，实际 %v", restored.Phase())
		}
		if restored.Round() != 2 {
			t.Errorf("回合: 期望 2，实际 %d", restored.Round())
		}
	})

	t.Run("未结算技能", func(t *testing.T) {
		snap := restored.Snapshot()
		if len(snap.PendingUses) != 1 {
			t.Fatalf("待结算技能数: 期望 1，实际 %d", len(snap.PendingUses))
		}
		u := snap.PendingUses[0]
		if u.PlayerID != "g" || u.Skill != SkillProtect || u.TargetID != "s" {
			t.Errorf("待结算技能内容不符: %+v", u)
		}
	})
}

// mustAlive 测试辅助
func mustAlive(t *testing.T, e *Engine, id string) bool {
	t.Helper()
	p, ok := e.PlayerInfo(id)
	if !ok {
		t.Fatalf("玩家不存在: %s", id)
	}
	return p.Alive
}

// TestSnapshot_IsDeepCopy 快照必须与引擎脱钩，后续推进不能改到已导出的快照。
func TestSnapshot_IsDeepCopy(t *testing.T) {
	eng := buildMidGameEngine(t)
	snap := eng.Snapshot()

	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}

	// 继续推进游戏
	for i := 0; i < 4; i++ {
		if _, err := eng.EndPhase(); err != nil {
			break
		}
	}

	after, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(after) {
		t.Error("引擎推进后，此前导出的快照被改动了")
	}
}

// TestSnapshot_Deterministic 同一局面导出的快照字节必须一致，
// 否则无法做幂等写入与快照比对（map 遍历顺序是随机的）。
func TestSnapshot_Deterministic(t *testing.T) {
	eng := buildMidGameEngine(t)

	first, err := json.Marshal(eng.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		next, err := json.Marshal(eng.Snapshot())
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(next) {
			t.Fatalf("第 %d 次导出与首次不一致\n首次: %s\n本次: %s", i, first, next)
		}
	}
}

func TestRestoreEngine_Rejects(t *testing.T) {
	valid := buildMidGameEngine(t).Snapshot()

	t.Run("nil 快照", func(t *testing.T) {
		if _, err := Restore(nil, DefaultRules(), nil); err != engine.ErrNilSnapshot {
			t.Errorf("期望 engine.ErrNilSnapshot，实际 %v", err)
		}
	})

	t.Run("版本不兼容", func(t *testing.T) {
		bad := *valid
		bad.Version = engine.SnapshotVersion + 1
		_, err := Restore(nil, DefaultRules(), &bad)
		if !engine.HasCode(err, engine.CodeInvalidSnapshot) {
			t.Errorf("期望 INVALID_SNAPSHOT，实际 %v", err)
		}
	})

	t.Run("玩家ID为空", func(t *testing.T) {
		bad := *valid
		bad.Players = append([]engine.PlayerSnapshot(nil), valid.Players...)
		bad.Players[0].ID = ""
		if _, err := Restore(nil, DefaultRules(), &bad); err != engine.ErrInvalidPlayerID {
			t.Errorf("期望 engine.ErrInvalidPlayerID，实际 %v", err)
		}
	})

	t.Run("玩家ID重复", func(t *testing.T) {
		bad := *valid
		bad.Players = append(append([]engine.PlayerSnapshot(nil), valid.Players...), valid.Players[0])
		_, err := Restore(nil, DefaultRules(), &bad)
		if !engine.HasCode(err, engine.CodeInvalidSnapshot) {
			t.Errorf("期望 INVALID_SNAPSHOT，实际 %v", err)
		}
	})

	t.Run("待结算技能引用不存在的玩家", func(t *testing.T) {
		bad := *valid
		bad.PendingUses = []engine.SkillUseSnapshot{{
			PlayerID: "查无此人",
			Skill:    SkillProtect,
			TargetID: "s",
		}}
		_, err := Restore(nil, DefaultRules(), &bad)
		if !engine.HasCode(err, engine.CodeInvalidSnapshot) {
			t.Errorf("期望 INVALID_SNAPSHOT，实际 %v", err)
		}
	})

	t.Run("阶段不在配置中", func(t *testing.T) {
		// 构造一个自身合法、但不含快照所在阶段的配置：
		// 只有白天与投票互相流转，没有任何夜晚阶段
		cfg := &GameConfig{
			StartPhase: PhaseDay,
			Phases: map[PhaseType]*PhaseConfig{
				PhaseDay: {
					Type:      PhaseDay,
					NextPhase: PhaseVote,
				},
				PhaseVote: {
					Type:      PhaseVote,
					NextPhase: PhaseDay,
					EndsRound: true,
				},
			},
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("前置：该配置本身应当合法，实际 %v", err)
		}

		// 快照停在 NIGHT_GUARD，而该配置里没有这个阶段
		_, err := Restore(cfg, DefaultRules(), valid)
		if !engine.HasCode(err, engine.CodeInvalidSnapshot) {
			t.Errorf("期望 INVALID_SNAPSHOT，实际 %v", err)
		}
	})
}

// TestSnapshot_EndedGame 已结束的对局同样可以快照与恢复。
func TestSnapshot_EndedGame(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), seer("s"), villagers("v1", "v2"),
	)...)
	g.setDead("w1")
	if _, err := g.e.EndPhase(); err != nil {
		t.Fatal(err)
	}
	if !g.e.IsGameOver() {
		t.Fatal("狼人全灭，游戏应当结束")
	}

	restored, err := Restore(nil, DefaultRules(), g.e.Snapshot())
	if err != nil {
		t.Fatalf("恢复已结束的对局失败: %v", err)
	}
	if !restored.IsGameOver() {
		t.Error("恢复后应当仍是已结束状态")
	}
	if _, err := restored.EndPhase(); err != engine.ErrGameEnded {
		t.Errorf("已结束的对局再推进应返回 engine.ErrGameEnded，实际 %v", err)
	}
}

// TestRestoreEngine_WithCustomResolver 恢复出来的引擎也要能带上自定义解析器。
//
// 解析器只能在构造时给出。漏掉的话恢复本身就会报错，
// 而不是给出一个「那个阶段的技能被静默丢弃」的引擎。
func TestRestoreEngine_WithCustomResolver(t *testing.T) {
	const customPhase = PhaseType("CUSTOM_RESOLVER_PHASE")

	cfg := DefaultGameConfig()
	cfg.Phases[customPhase] = &PhaseConfig{
		Type:      customPhase,
		NextPhase: PhaseDay,
		Steps: []PhaseStep{
			{Role: RoleVillager, Skill: SkillSkip},
		},
	}
	cfg.Phases[PhaseNightResolve].NextPhase = customPhase

	marker := &markerResolver{}

	eng, err := NewWith(cfg, DefaultRules(), engine.WithResolver(customPhase, marker))
	if err != nil {
		t.Fatalf("engine.NewEngine 失败: %v", err)
	}
	mustAdd(t, eng, "w1", RoleWerewolf)
	mustAdd(t, eng, "v1", RoleVillager)
	mustAdd(t, eng, "v2", RoleVillager)
	if err := eng.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}

	// 推到自定义阶段再存档
	for eng.Phase() != customPhase {
		if _, err := eng.EndPhase(); err != nil {
			t.Fatalf("推进失败: %v", err)
		}
	}
	snap := eng.Snapshot()

	// 忘了带解析器：必须直接报错，而不是给一个会静默丢技能的引擎
	if _, err := Restore(cfg, DefaultRules(), snap); err == nil {
		t.Fatal("缺少自定义阶段的解析器时，恢复应当报错")
	}

	restored, err := Restore(cfg, DefaultRules(), snap, engine.WithResolver(customPhase, marker))
	if err != nil {
		t.Fatalf("engine.RestoreEngine 失败: %v", err)
	}
	if err := restored.SubmitSkillUse(&SkillUse{
		PlayerID: "v1", Skill: SkillSkip,
	}); err != nil {
		t.Fatalf("提交技能失败: %v", err)
	}
	effects, err := restored.EndPhase()
	if err != nil {
		t.Fatalf("EndPhase 失败: %v", err)
	}
	if len(effects) == 0 {
		t.Error("自定义解析器没有被调用，技能被静默丢弃了")
	}
}

// markerResolver 一个只产出可辨认效果的解析器，用于验证它确实被调用了。
type markerResolver struct{}

func (r *markerResolver) Resolve(uses []*SkillUse, view GameView) []*Effect {
	out := make([]*Effect, 0, len(uses))
	for _, use := range uses {
		out = append(out, engine.NewEffect(EventSkip, use.PlayerID, ""))
	}
	return out
}

// TestRestoreEngine_RejectsInvalidPlayers 恢复不该放行 AddPlayer 会拒绝的东西。
//
// restorePlayer 刻意不走 AddPlayer（要原样还原存活状态与药剂），
// 但角色校验也跟着一起绕过去了；技能引用的目标同样没有校验，
// 指向一个不存在的人时会在结算时被静默丢弃。
func TestRestoreEngine_RejectsInvalidPlayers(t *testing.T) {
	base := func() *Snapshot {
		return &Snapshot{
			Version: engine.SnapshotVersion,
			Phase:   PhaseNightWolf,
			Round:   1,
			Players: []engine.PlayerSnapshot{
				{ID: "w1", Role: RoleWerewolf, Alive: true,
					Vars: map[string]string{VarCamp: string(CampEvil)}},
				{ID: "v1", Role: RoleVillager, Alive: true,
					Vars: map[string]string{VarCamp: string(CampGood)}},
			},
		}
	}

	// 正常快照能恢复
	if _, err := Restore(nil, DefaultRules(), base()); err != nil {
		t.Fatalf("前置条件：正常快照应能恢复，实际 %v", err)
	}

	snap := base()
	snap.Players[0].Role = RoleGod
	if _, err := Restore(nil, DefaultRules(), snap); !errors.Is(err, engine.ErrInvalidRole) {
		t.Errorf("上帝不是玩家身份，恢复应当被拒，实际 %v", err)
	}

	snap = base()
	snap.PendingUses = []engine.SkillUseSnapshot{{
		PlayerID: "w1",
		Skill:    SkillKill,
		TargetID: "查无此人",
		Phase:    PhaseNightWolf,
		Round:    1,
	}}
	if _, err := Restore(nil, DefaultRules(), snap); err == nil {
		t.Error("技能指向不存在的目标，恢复应当被拒")
	}
}

// TestSnapshot_CarriesEveryPlayerField 快照必须把玩家状态一个字段不落地带走。
//
// LastProtectedRound 曾经漏在 snapshotPlayers 之外整整一轮：存一次档，
// 守卫的连守限制当场失效——原引擎判连守取消、恢复后的引擎放行。
// 只比阶段与回合的往返测试挡不住这一类，因为两边照样能同步地走完一局，
// 只是规则判定不一样了。
func TestSnapshot_CarriesEveryPlayerField(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), guard("gd"), witch("wi"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	// 让每一类玩家状态都有非零值
	g.mustUse("gd", SkillProtect, "v1")
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v2")
	g.end(PhaseNightWitch)
	g.mustUse("wi", SkillAntidote, "v2")
	g.endAny()

	// 再加一项第三方的自定义状态
	g.e.Apply(engine.NewSetPlayerVarEffect("v3", "custom.flag", "yes"))

	snap := g.e.Snapshot()
	byID := make(map[string]engine.PlayerSnapshot, len(snap.Players))
	for _, p := range snap.Players {
		byID[p.ID] = p
	}

	// 逐个字段与引擎给出的真值比对
	for _, id := range g.e.AlivePlayerIDs() {
		want, _ := g.e.PlayerInfo(id)
		got := byID[id]
		switch {
		case got.Role != want.Role:
			t.Errorf("%s 的角色没带上: %+v", id, got)
		case got.Alive != want.Alive:
			t.Errorf("%s 的存活状态没带上", id)
		case !sameVars(got.Vars, want.Vars):
			t.Errorf("%s 的自定义状态没带上（女巫的药也在这里）: 快照 %v，实际 %v",
				id, got.Vars, want.Vars)
		case !sameVars(got.RoundVars, want.RoundVars):
			t.Errorf("%s 的回合标记没带上: 快照 %v，实际 %v",
				id, got.RoundVars, want.RoundVars)
		}
	}

	// 往返之后再导一次，必须逐字节一致
	restored, err := Restore(nil, DefaultRules(), snap)
	if err != nil {
		t.Fatalf("恢复失败: %v", err)
	}
	a, _ := json.Marshal(snap)
	b, _ := json.Marshal(restored.Snapshot())
	if string(a) != string(b) {
		t.Errorf("往返后不一致:\n  原  %s\n  副本 %s", a, b)
	}
}

// TestPlayerVar_SurvivesSnapshotAndReplay 第三方角色的状态随快照与效果流一起走。
//
// 这是「扩展的状态该住在哪」的答案：住在引擎里，写走 NewSetPlayerVarEffect。
// 住在 Resolver 自己的字段里的话，快照带不上、回放也重建不出——
// 而 Resolver 接口本来就要求它无状态。
func TestPlayerVar_SurvivesSnapshotAndReplay(t *testing.T) {
	const customPhase = PhaseType("CUSTOM_PHASE")

	cfg := DefaultGameConfig()
	cfg.Phases[customPhase] = &PhaseConfig{
		Type:      customPhase,
		NextPhase: PhaseDay,
		Steps:     []PhaseStep{{Role: RoleVillager, Skill: SkillSkip}},
	}
	cfg.Phases[PhaseNightResolve].NextPhase = customPhase

	opts := []EngineOption{engine.WithResolver(customPhase, varWritingResolver{})}
	g := newRuleGameWith(t, cfg, opts,
		seats(wolf("w1"), wolf("w2"), seer("s"), villagers("v1", "v2", "v3"))...)

	for g.e.Phase() != customPhase {
		g.endAny()
	}
	g.endAny()

	if got := g.info("v1").Vars["custom.mark"]; got != "set" {
		t.Fatalf("自定义状态应当写进了引擎，实际 %q", got)
	}

	// 快照
	restored, err := Restore(cfg, DefaultRules(), g.e.Snapshot(), opts...)
	if err != nil {
		t.Fatalf("恢复失败: %v", err)
	}
	if p, _ := restored.PlayerInfo("v1"); p.Vars["custom.mark"] != "set" {
		t.Errorf("恢复后自定义状态丢了: %v", p.Vars)
	}

	// 效果流回放
	replayed, err := Replay(cfg, DefaultRules(), g.e.EffectLog(), opts...)
	if err != nil {
		t.Fatalf("回放失败: %v", err)
	}
	if p, _ := replayed.PlayerInfo("v1"); p.Vars["custom.mark"] != "set" {
		t.Errorf("回放后自定义状态丢了: %v", p.Vars)
	}
}

// varWritingResolver 一个只写自定义状态的解析器。
type varWritingResolver struct{}

func (varWritingResolver) Resolve([]*SkillUse, GameView) []*Effect {
	return []*Effect{engine.NewSetPlayerVarEffect("v1", "custom.mark", "set")}
}
