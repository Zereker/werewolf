package onenight

import (
	"testing"

	"github.com/Zereker/hiddenrole"
)

// game 一局测试用的对局，外加几个断言辅助。
type game struct {
	t *testing.T
	e *hiddenrole.Engine
}

// newGame 开一局：seats 是「玩家 ID -> 发到手的牌」，center 是中央三张。
func newGame(t *testing.T, center [CenterCount]hiddenrole.RoleType, seats ...seat) *game {
	t.Helper()

	e, err := hiddenrole.NewEngine(GameConfig(), Options(center)...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	for _, s := range seats {
		if err := e.AddPlayer(s.id, s.role); err != nil {
			t.Fatalf("AddPlayer(%s): %v", s.id, err)
		}
	}
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return &game{t: t, e: e}
}

type seat struct {
	id   string
	role hiddenrole.RoleType
}

func at(id string, role hiddenrole.RoleType) seat { return seat{id, role} }

// use 提交一次技能，失败即终止。
func (g *game) use(playerID string, skill hiddenrole.SkillType, targets ...string) {
	g.t.Helper()
	err := g.e.SubmitSkillUse(&hiddenrole.SkillUse{
		PlayerID: playerID, Skill: skill, Targets: targets,
	})
	if err != nil {
		g.t.Fatalf("%s 提交 %v 失败: %v", playerID, skill, err)
	}
}

// end 结束当前阶段，并断言走到了 want。
func (g *game) end(want hiddenrole.PhaseType) {
	g.t.Helper()
	if _, err := g.e.EndPhase(); err != nil {
		g.t.Fatalf("EndPhase: %v", err)
	}
	if got := g.e.Status().Phase; got != want {
		g.t.Fatalf("阶段 = %v，期望 %v", got, want)
	}
}

// advance 一直推进到某个阶段为止。
//
// 比 end 好数：end 是「结束当前阶段，落到 want」，写一串的时候很容易差一格。
func (g *game) advance(to hiddenrole.PhaseType) {
	g.t.Helper()
	for i := 0; i < 20; i++ {
		if g.e.Status().Phase == to {
			return
		}
		if _, err := g.e.EndPhase(); err != nil {
			g.t.Fatalf("推进到 %v: %v", to, err)
		}
	}
	g.t.Fatalf("推了 20 步还没到 %v，现在在 %v", to, g.e.Status().Phase)
}

// toVote 一路推到投票，中途不做任何夜晚动作。
func (g *game) toVote() {
	g.t.Helper()
	for _, p := range []hiddenrole.PhaseType{
		PhaseNightMinion, PhaseNightMason, PhaseNightSeer, PhaseNightRobber,
		PhaseNightTroublemake, PhaseNightDrunk, PhaseNightInsomniac,
		PhaseDay, PhaseVote,
	} {
		g.end(p)
	}
}

// card 这名玩家现在手上是什么牌。
func (g *game) card(playerID string) hiddenrole.RoleType {
	g.t.Helper()
	return card(g.e.View(), playerID)
}

// info 这名玩家的视角里，角色专属信息是什么。
func (g *game) info(playerID string) map[string]string {
	g.t.Helper()
	v := g.e.PlayerView(playerID)
	if v == nil {
		g.t.Fatalf("%s 没有视角", playerID)
	}
	return v.RoleInfo
}

// TestGame_FullNightAndVote 跑通一整局：夜里换牌，白天投票，判出胜负。
//
// 这是这一套规则包的第一条测试，验的是「它到底能不能在这个内核上跑起来」。
func TestGame_FullNightAndVote(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleWerewolf, RoleVillager},
		at("w", RoleWerewolf), at("s", RoleSeer), at("r", RoleRobber),
		at("t", RoleTroublemaker), at("v", RoleVillager))

	// 狼：场上有两只吗？只有一只（另一张在中央），所以可以看中央第 0 张。
	g.use("w", SkillPeekCenter0)
	g.end(PhaseNightMinion)
	g.end(PhaseNightMason)
	g.end(PhaseNightSeer)

	// 预言家看狼。
	g.use("s", SkillSeerPlayer, "w")
	g.end(PhaseNightRobber)

	// 抢劫者抢狼的牌——他因此变成狼队，但**夜里已经不会再做狼的事**。
	g.use("r", SkillRob, "w")
	g.end(PhaseNightTroublemake)

	if g.card("r") != RoleWerewolf {
		t.Fatalf("抢劫者抢了狼的牌，现在应当拿着 WEREWOLF，实际 %v", g.card("r"))
	}
	if g.card("w") != RoleRobber {
		t.Fatalf("狼的牌被抢走，现在应当拿着 ROBBER，实际 %v", g.card("w"))
	}

	// 捣蛋鬼交换村民与预言家的牌，两人都不知道。
	g.use("t", SkillMeddle, "v", "s")
	g.end(PhaseNightDrunk)
	g.end(PhaseNightInsomniac)
	g.end(PhaseDay)
	g.end(PhaseVote)

	// 预言家看到的仍然是**当时**的那一张，不是现在的。
	if got := g.info("s")["learn.player.w"]; got != string(RoleWerewolf) {
		t.Errorf("预言家当时看到的是 WEREWOLF，现在读到 %q", got)
	}
	if g.card("w") == RoleWerewolf {
		t.Error("狼的牌已经被抢走了，这条测试没测到「信息会过期」")
	}

	// 全票投抢劫者——他现在拿着狼人牌，村民赢。
	for _, id := range []string{"w", "s", "t", "v"} {
		g.use(id, SkillVote, "r")
	}
	g.use("r", SkillVote, "w")
	g.end(hiddenrole.PhaseEnd)

	st := g.e.Status()
	if !st.Over {
		t.Fatal("投票结束游戏就该结束")
	}
	if !Won(st.Winner, CampVillage) {
		t.Errorf("出局的人拿着狼人牌，村民队应当赢，实际 %v", st.Winner)
	}
	if p, _ := g.e.PlayerInfo("r"); p.Alive {
		t.Error("被票最多的人应当出局")
	}
}

