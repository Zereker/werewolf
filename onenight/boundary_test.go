package onenight

import (
	"sort"
	"strings"
	"testing"

	"github.com/Zereker/werewolf/engine"
)

// TestBoundary_WolvesRecogniseEachOtherMinionSeesThemNotViceVersa
// 狼互认；爪牙看得见狼，狼看不见爪牙。
//
// 这是**单向**的不对称，与阿瓦隆的奥伯伦（双向隔离）是两个方向。
// 内核允许不对称正是为了这一类。
func TestBoundary_WolvesRecogniseEachOtherMinionSeesThemNotViceVersa(t *testing.T) {
	g := newGame(t,
		[CenterCount]engine.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w1", RoleWerewolf), at("w2", RoleWerewolf),
		at("m", RoleMinion), at("v", RoleVillager))

	if got := g.info("w1")["wolves"]; got != "w2" {
		t.Errorf("w1 应当认得 w2，实际 %q", got)
	}
	if got := g.info("m")["wolves"]; got != "w1,w2" {
		t.Errorf("爪牙应当看见两只狼，实际 %q", got)
	}
	// 狼的信息里没有爪牙——单向。
	for _, id := range []string{"w1", "w2"} {
		if strings.Contains(g.info(id)["wolves"], "m") {
			t.Errorf("%s 不该看见爪牙", id)
		}
	}
	// 队友关系同样只在狼之间。
	if mates := g.e.Teammates("m"); len(mates) != 0 {
		t.Errorf("爪牙不是任何人的队友，实际 %v", mates)
	}
	if got := g.info("v")["wolves"]; got != "" {
		t.Errorf("村民什么都不该看到，实际 %q", got)
	}
}

// TestBoundary_LoneWolfSeesEmptyList 独狼看到的是一份空名单。
//
// 「名单是空的」本身就是信息：它等于「我是独狼」，可以去看一张中央牌。
// 因此空名单也要送到，不能因为空就不给。
func TestBoundary_LoneWolfSeesEmptyList(t *testing.T) {
	g := newGame(t,
		[CenterCount]engine.RoleType{RoleWerewolf, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	info := g.info("w")
	got, ok := info["wolves"]
	if !ok {
		t.Fatal("独狼也该收到 wolves 这一项——空名单是信息")
	}
	if got != "" {
		t.Errorf("场上只有一只狼，名单应当是空的，实际 %q", got)
	}
}

// TestBoundary_LoneMasonKnowsTheOtherIsInCenter 只有一名守夜人时名单是空的。
func TestBoundary_LoneMasonKnowsTheOtherIsInCenter(t *testing.T) {
	g := newGame(t,
		[CenterCount]engine.RoleType{RoleMason, RoleVillager, RoleVillager},
		at("m", RoleMason), at("v1", RoleVillager), at("v2", RoleVillager))

	if got, ok := g.info("m")["masons"]; !ok || got != "" {
		t.Errorf("另一名守夜人在中央，名单应当是空的，实际 %q（存在=%v）", got, ok)
	}
}

// TestBoundary_NightEventsGoOnlyToTheActor 夜里的事只告诉当事人。
//
// 捣蛋鬼那条尤其要紧：被换的两个人也不能知道。
func TestBoundary_NightEventsGoOnlyToTheActor(t *testing.T) {
	g := newGame(t,
		[CenterCount]engine.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("t", RoleTroublemaker), at("w", RoleWerewolf),
		at("v1", RoleVillager), at("v2", RoleVillager))

	cases := []struct {
		event *engine.Event
		want  []string
		why   string
	}{
		{engine.NewEffect(EventMeddled, "t", "").ToEvent(), []string{"t"}, "只有捣蛋鬼自己知道"},
		{engine.NewEffect(EventSeerLook, "v1", "w").ToEvent(), []string{"v1"}, "只有预言家自己知道"},
		{engine.NewEffect(EventDrunkSwap, "v2", "").ToEvent(), []string{"v2"}, "酒鬼自己也只知道他换了"},
		{engine.NewEffect(EventLynched, "", "w").ToEvent(), []string{"t", "v1", "v2", "w"}, "出局是公开的"},
		{engine.NewEffect(EventVoted, "v1", "w").ToEvent(), []string{"t", "v1", "v2", "w"}, "投票是公开的"},
		{engine.NewEffect(EventNoOneDies, "", "").ToEvent(), []string{"t", "v1", "v2", "w"}, "无人出局是公开的"},
	}

	for _, c := range cases {
		got, known := g.e.AudienceOf(c.event)
		if !known {
			t.Errorf("%v 应当有明确的受众判定（%s）", c.event.Type, c.why)
			continue
		}
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%v 的受众 = %v，期望 %v（%s）", c.event.Type, got, c.want, c.why)
		}
	}

	// 规则没管的事件交回「不知道」，由调用方自己路由。
	if _, known := g.e.AudienceOf(engine.NewEffect(engine.EventType("SOMETHING_ELSE"), "t", "").ToEvent()); known {
		t.Error("本包没有为这个事件表态，答案该是「不知道」")
	}
}

// TestBoundary_StatePrimitivesNeverReachPlayers 状态原语一条都不外发。
//
// 这一套规则里这条格外要紧：「三号现在手上是狼人牌」就是一条 SET_VAR。
// 它是内核不可配置的那一条，本包写不写 AudienceProvider 都拦得住。
func TestBoundary_StatePrimitivesNeverReachPlayers(t *testing.T) {
	g := newGame(t,
		[CenterCount]engine.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	primitives := []*engine.Effect{
		setCard("v1", RoleWerewolf),
		setCenterCard(0, RoleWerewolf),
		learnSelf("v1", RoleWerewolf),
		engine.NewSetAliveEffect("v1", false),
	}
	for _, ef := range primitives {
		got, known := g.e.AudienceOf(ef.ToEvent())
		if !known {
			t.Errorf("%v 应当是明确的判定，不是「不知道」", ef.Type)
		}
		if len(got) != 0 {
			t.Errorf("%v 是状态原语，不该发给任何人，实际 %v", ef.Type, got)
		}
	}
}

// TestBoundary_SpeechIsPublicAllGame 发言全场可听，全程。
func TestBoundary_SpeechIsPublicAllGame(t *testing.T) {
	g := newGame(t,
		[CenterCount]engine.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	g.advance(PhaseDay)
	got := g.e.MessageReceivers("w")
	sort.Strings(got)
	if strings.Join(got, ",") != "v1,v2,w" {
		t.Errorf("发言应当全场可听，实际 %v", got)
	}
}

// TestCampOf 翻牌时算阵营。爪牙属狼队但他不是狼牌。
func TestCampOf(t *testing.T) {
	cases := []struct {
		role engine.RoleType
		want engine.Camp
	}{
		{RoleWerewolf, CampWolf},
		{RoleMinion, CampWolf},
		{RoleTanner, CampTanner},
		{RoleVillager, CampVillage},
		{RoleSeer, CampVillage},
		{RoleHunter, CampVillage},
	}
	for _, c := range cases {
		if got := CampOf(c.role); got != c.want {
			t.Errorf("CampOf(%v) = %v，期望 %v", c.role, got, c.want)
		}
	}
	// 「狼人出局」数的是狼人牌，不是狼队——爪牙出局不算。
	if isWolfCard(RoleMinion) {
		t.Error("爪牙属狼队，但他不是狼人牌")
	}
	if !isWolfCard(RoleWerewolf) {
		t.Error("狼人牌就是狼人牌")
	}
}
