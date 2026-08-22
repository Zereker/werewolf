package engine

import (
	"fmt"
	"testing"
)

// diceRoller 一个真的用随机数的解析器：给每个活着的玩家摇一个点数
type diceRoller struct{}

func (diceRoller) Resolve(_ []*SkillUse, view GameView) []*Effect {
	rng := view.Rand()
	var out []*Effect
	for _, p := range view.AllPlayers() {
		out = append(out, NewSetPlayerVarEffect(p.ID, "dice", fmt.Sprint(rng.Intn(6)+1)))
	}
	return out
}

func rollOnce(t *testing.T, seed int64) map[string]string {
	t.Helper()
	cfg := testConfig()
	cfg.Seed = seed
	opts := append(withNoopResolvers(), WithResolver(phaseNightGuard, diceRoller{}))
	e, err := NewEngine(cfg, opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	mustAdd(t, e, "a", roleWerewolf)
	mustAdd(t, e, "b", roleGuard)
	mustAdd(t, e, "c", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}
	out := map[string]string{}
	for _, id := range []string{"a", "b", "c"} {
		p, _ := e.PlayerInfo(id)
		out[id] = p.Vars["dice"]
	}
	return out
}

// TestRand_SameSeedSameResult 同一个种子摇出同一串数。
func TestRand_SameSeedSameResult(t *testing.T) {
	first := rollOnce(t, 42)
	for i := 0; i < 10; i++ {
		if got := rollOnce(t, 42); fmt.Sprint(got) != fmt.Sprint(first) {
			t.Fatalf("第 %d 次结果不同：%v vs %v", i+1, first, got)
		}
	}
	if first["a"] == "" {
		t.Fatal("一个点数都没摇出来，这个测试什么都没验到")
	}
}

// TestRand_DifferentSeedDifferentResult 换个种子结果就该不同。
//
// 不然「可以每局不同」这句话是假的——随机流没真的用上种子。
func TestRand_DifferentSeedDifferentResult(t *testing.T) {
	seen := map[string]bool{}
	for seed := int64(1); seed <= 20; seed++ {
		seen[fmt.Sprint(rollOnce(t, seed))] = true
	}
	if len(seen) < 5 {
		t.Errorf("20 个种子只摇出 %d 种结果——种子多半没起作用", len(seen))
	}
}

// TestRand_SurvivesReplayAndSnapshot 摇过随机数的对局，回放与恢复都能重现。
//
// 这是这条能力存在的全部意义。此前内核没有随机，规则要摇只能让宿主在引擎
// 外面摇——摇出来的结果不进效果流，那一部分**回放不出来**，而可回放是这个
// 库的招牌之一。
//
// 我们没有像对照实现那样把 PRNG 的内部状态存进游戏状态。不需要：结算是局面的
// 纯函数，随机流由 (种子, 回合, 阶段) 唯一决定，回放走到同一个局面自然摇出
// 同一串数。种子随快照走，因此恢复出来的对局也一样。
func TestRand_SurvivesReplayAndSnapshot(t *testing.T) {
	cfg := testConfig()
	cfg.Seed = 7
	opts := append(withNoopResolvers(), WithResolver(phaseNightGuard, diceRoller{}))
	e, err := NewEngine(cfg, opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	mustAdd(t, e, "a", roleWerewolf)
	mustAdd(t, e, "b", roleGuard)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}
	want := diceOf(t, e)

	replayed, err := ReplayEngine(cfg, e.EffectLog(), opts...)
	if err != nil {
		t.Fatalf("ReplayEngine: %v", err)
	}
	if got := diceOf(t, replayed); got != want {
		t.Errorf("回放摇出的点数不同：%s vs %s", want, got)
	}

	restored, err := RestoreEngine(cfg, e.Snapshot(), opts...)
	if err != nil {
		t.Fatalf("RestoreEngine: %v", err)
	}
	if got := diceOf(t, restored); got != want {
		t.Errorf("恢复后摇出的点数不同：%s vs %s", want, got)
	}
	if restored.Snapshot().Seed != 7 {
		t.Errorf("种子没随快照走：%d", restored.Snapshot().Seed)
	}
}

func diceOf(t *testing.T, e *Engine) string {
	t.Helper()
	out := ""
	for _, id := range []string{"a", "b"} {
		p, _ := e.PlayerInfo(id)
		out += id + "=" + p.Vars["dice"] + " "
	}
	return out
}

// TestRand_StreamVariesByRoundAndPhase 不同回合、不同阶段拿到的是不同的流。
//
// 这条是变异验证逼出来的：把回合与阶段从流的推导里拿掉之后，整套测试一条
// 都不红——而后果相当严重：整局每个阶段摇出的都是同一串数，同一个玩家会
// 每回合掷出同一个点。「随机」名存实亡。
//
// 这是第三次遇到「写了理由却没东西守着」。做法同前两次。
func TestRand_StreamVariesByRoundAndPhase(t *testing.T) {
	const seed = 99
	draw := func(round int, phase PhaseType) int {
		return randStream(seed, round, phase).Intn(1 << 30)
	}

	// 同一个 (回合, 阶段) 必须稳定
	if a, b := draw(1, phaseNightGuard), draw(1, phaseNightGuard); a != b {
		t.Fatalf("同一个局面摇出了不同的数：%d vs %d", a, b)
	}

	// 换回合、换阶段都该换一条流
	base := draw(1, phaseNightGuard)
	if got := draw(2, phaseNightGuard); got == base {
		t.Error("换了回合还是同一条流——整局每回合会摇出同样的结果")
	}
	if got := draw(1, phaseNightWolf); got == base {
		t.Error("换了阶段还是同一条流——一个回合内每个阶段会摇出同样的结果")
	}

	// 阶段名是字符串，混种子时不能简单相加：不同阶段不该撞在一起
	seen := map[int]bool{}
	for _, p := range []PhaseType{
		phaseNightGuard, phaseNightWolf, phaseNightWitch,
		phaseNightSeer, phaseDay, phaseVote,
	} {
		seen[draw(1, p)] = true
	}
	if len(seen) != 6 {
		t.Errorf("六个阶段只摇出 %d 条不同的流，有阶段撞在一起了", len(seen))
	}
}
