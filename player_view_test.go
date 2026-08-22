package werewolf

import (
	"testing"

	pb "github.com/Zereker/werewolf/proto"
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
		if v.Self.Role != pb.RoleType_ROLE_TYPE_VILLAGER {
			t.Errorf("自己的身份应当可见，实际 %v", v.Self.Role)
		}
		if len(v.Teammates) != 0 {
			t.Errorf("平民不应有队友信息，实际 %v", v.Teammates)
		}
		for _, p := range v.Players {
			if p.ID == "v1" {
				continue
			}
			if p.Role != pb.RoleType_ROLE_TYPE_UNSPECIFIED {
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
				if p.Role != pb.RoleType_ROLE_TYPE_WEREWOLF {
					t.Errorf("%s 对狼人应当可见为狼，实际 %v", p.ID, p.Role)
				}
			default:
				if p.Role != pb.RoleType_ROLE_TYPE_UNSPECIFIED {
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
		PlayerID: "w1", Skill: pb.SkillType_SKILL_TYPE_KILL, TargetID: "v1",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.EndPhase(); err != nil { // -> NIGHT_WITCH
		t.Fatal(err)
	}

	if got := e.PlayerView("wi").KillTarget; got != "v1" {
		t.Errorf("解药在手时女巫应看到刀口，实际 %q", got)
	}
	// 别人看不到
	for _, id := range []string{"v1", "w1", "s", "g"} {
		if got := e.PlayerView(id).KillTarget; got != "" {
			t.Errorf("%s 不应看到刀口，实际 %q", id, got)
		}
	}

	// 用掉解药后不再可见
	if err := e.SubmitSkillUse(&SkillUse{
		PlayerID: "wi", Skill: pb.SkillType_SKILL_TYPE_ANTIDOTE, TargetID: "v1",
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatal(err)
		}
	}
	for e.Phase() != pb.PhaseType_PHASE_TYPE_NIGHT_WITCH {
		if _, err := e.EndPhase(); err != nil {
			t.Fatal(err)
		}
	}
	if got := e.PlayerView("wi").KillTarget; got != "" {
		t.Errorf("解药已用完，女巫不应再看到刀口，实际 %q", got)
	}
}

// TestPlayerView_AllowedSkillsGateAction AllowedSkills 就是「轮到我了吗」。
func TestPlayerView_AllowedSkillsGateAction(t *testing.T) {
	e := newViewGame(t)

	// NIGHT_GUARD：只有守卫能行动
	if got := e.PlayerView("g").AllowedSkills; len(got) != 1 ||
		got[0] != pb.SkillType_SKILL_TYPE_PROTECT {
		t.Errorf("守卫阶段守卫应可守护，实际 %v", got)
	}
	for _, id := range []string{"w1", "s", "wi", "v1"} {
		if got := e.PlayerView(id).AllowedSkills; len(got) != 0 {
			t.Errorf("守卫阶段 %s 不应有可用技能，实际 %v", id, got)
		}
	}

	// 出局玩家没有可用技能
	e.state.applyEffect(NewEffect(pb.EventType_EVENT_TYPE_KILL, "", "g"))
	if got := e.PlayerView("g").AllowedSkills; len(got) != 0 {
		t.Errorf("已出局玩家不应有可用技能，实际 %v", got)
	}
}

// TestAudienceOf 效果的默认受众划分
func TestAudienceOf(t *testing.T) {
	e := newViewGame(t)
	all := 8

	cases := []struct {
		name   string
		effect *Effect
		want   []string // nil 表示只校验数量
		count  int
	}{
		{"击杀全场可见", NewEffect(pb.EventType_EVENT_TYPE_KILL, "", "v1"), nil, all},
		{"放逐全场可见", NewEffect(pb.EventType_EVENT_TYPE_ELIMINATE, "", "v1"), nil, all},
		{"开枪全场可见", NewEffect(pb.EventType_EVENT_TYPE_SHOOT, "h", "v1"), nil, all},
		{"游戏结束全场可见", NewEffect(pb.EventType_EVENT_TYPE_GAME_ENDED, "", ""), nil, all},
		{"查验只给预言家", NewEffect(pb.EventType_EVENT_TYPE_CHECK, "s", "w1"), []string{"s"}, 1},
		{"守护只给守卫", NewEffect(pb.EventType_EVENT_TYPE_PROTECT, "g", "v1"), []string{"g"}, 1},
		{"解药只给女巫", NewEffect(pb.EventType_EVENT_TYPE_SAVE, "wi", "v1"), []string{"wi"}, 1},
		{"内部效果不给任何人", NewEffect(pb.EventType_EVENT_TYPE_SET_NIGHT_KILL, "", "v1"), nil, 0},
		{"消耗解药不给任何人", NewEffect(pb.EventType_EVENT_TYPE_USE_ANTIDOTE, "wi", ""), nil, 0},
		{"触发效果不给任何人", NewAbilityTriggerEffect("h", pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER), nil, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := e.AudienceOf(tc.effect)
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

	if got := e.AudienceOf(nil); got != nil {
		t.Errorf("nil 效果应返回 nil，实际 %v", got)
	}
}

// TestAudienceOf_CoversEveryPublicEvent 每个外部可见的事件类型都要有明确受众，
// 新增事件类型时不能忘了在这里划分可见性。
func TestAudienceOf_CoversEveryPublicEvent(t *testing.T) {
	e := newViewGame(t)

	external := []pb.EventType{
		pb.EventType_EVENT_TYPE_GAME_STARTED,
		pb.EventType_EVENT_TYPE_GAME_ENDED,
		pb.EventType_EVENT_TYPE_KILL,
		pb.EventType_EVENT_TYPE_PROTECT,
		pb.EventType_EVENT_TYPE_SAVE,
		pb.EventType_EVENT_TYPE_POISON,
		pb.EventType_EVENT_TYPE_CHECK,
		pb.EventType_EVENT_TYPE_ELIMINATE,
		pb.EventType_EVENT_TYPE_SHOOT,
		pb.EventType_EVENT_TYPE_SKIP,
	}
	for _, typ := range external {
		ef := NewEffect(typ, "s", "v1")
		if got := e.AudienceOf(ef); len(got) == 0 {
			t.Errorf("外部事件 %v 没有划分受众", typ)
		}
	}
}
