package werewolf

import (
	"encoding/json"
	"testing"
)

// TestEnumJSON_NamesNotNumbers 枚举的 JSON 是名字，不是编号。
//
// 编号是给存储省字节用的，不该是给人和跨语言客户端用的：
// example/netserver 推给客户端的曾经是 {"role":2,"phase":21}，
// 每个客户端都得自己维护一张对照表。
func TestEnumJSON_NamesNotNumbers(t *testing.T) {
	type box struct {
		Phase PhaseType    `json:"phase"`
		Role  RoleType     `json:"role"`
		Camp  Camp         `json:"camp"`
		Cat   RoleCategory `json:"category"`
		Event EventType    `json:"event"`
		Skill SkillType    `json:"skill"`
		Code  ErrorCode    `json:"code"`
	}

	in := box{
		PhaseNightGuard, RoleWerewolf, CampEvil, RoleCategoryGod,
		EventKill, SkillAntidote, CodePlayerDead,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	want := `{"phase":"NIGHT_GUARD","role":"WEREWOLF","camp":"EVIL","category":"GOD",` +
		`"event":"KILL","skill":"ANTIDOTE","code":"PLAYER_DEAD"}`
	if string(b) != want {
		t.Errorf("序列化结果:\n  期望 %s\n  实际 %s", want, b)
	}

	var out box
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if out != in {
		t.Errorf("往返不一致: %+v vs %+v", in, out)
	}
}

// TestEnumJSON_CustomValuesRoundTrip 第三方的自定义取值没有名字，按编号往返。
//
// 这个库支持从 1000 起的自定义角色、技能与阶段。只输出名字的话，
// 那些取值就再也读不回来了——扩展契约会当场断掉。
func TestEnumJSON_CustomValuesRoundTrip(t *testing.T) {
	type box struct {
		Phase PhaseType `json:"phase"`
		Role  RoleType  `json:"role"`
		Skill SkillType `json:"skill"`
	}

	in := box{PhaseType(1000), RoleType(1000), SkillType(1000)}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if want := `{"phase":1000,"role":1000,"skill":1000}`; string(b) != want {
		t.Errorf("自定义取值应当按编号写:\n  期望 %s\n  实际 %s", want, b)
	}

	var out box
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if out != in {
		t.Errorf("往返不一致: %+v vs %+v", in, out)
	}
}

// TestEnumJSON_AcceptsNumbers 编号也照收——手写的报文、别处来的数据都可能是编号。
func TestEnumJSON_AcceptsNumbers(t *testing.T) {
	var v struct {
		Phase PhaseType `json:"phase"`
		Role  RoleType  `json:"role"`
	}
	if err := json.Unmarshal([]byte(`{"phase":21,"role":2}`), &v); err != nil {
		t.Fatalf("按编号回读失败: %v", err)
	}
	if v.Phase != PhaseNightGuard || v.Role != RoleWerewolf {
		t.Errorf("按编号回读错了: %v / %v", v.Phase, v.Role)
	}
}

// TestEnumJSON_RejectsUnknownName 不认识的名字要报错，不能静默变成零值。
//
// 零值在这个库里大多是 UNSPECIFIED，静默退回它比报错难查得多——
// 一个拼错的阶段名会变成「未指定阶段」，然后在很远的地方出问题。
func TestEnumJSON_RejectsUnknownName(t *testing.T) {
	var v struct {
		Phase PhaseType `json:"phase"`
	}
	err := json.Unmarshal([]byte(`{"phase":"NO_SUCH_PHASE"}`), &v)
	if err == nil {
		t.Fatal("不认识的名字应当报错")
	}
	if v.Phase != PhaseUnspecified {
		t.Errorf("报错之后取值应当还是零值，实际 %v", v.Phase)
	}
}

// TestSnapshot_JSONIsReadable 存档现在是人能读的。
func TestSnapshot_JSONIsReadable(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), witch("wi"), seer("s"),
		villagers("v1", "v2", "v3", "v4"),
	)...)
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)

	raw, err := json.Marshal(g.e.Snapshot())
	if err != nil {
		t.Fatalf("序列化快照失败: %v", err)
	}

	var snap Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("回读快照失败: %v", err)
	}
	restored, err := RestoreEngine(nil, &snap)
	if err != nil {
		t.Fatalf("恢复失败: %v", err)
	}
	if restored.Phase() != g.e.Phase() || restored.Round() != g.e.Round() {
		t.Errorf("恢复后局面不一致: %v/%d vs %v/%d",
			restored.Phase(), restored.Round(), g.e.Phase(), g.e.Round())
	}

	// 存档里出现的应当是名字
	if !jsonContains(raw, `"phase":"NIGHT_WITCH"`) {
		t.Errorf("快照里的阶段应当是名字，实际 %s", raw)
	}
	if !jsonContains(raw, `"role":"WEREWOLF"`) {
		t.Errorf("快照里的角色应当是名字，实际 %s", raw)
	}
}

func jsonContains(raw []byte, sub string) bool {
	return len(raw) > 0 && len(sub) > 0 &&
		func() bool {
			for i := 0; i+len(sub) <= len(raw); i++ {
				if string(raw[i:i+len(sub)]) == sub {
					return true
				}
			}
			return false
		}()
}