// voteAll 全员投同一个人。
func (g *game) voteAll(target string) {
	g.t.Helper()
	for _, p := range g.e.View().AllPlayers() {
		want := target
		if p.ID == target {
			// 不能投自己，随便换一个别人。
			for _, q := range g.e.View().AllPlayers() {
				if q.ID != target {
					want = q.ID
					break
				}
			}
		}
		g.use(p.ID, SkillVote, want)
	}
}

// TestVictory_WolfDies 至少一名狼人出局 → 村民队赢。
func TestVictory_WolfDies(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	g.toVote()
	g.voteAll("w")
	g.end(hiddenrole.PhaseEnd)

	if got := g.e.Status().Winner; !Won(got, CampVillage) {
		t.Errorf("狼出局，村民队应当赢，实际 %v", got)
	}
	if Won(g.e.Status().Winner, CampWolf) {
		t.Error("狼出局了，狼队不该赢")
	}
}

// TestVictory_NoWolfDies 场上有狼且没有狼出局 → 狼队赢。
func TestVictory_NoWolfDies(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	g.toVote()
	g.voteAll("v1")
	g.end(hiddenrole.PhaseEnd)

	if got := g.e.Status().Winner; !Won(got, CampWolf) {
		t.Errorf("没有狼出局，狼队应当赢，实际 %v", got)
	}
}

// TestVictory_NoWolfInPlayAndNobodyDies 场上没有狼且无人出局 → 村民队赢。
//
// 「无人出局」由「每人恰好各得一票」达成——这是官方规则明写的一条，
// 不是平票的特例。三个人各投下一个，正好一人一票。
func TestVictory_NoWolfInPlayAndNobodyDies(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleWerewolf, RoleVillager, RoleVillager},
		at("v1", RoleVillager), at("v2", RoleSeer), at("v3", RoleVillager))

	g.toVote()
	g.use("v1", SkillVote, "v2")
	g.use("v2", SkillVote, "v3")
	g.use("v3", SkillVote, "v1")
	g.end(hiddenrole.PhaseEnd)

	for _, p := range g.e.View().AllPlayers() {
		if !p.Alive {
			t.Fatalf("每人各得一票时不该有人出局，%s 却出局了", p.ID)
		}
	}
	if got := g.e.Status().Winner; !Won(got, CampVillage) {
		t.Errorf("场上没有狼且无人出局，村民队应当赢，实际 %v", got)
	}
}

