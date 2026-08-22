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
// 种子固定，因此失败可以复现：日志里会带上 seed 与 step。
// games 调大即可加大搜索强度（3000 局约 15 秒）。
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
			stats[playRandom(t, seed, rng)]++
		}()
	}
	if n := stats["400 步未结束"]; n > 0 {
		t.Errorf("有 %d 局在 400 步内没有结束", n)
	}
}

func playRandom(t *testing.T, seed int, rng *rand.Rand) string {
	e := MustNewEngine(nil)
	roles := []RoleType{
		RoleWerewolf, RoleWerewolf,
		RoleWerewolf, RoleSeer,
		RoleWitch, RoleGuard,
		RoleHunter, RoleVillager,
		RoleVillager, RoleVillager,
		RoleVillager, RoleVillager,
	}
	rng.Shuffle(len(roles), func(i, j int) { roles[i], roles[j] = roles[j], roles[i] })

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
	for step := 0; step < 400; step++ {
		if e.IsGameOver() {
			return "结束"
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
		clone, errR := RestoreEngine(nil, &round)
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
		replayed, err := ReplayEngine(nil, e.EffectLog())
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
	return "400 步未结束"
}
