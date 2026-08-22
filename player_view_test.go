package werewolf

import (
	"bytes"
	"encoding/json"
	"testing"
)

func newViewGame(t *testing.T) *Engine {
	t.Helper()
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), seer("s"), witch("wi"), guard("g"),
		villagers("v1", "v2", "v3"),
	)...)
	return g.e
}

// TestPlayerView_HidesOtherIdentities 视角只暴露本人有权知道的身份。
func TestPlayerView_HidesOtherIdentities(t *testing.T) {
	e := newViewGame(t)

	t.Run("平民只知道自己", func(t *testing.T) {
		v := e.PlayerView("v1")
		if v.Self.Role != RoleVillager {
			t.Errorf("自己的身份应当可见，实际 %v", v.Self.Role)
		}
		if len(v.Teammates) != 0 {
			t.Errorf("平民不应有队友信息，实际 %v", v.Teammates)
		}
		for _, p := range v.Players {
			if p.ID == "v1" {
				continue
			}
			if p.Role != RoleUnspecified {
				t.Errorf("不应看到 %s 的身份，实际 %v", p.ID, p.Role)
			}
		}
	})

	t.Run("狼人互相可见但看不到神职", func(t *testing.T) {
		v := e.PlayerView("w1")
		if len(v.Teammates) != 1 || v.Teammates[0] != "w2" {
			t.Errorf("狼队友: 期望 [w2]，实际 %v", v.Teammates)
		}
		for _, p := range v.Players {
			switch p.ID {
			case "w1", "w2":
				if p.Role != RoleWerewolf {
					t.Errorf("%s 对狼人应当可见为狼，实际 %v", p.ID, p.Role)
				}
			default:
				if p.Role != RoleUnspecified {
					t.Errorf("狼人不应看到 %s 的身份，实际 %v", p.ID, p.Role)
				}
			}
		}
	})

	t.Run("全场存活状态是公开的", func(t *testing.T) {
		v := e.PlayerView("v1")
		if len(v.Players) != 8 {
			t.Errorf("应当看到全部 8 名玩家，实际 %d", len(v.Players))
		}
		for _, p := range v.Players {
			if !p.Alive {
				t.Errorf("开局所有人都应存活，%s 却是 false", p.ID)
			}
		}
	})

	t.Run("玩家不存在返回 nil", func(t *testing.T) {
		if v := e.PlayerView("查无此人"); v != nil {
			t.Errorf("期望 nil，实际 %+v", v)
		}
	})
}