// TestVictory_TannerDiesAlone 皮匠出局且无狼出局 → 皮匠独赢，狼不赢。
func TestVictory_TannerDiesAlone(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("tn", RoleTanner), at("v", RoleVillager))

	g.toVote()
	g.voteAll("tn")
	g.end(hiddenrole.PhaseEnd)

	got := g.e.Status().Winner
	if !Won(got, CampTanner) {
		t.Errorf("皮匠出局，他应当赢，实际 %v", got)
	}
	if Won(got, CampWolf) {
		t.Errorf("皮匠出局时狼不该赢，实际 %v", got)
	}
	if Won(got, CampVillage) {
		t.Errorf("没有狼出局，村民队不该赢，实际 %v", got)
	}
}

// TestVictory_TannerAndWolfBothDie 皮匠与狼同时出局 → 皮匠与村民都赢。
//
// **这是内核给不出的答案**：VictoryChecker 返回一个 Camp，而这里有两个
// 赢家。本包把它们拼成一个字符串（"TANNER+VILLAGE"），编码与解码的规矩
// 只能由规则包自己带着。见 SCARS.md 疤 5。
func TestVictory_TannerAndWolfBothDie(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("tn", RoleTanner),
		at("v1", RoleVillager), at("v2", RoleVillager))

	g.toVote()
	// 狼与皮匠各得两票，并列最高 → 两人一起出局。
	// 谁也不能投自己，所以票要这么绕：
	g.use("v1", SkillVote, "w")
	g.use("tn", SkillVote, "w") // 狼 2 票
	g.use("v2", SkillVote, "tn")
	g.use("w", SkillVote, "tn") // 皮匠 2 票
	g.end(hiddenrole.PhaseEnd)

	for _, id := range []string{"w", "tn"} {
		if p, _ := g.e.PlayerInfo(id); p.Alive {
			t.Fatalf("平票时并列最高的都该出局，%s 却活着", id)
		}
	}

	got := g.e.Status().Winner
	if !Won(got, CampTanner) {
		t.Errorf("皮匠出局，他应当赢，实际 %v", got)
	}
	if !Won(got, CampVillage) {
		t.Errorf("有狼出局，村民队也应当赢，实际 %v", got)
	}
	if len(Winners(got)) != 2 {
		t.Errorf("应当有两个赢家，实际 %v", Winners(got))
	}
}

// TestHunter_TakesHisVoteWithHim 猎人出局时，他投的那个人也出局。
//
// 「猎人」按**现在手上那张牌**算：天亮翻的是手上的牌，抢到猎人牌的人就是
// 猎人，而发到手是猎人、后来被换走的那个人不是。
func TestHunter_TakesHisVoteWithHim(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("h", RoleHunter), at("w", RoleWerewolf),
		at("v1", RoleVillager), at("v2", RoleVillager))

	g.toVote()
	g.use("v1", SkillVote, "h")
	g.use("v2", SkillVote, "h")
	g.use("w", SkillVote, "h") // 猎人三票，出局
	g.use("h", SkillVote, "w") // 他投的是狼 —— 狼被带走
	g.end(hiddenrole.PhaseEnd)

	if p, _ := g.e.PlayerInfo("h"); p.Alive {
		t.Fatal("猎人得票最多，应当出局")
	}
	if p, _ := g.e.PlayerInfo("w"); p.Alive {
		t.Fatal("猎人出局时应当带走他投的那个人")
	}
	if got := g.e.Status().Winner; !Won(got, CampVillage) {
		t.Errorf("狼被猎人带走了，村民队应当赢，实际 %v", got)
	}
}

