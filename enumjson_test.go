package werewolf

import (
	"encoding/json"
	"github.com/Zereker/hiddenrole"
	"strings"
	"testing"
)

// 枚举的底层是字符串，JSON 因此不需要任何自定义的 Marshal/Unmarshal——
// 名字直接就是值。此前它们是整数，每个类型都得挂一张「编号到名字」的
// 对照表和一对 JSON 方法，一百多行代码只为把值翻译回它本来的样子。
//
// 这几条测试守的是那件事真的成立：存档里是人能读的名字，读回来还是原值，
// 第三方自己定义的取值一视同仁。

func TestEnumJSON_MarshalsAsNames(t *testing.T) {
	type box struct {
		Phase PhaseType            `json:"phase"`
		Role  RoleType             `json:"role"`
		Skill SkillType            `json:"skill"`
		Event EventType            `json:"event"`
		Camp  Camp                 `json:"camp"`
		Cat   RoleCategory         `json:"category"`
		Code  hiddenrole.ErrorCode `json:"code"`
	}

	got, err := json.Marshal(box{
		Phase: PhaseNightGuard, Role: RoleWitch, Skill: SkillAntidote,
		Event: hiddenrole.EventSetAlive, Camp: CampEvil, Cat: RoleCategoryGod,
		Code: hiddenrole.CodeInvalidBoard,
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	want := `{"phase":"NIGHT_GUARD","role":"WITCH","skill":"ANTIDOTE",` +
		`"event":"SET_ALIVE","camp":"EVIL","category":"GOD","code":"INVALID_BOARD"}`
	if string(got) != want {
		t.Errorf("枚举应当按名字序列化\n  期望 %s\n  实际 %s", want, got)
	}
}

func TestEnumJSON_RoundTrip(t *testing.T) {
	type box struct {
		Phase PhaseType `json:"phase"`
		Role  RoleType  `json:"role"`
		Skill SkillType `json:"skill"`
	}

	cases := map[string]box{
		"内置取值": {PhaseNightWitch, RoleWitch, SkillPoison},
		// 第三方定义的取值与内置的没有身份差别——字符串不会撞号，
		// 也就不再需要「自定义取值从 1000 起」这类避让约定
		"第三方取值": {PhaseType("PHASE_WOLF_KING"), RoleType("WOLF_KING"), SkillType("WOLF_CLAW")},
		"零值":    {},
	}

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(in)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			var out box
			if err := json.Unmarshal(data, &out); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if out != in {
				t.Errorf("往返后不一致: %+v -> %s -> %+v", in, data, out)
			}
		})
	}
}

// TestEnumJSON_ZeroValueIsUnspecified 零值即「未指定」。
//
// 字符串枚举顺带解决了一件旧事：整数时代零值是 0，而 0 恰好也是
// UNSPECIFIED 的编号——两者相等是巧合，不是设计。现在零值是空串，
// 而空串就是 UNSPECIFIED 本身。
func TestEnumJSON_ZeroValueIsUnspecified(t *testing.T) {
	var (
		p PhaseType
		r RoleType
		s SkillType
		e EventType
		c Camp
	)
	if p != hiddenrole.PhaseUnspecified || r != hiddenrole.RoleUnspecified || s != hiddenrole.SkillUnspecified ||
		e != hiddenrole.EventUnspecified || c != hiddenrole.CampUnspecified {
		t.Error("零值应当等于各自的 Unspecified")
	}
	// String() 仍然给出可读的名字，日志里不会出现一个空白
	if p.String() != "UNSPECIFIED" || r.String() != "UNSPECIFIED" ||
		s.String() != "UNSPECIFIED" || e.String() != "UNSPECIFIED" ||
		c.String() != "UNSPECIFIED" {
		t.Error("零值的 String() 应当是 UNSPECIFIED")
	}
}

// TestEnumJSON_SnapshotIsReadable 快照里的枚举是人能读的。
//
// 这是当初把快照从编号改成名字的理由：存档要给人看，也可能被别的语言读，
// 编号对不上号。现在这一点由类型本身保证，不再依赖一层翻译。
func TestEnumJSON_SnapshotIsReadable(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), seer("s"), witch("wi"), villagers("v1", "v2"),
	)...)

	data, err := json.Marshal(g.e.Snapshot())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, want := range []string{
		`"role":"WEREWOLF"`,
		`"role":"WITCH"`,
		`"phase":"NIGHT_GUARD"`,
		`"camp":"EVIL"`,
		`"category":"GOD"`,
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("快照里应当出现 %s，实际:\n%s", want, data)
		}
	}

	// 反过来：不该再出现任何裸编号形式的枚举
	if strings.Contains(string(data), `"role":2`) || strings.Contains(string(data), `"phase":21`) {
		t.Errorf("快照里不该出现编号形式的枚举:\n%s", data)
	}
}