// TestPlayerView_WitchKillTargetFollowsRule 女巫的刀口可见性随解药存亡。
func TestPlayerView_WitchKillTargetFollowsRule(t *testing.T) {
	e := newViewGame(t)
	if _, err := e.EndPhase(); err != nil { // -> NIGHT_WOLF
		t.Fatal(err)
	}
	if err := e.SubmitSkillUse(&SkillUse{
		PlayerID: "w1", Skill: SkillKill, TargetID: "v1",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.EndPhase(); err != nil { // -> NIGHT_WITCH
		t.Fatal(err)
	}

	if got := e.PlayerView("wi").RoleInfo[RoleInfoKillTarget]; got != "v1" {
		t.Errorf("解药在手时女巫应看到刀口，实际 %q", got)
	}
	// 别人看不到
	for _, id := range []string{"v1", "w1", "s", "g"} {
		if got := e.PlayerView(id).RoleInfo[RoleInfoKillTarget]; got != "" {
			t.Errorf("%s 不应看到刀口，实际 %q", id, got)
		}
	}

	// 用掉解药后不再可见
	if err := e.SubmitSkillUse(&SkillUse{
		PlayerID: "wi", Skill: SkillAntidote, TargetID: "v1",
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatal(err)
		}
	}
	for e.Phase() != PhaseNightWitch {
		if _, err := e.EndPhase(); err != nil {
			t.Fatal(err)
		}
	}
	if got := e.PlayerView("wi").RoleInfo[RoleInfoKillTarget]; got != "" {
		t.Errorf("解药已用完，女巫不应再看到刀口，实际 %q", got)
	}
}

// TestPlayerView_AllowedSkillsGateAction AllowedSkills 就是「轮到我了吗」。
func TestPlayerView_AllowedSkillsGateAction(t *testing.T) {
	e := newViewGame(t)

	// NIGHT_GUARD：只有守卫能行动
	if got := e.PlayerView("g").AllowedSkills; len(got) != 1 ||
		got[0] != SkillProtect {
		t.Errorf("守卫阶段守卫应可守护，实际 %v", got)
	}
	for _, id := range []string{"w1", "s", "wi", "v1"} {
		if got := e.PlayerView(id).AllowedSkills; len(got) != 0 {
			t.Errorf("守卫阶段 %s 不应有可用技能，实际 %v", id, got)
		}
	}

	// 出局玩家没有可用技能
	e.state.applyEffect(NewSetAliveEffect("g", false))
	if got := e.PlayerView("g").AllowedSkills; len(got) != 0 {
		t.Errorf("已出局玩家不应有可用技能，实际 %v", got)
	}
}

// TestAudienceOf 效果的默认受众划分
// TestPlayerView_DeadWitchCannotSeeKillTarget 出局的女巫拿不到今晚的刀口。
//
// 「解藥未使用時可以得知狼人的殺害對象」的前提是她还在场上行动。
// 已出局的女巫在天亮公布之前就知道刀口，等于提前拿到了全场信息。
func TestPlayerView_DeadWitchCannotSeeKillTarget(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), witch("wi"), seer("s"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	// 第一夜刀死女巫，她不救自己
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "wi")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)
	g.assertAlive("wi", false, "第一夜被刀")

	// 第二夜狼队刀预言家，此刻停在女巫阶段
	g.toNextNight()
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "s")
	g.end(PhaseNightWitch)

	v := g.e.PlayerView("wi")
	if v == nil {
		t.Fatal("PlayerView 不应为 nil")
	}
	if v.Self.Alive {
		t.Fatal("前置条件：女巫此时应已出局")
	}
	if v.RoleInfo[RoleInfoAntidote] == "" {
		t.Fatal("前置条件：女巫的解药未使用")
	}
	if v.RoleInfo[RoleInfoKillTarget] != "" {
		t.Errorf("出局的女巫不应看到刀口，实际 %q", v.RoleInfo[RoleInfoKillTarget])
	}
}

// TestPlayerView_SelfDoesNotLeakProtection 被守护的玩家不该从自己的视图里读出来。
//
// 「守卫守了谁」是守卫独占的信息：被守的人一旦知道，就等于知道自己
// 今晚刀不死，也大幅缩小了守卫的范围。AudienceOf 把 PROTECT 判给守卫
// 一个人，PublicPlayerInfo 也刻意不含这个字段——Self 不能是漏的那半。
func TestPlayerView_SelfDoesNotLeakProtection(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), guard("g"),
		villagers("v1", "v2", "v3", "v4", "v5", "v6"),
	)...)

	g.mustUse("g", SkillProtect, "v1")
	g.end(PhaseNightWolf)

	// 前置：守护确实生效了（上帝视角看得到）
	if g.info("v1").RoundVar(PlayerRoundVarProtected) == "" {
		t.Fatal("前置条件：v1 应处于被守护状态")
	}

	// SelfInfo 里根本没有这个字段，序列化出去也不会带上
	data, err := json.Marshal(g.e.PlayerView("v1").Self)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if bytes.Contains(bytes.ToLower(data), []byte("protect")) {
		t.Errorf("玩家自视图不应包含守护状态，实际 %s", data)
	}
}