// TestNightAbilityFollowsDealtCard_NotHeldCard
// 夜里做什么由**发到手**的那张牌决定，不由现在手上那张决定。
//
// 这是这一套规则的支点，也是它与前两套最不一样的地方。抢劫者抢走狼人牌
// 之后不会变成狼、不会跟狼一起醒；而狼的牌被抢走之后，他那一夜**已经动过了**。
func TestNightAbilityFollowsDealtCard_NotHeldCard(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("r", RoleRobber),
		at("i", RoleInsomniac), at("v", RoleVillager))

	// 狼阶段：场上只有一只狼，他能看中央牌。
	g.use("w", SkillPeekCenter1)

	// 抢劫者抢狼的牌。提交要等这个阶段结束才结算，所以先推过去。
	g.advance(PhaseNightRobber)
	g.use("r", SkillRob, "w")
	g.advance(PhaseNightTroublemake)

	if g.card("r") != RoleWerewolf {
		t.Fatalf("抢劫者现在应当拿着狼人牌，实际 %v", g.card("r"))
	}

	// 抢劫者现在拿着狼人牌，但他的**队友名单**是空的——
	// 互认发生在他抢牌之前，而且按发到手的牌算。
	if mates := g.e.Teammates("r"); len(mates) != 0 {
		t.Errorf("抢劫者不该出现在狼的队友关系里，实际 %v", mates)
	}
	// 反过来，狼的牌被抢走了，他自己仍然知道自己发到手是狼。
	if got := g.e.PlayerView("w").Self.Role; got != RoleWerewolf {
		t.Errorf("发到手的牌不该变，实际 %v", got)
	}

	g.advance(PhaseDay)

	// 失眠者最后动，看到的是所有交换之后的结果。
	if got := g.info("i")["learn.self"]; got != string(RoleInsomniac) {
		t.Errorf("没人动失眠者的牌，他应当看到 INSOMNIAC，实际 %q", got)
	}
}

// TestDrunk_DoesNotKnowWhatHeHolds 酒鬼换了牌，而且不知道换到了什么。
//
// 这是内核那条「不给玩家自由格式状态口袋」的规矩在这一套规则里的价值：
// 酒鬼手上的牌是一项整局状态，若内核默认把 Vars 交给玩家，这个角色
// 当场就不成立了。
func TestDrunk_DoesNotKnowWhatHeHolds(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleWerewolf, RoleVillager, RoleVillager},
		at("d", RoleDrunk), at("v1", RoleVillager), at("v2", RoleVillager))

	g.advance(PhaseNightDrunk)
	g.use("d", SkillDrinkCenter0)
	g.advance(PhaseNightInsomniac)

	if g.card("d") != RoleWerewolf {
		t.Fatalf("酒鬼应当拿到中央第 0 张（狼人牌），实际 %v", g.card("d"))
	}

	view := g.e.PlayerView("d")
	for k, v := range view.RoleInfo {
		t.Errorf("酒鬼不该知道任何东西，却看到 %s=%s", k, v)
	}
	if view.Self.Camp != hiddenrole.CampUnspecified {
		t.Errorf("酒鬼不该知道自己现在算哪边，Self.Camp = %v", view.Self.Camp)
	}
	if view.Self.Role != RoleDrunk {
		t.Errorf("他知道的只有「我发到手是酒鬼」，实际 %v", view.Self.Role)
	}
}

// TestTroublemaker_VictimsAreNotTold 被捣蛋鬼换过牌的两个人不知道。
func TestTroublemaker_VictimsAreNotTold(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("t", RoleTroublemaker), at("w", RoleWerewolf),
		at("v1", RoleVillager), at("v2", RoleVillager))

	g.advance(PhaseNightTroublemake)
	g.use("t", SkillMeddle, "w", "v1")
	g.advance(PhaseNightDrunk)

	if g.card("w") != RoleVillager || g.card("v1") != RoleWerewolf {
		t.Fatalf("两人的牌应当已经交换，实际 w=%v v1=%v", g.card("w"), g.card("v1"))
	}
	for _, id := range []string{"w", "v1", "t"} {
		if got := g.info(id)["learn.self"]; got != "" {
			t.Errorf("%s 不该知道自己现在拿的是什么，却看到 %q", id, got)
		}
	}
}
