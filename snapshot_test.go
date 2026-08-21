package werewolf

import (
	"encoding/json"
	"reflect"
	"testing"

	pb "github.com/Zereker/werewolf/proto"
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
	g.mustUse("g", pb.SkillType_SKILL_TYPE_PROTECT, "v2")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
	g.mustUse("wi", pb.SkillType_SKILL_TYPE_ANTIDOTE, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	g.mustUse("s", pb.SkillType_SKILL_TYPE_CHECK, "w1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	g.end(pb.PhaseType_PHASE_TYPE_DAY)

	// 白天投票放逐 v3
	g.end(pb.PhaseType_PHASE_TYPE_VOTE)
	g.vote("v3", "w1", "w2", "v1", "v2", "s")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_GUARD)

	// 第二夜：守卫已提交技能，但本阶段尚未结算
	g.mustUse("g", pb.SkillType_SKILL_TYPE_PROTECT, "s")

	return g.e
}

func TestSnapshot_RoundTripThroughJSON(t *testing.T) {
	engine := buildMidGameEngine(t)
	snap := engine.Snapshot()

	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	var decoded Snapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	restored, err := RestoreEngine(nil, &decoded)
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

	restored, err := RestoreEngine(nil, original.Snapshot())
	if err != nil {
		t.Fatalf("恢复失败: %v", err)
	}

	// 两个引擎并行走完第二夜，每一步都比对
	steps := []struct {
		player string
		skill  pb.SkillType
		target string
	}{
		{"", 0, ""}, // NIGHT_GUARD 结算（技能已在快照里）
		{"w1", pb.SkillType_SKILL_TYPE_KILL, "s"},
		{"wi", pb.SkillType_SKILL_TYPE_POISON, "w2"},
		{"s", pb.SkillType_SKILL_TYPE_CHECK, "w2"},
		{"", 0, ""}, // NIGHT_RESOLVE
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
	engine := buildMidGameEngine(t)
	restored, err := RestoreEngine(nil, engine.Snapshot())
	if err != nil {
		t.Fatalf("恢复失败: %v", err)
	}

	t.Run("女巫药剂", func(t *testing.T) {
		wi, _ := restored.GetPlayerInfo("wi")
		if wi.HasAntidote {
			t.Error("第一夜用掉的解药不应恢复回来")
		}
		if !wi.HasPoison {
			t.Error("未使用的毒药应当保留")
		}
	})

	t.Run("守卫上回合目标", func(t *testing.T) {
		// 第一夜守的是 v2，恢复后连守限制必须仍然生效
		p, ok := restored.state.getPlayer("g")
		if !ok {
			t.Fatal("守卫丢失")
		}
		if p.LastProtectedTarget != "v2" {
			t.Errorf("守卫上回合目标: 期望 v2，实际 %q", p.LastProtectedTarget)
		}
	})

	t.Run("出局玩家", func(t *testing.T) {
		if restored.state.mustAlive(t, "v3") {
			t.Error("被放逐的 v3 不应复活")
		}
		if !restored.state.mustAlive(t, "v1") {
			t.Error("被救回的 v1 应当存活")
		}
	})

	t.Run("阶段与回合", func(t *testing.T) {
		if restored.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
			t.Errorf("阶段: 期望 NIGHT_GUARD，实际 %v", restored.GetCurrentPhase())
		}
		if restored.GetCurrentRound() != 2 {
			t.Errorf("回合: 期望 2，实际 %d", restored.GetCurrentRound())
		}
	})

	t.Run("未结算技能", func(t *testing.T) {
		if len(restored.pendingUses) != 1 {
			t.Fatalf("待结算技能数: 期望 1，实际 %d", len(restored.pendingUses))
		}
		u := restored.pendingUses[0]
		if u.PlayerID != "g" || u.Skill != pb.SkillType_SKILL_TYPE_PROTECT || u.TargetID != "s" {
			t.Errorf("待结算技能内容不符: %+v", u)
		}
	})
}

// mustAlive 测试辅助
func (s *State) mustAlive(t *testing.T, id string) bool {
	t.Helper()
	p, ok := s.getPlayer(id)
	if !ok {
		t.Fatalf("玩家不存在: %s", id)
	}
	return p.Alive
}

// TestSnapshot_IsDeepCopy 快照必须与引擎脱钩，后续推进不能改到已导出的快照。
func TestSnapshot_IsDeepCopy(t *testing.T) {
	engine := buildMidGameEngine(t)
	snap := engine.Snapshot()

	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}

	// 继续推进游戏
	for i := 0; i < 4; i++ {
		if _, err := engine.EndPhase(); err != nil {
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
	engine := buildMidGameEngine(t)

	first, err := json.Marshal(engine.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		next, err := json.Marshal(engine.Snapshot())
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
		if _, err := RestoreEngine(nil, nil); err != ErrNilSnapshot {
			t.Errorf("期望 ErrNilSnapshot，实际 %v", err)
		}
	})

	t.Run("版本不兼容", func(t *testing.T) {
		bad := *valid
		bad.Version = SnapshotVersion + 1
		_, err := RestoreEngine(nil, &bad)
		if !IsErrorCode(err, pb.ErrorCode_ERROR_CODE_INVALID_SNAPSHOT) {
			t.Errorf("期望 INVALID_SNAPSHOT，实际 %v", err)
		}
	})

	t.Run("玩家ID为空", func(t *testing.T) {
		bad := *valid
		bad.Players = append([]PlayerSnapshot(nil), valid.Players...)
		bad.Players[0].ID = ""
		if _, err := RestoreEngine(nil, &bad); err != ErrInvalidPlayerID {
			t.Errorf("期望 ErrInvalidPlayerID，实际 %v", err)
		}
	})

	t.Run("玩家ID重复", func(t *testing.T) {
		bad := *valid
		bad.Players = append(append([]PlayerSnapshot(nil), valid.Players...), valid.Players[0])
		_, err := RestoreEngine(nil, &bad)
		if !IsErrorCode(err, pb.ErrorCode_ERROR_CODE_INVALID_SNAPSHOT) {
			t.Errorf("期望 INVALID_SNAPSHOT，实际 %v", err)
		}
	})

	t.Run("待结算技能引用不存在的玩家", func(t *testing.T) {
		bad := *valid
		bad.PendingUses = []SkillUseSnapshot{{
			PlayerID: "查无此人",
			Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
			TargetID: "s",
		}}
		_, err := RestoreEngine(nil, &bad)
		if !IsErrorCode(err, pb.ErrorCode_ERROR_CODE_INVALID_SNAPSHOT) {
			t.Errorf("期望 INVALID_SNAPSHOT，实际 %v", err)
		}
	})

	t.Run("阶段不在配置中", func(t *testing.T) {
		// 构造一个自身合法、但不含快照所在阶段的配置：
		// 只有白天与投票互相流转，没有任何夜晚阶段
		cfg := &GameConfig{
			VictoryMode: VictoryModeSideWipe,
			StartPhase:  pb.PhaseType_PHASE_TYPE_DAY,
			Phases: map[pb.PhaseType]*PhaseConfig{
				pb.PhaseType_PHASE_TYPE_DAY: {
					Type:      pb.PhaseType_PHASE_TYPE_DAY,
					NextPhase: pb.PhaseType_PHASE_TYPE_VOTE,
				},
				pb.PhaseType_PHASE_TYPE_VOTE: {
					Type:      pb.PhaseType_PHASE_TYPE_VOTE,
					NextPhase: pb.PhaseType_PHASE_TYPE_DAY,
				},
			},
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("前置：该配置本身应当合法，实际 %v", err)
		}

		// 快照停在 NIGHT_GUARD，而该配置里没有这个阶段
		_, err := RestoreEngine(cfg, valid)
		if !IsErrorCode(err, pb.ErrorCode_ERROR_CODE_INVALID_SNAPSHOT) {
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

	restored, err := RestoreEngine(nil, g.e.Snapshot())
	if err != nil {
		t.Fatalf("恢复已结束的对局失败: %v", err)
	}
	if !restored.IsGameOver() {
		t.Error("恢复后应当仍是已结束状态")
	}
	if _, err := restored.EndPhase(); err != ErrGameEnded {
		t.Errorf("已结束的对局再推进应返回 ErrGameEnded，实际 %v", err)
	}
}