func TestAudienceOf(t *testing.T) {
	e := newViewGame(t)
	all := 8

	cases := []struct {
		name   string
		effect *Effect
		want   []string // nil 表示只校验数量
		count  int
	}{
		{"击杀全场可见", NewEffect(EventKill, "", "v1"), nil, all},
		{"放逐全场可见", NewEffect(EventEliminate, "", "v1"), nil, all},
		{"开枪全场可见", NewEffect(EventShoot, "h", "v1"), nil, all},
		{"平票全场可见", NewEffect(EventVoteTied, "", ""), nil, all},
		{"游戏结束全场可见", NewEffect(EventGameEnded, "", ""), nil, all},
		{"查验只给预言家", NewEffect(EventCheck, "s", "w1"), []string{"s"}, 1},
		{"守护只给守卫", NewEffect(EventProtect, "g", "v1"), []string{"g"}, 1},
		{"解药只给女巫", NewEffect(EventSave, "wi", "v1"), []string{"wi"}, 1},
		{"内部效果不给任何人", NewSetAliveEffect("v1", false), nil, 0},
		{"消耗解药不给任何人", NewSetPlayerVarEffect("wi", VarWitchAntidote, ""), nil, 0},
		{"触发效果不给任何人", NewAbilityTriggerEffect("h", PhaseNightHunter), nil, 0},
		{"被否决的用毒只给女巫本人", canceledEffect(
			NewEffect(EventPoison, "wi", "v1")), []string{"wi"}, 1},
		{"被否决的守护只给守卫本人", canceledEffect(
			NewEffect(EventProtect, "g", "v1")), []string{"g"}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, known := e.AudienceOf(tc.effect.ToEvent())
			if !known {
				t.Fatalf("引擎应当认得 %v", tc.effect.Type)
			}
			if len(got) != tc.count {
				t.Fatalf("受众数量: 期望 %d，实际 %d (%v)", tc.count, len(got), got)
			}
			for i, want := range tc.want {
				if got[i] != want {
					t.Errorf("受众[%d]: 期望 %s，实际 %s", i, want, got[i])
				}
			}
		})
	}

	if got, known := e.AudienceOf(nil); got != nil || known {
		t.Errorf("nil 效果应返回 (nil, false)，实际 (%v, %v)", got, known)
	}

	// 第三方自定义的外部事件类型：引擎无从判断可见性，必须说「不知道」，
	// 而不是给出一个看起来权威的空受众
	custom := NewEffect(EventType(50), "w1", "v1")
	if got, known := e.AudienceOf(custom.ToEvent()); known || got != nil {
		t.Errorf("未知外部类型应返回 (nil, false)，实际 (%v, %v)", got, known)
	}
}

// canceledEffect 把效果标成被否决，供表驱动用例内联构造。
func canceledEffect(e *Effect) *Effect {
	e.Cancel("test")
	return e
}

// TestAudienceOf_CoversEveryPublicEvent 每个外部可见的事件类型都要有明确受众，
// 新增事件类型时不能忘了在这里划分可见性。
//
// 遍历 proto 里的全部枚举值，而不是手写一份清单：手写清单挡不住
// 「新加了类型但忘了同步清单」——它恰恰是这个测试声称要挡的那类问题。
func TestAudienceOf_CoversEveryPublicEvent(t *testing.T) {
	e := newViewGame(t)

	for num, name := range eventTypeNames {
		typ := EventType(num)
		if typ == EventUnspecified || isInternalEvent(typ) {
			continue
		}
		ef := NewEffect(typ, "s", "v1")
		got, known := e.AudienceOf(ef.ToEvent())
		if !known {
			t.Errorf("外部事件 %s 没有划分受众", name)
			continue
		}
		if len(got) == 0 {
			t.Errorf("外部事件 %s 的受众为空", name)
		}
	}
}

// TestAudienceOf_UnknownActorGetsNobody 行动者不在场上时不该给出一个投递不到的 ID。
func TestAudienceOf_UnknownActorGetsNobody(t *testing.T) {
	e := newViewGame(t)

	canceled := NewSetAliveEffect("v1", false)
	canceled.Cancel("no poison")
	if got, known := e.AudienceOf(canceled.ToEvent()); len(got) != 0 || !known {
		t.Errorf("被否决效果的 source 不在场上，受众应为空，实际 (%v, %v)", got, known)
	}

	private := NewEffect(EventCheck, "查无此人", "v1")
	if got, _ := e.AudienceOf(private.ToEvent()); len(got) != 0 {
		t.Errorf("私密效果的 source 不在场上，受众应为空，实际 %v", got)
	}
}
