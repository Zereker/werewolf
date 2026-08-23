// Package gamefuzz 随机对局 + 通用不变量，供每一套规则包复用。
//
// # 为什么要有这个包
//
// 逐条写的用例只能覆盖想得到的局面。随机对局反过来做：让引擎跑成千上万局，
// 每一步都核对那些「无论如何都必须成立」的性质。这个项目查出来的几个最难
// 复现的问题——回放分叉、快照漏字段、游戏结束时回合数多加一次——全都属于
// 「某个特定局面下才现形」，也全都是被这类不变量抓住的。
//
// 它此前只存在于狼人杀那一套里（根包的 fuzz_test.go），于是**内核的确定性、
// 快照往返、效果流回放只被三分之一的规则验证过**。而这三样恰恰是内核的
// 承重墙，不该只有一套规则替它们作证。
//
// # 为什么它是公开的
//
// 它是测试设施，位置与 net/http/httptest 相同：**给使用者用的测试支架，
// 不是被测对象。**
//
// 它此前叫 `internal/gamefuzz`，理由是「不该为一件测试用的东西给刚冻结的
// API 加名字」。那个位置在**引擎独立成一个 module** 之后不成立了——
// Go 的规则是 `internal/` 只能被同一个 module 里的代码 import，而规则包
// 届时在另一个 module 里，一行都用不上它。
//
// 公开了就该被冻结守着：`TestAPI_SurfaceIsPinned` 连这个子包一起钉住，
// 不然它会成为一个绕开纪律的后门。
//
// 对照：engine.Board / Seat / Mark 也是公开的测试 API，它们做的是同一件
// 事的另一半——那三个手工摆一个局面单测解析器，这一套跑成千上万局验
// 不变量。
//
// # 这里的不变量都不认识任何游戏
//
// 一条都不提「狼人」「任务」「中央牌」。每一条问的都是内核层面的事：
// 存了再读回来一样吗、回放走到同一个局面吗、说不能行动的人是不是真的
// 不能行动。规则包只负责摆局面与出招。
package enginetest

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	"github.com/Zereker/werewolf/engine"
)

// Seat 一名玩家的入座信息。
type Seat struct {
	ID   string
	Role engine.RoleType
}

// Game 一局随机对局要用的全部材料，由规则包提供。
type Game struct {
	// Config 阶段图。可以每局随机——随机配置比随机打法更能翻出问题，
	// 狼人杀那套查出的三条会改变对局结果的问题里有两条就出在自定义配置上。
	Config *engine.Config

	// Options 装配。必须与 Config 配套，且**恢复时要能原样再传一遍**
	// ——快照往返那条不变量就是这么验的。
	Options []engine.EngineOption

	// Seats 入座名单。
	Seats []Seat

	// Label 这一局的特征标签，用来盯住「随机化有没有退化」。
	// 某个分支永远走不到的话，这个测试会安静地变成只跑一种局面。
	Labels []string
}

// Setup 摆一局。同一个 rng 喂进去必须摆出同一局，失败才可复现。
type Setup func(rng *rand.Rand) Game

// Act 出一步招。规则包自己决定怎么出——泛泛地随机提交在多目标技能上
// 几乎必然被拒（任务制那一套的提名要 N 个人，一夜狼人的捣蛋鬼要正好 2 个），
// 那样对局推不到有意思的局面去。
//
// 允许什么都不做：不出招也能推进阶段，很多规则的夜晚能力本来就是可选的。
type Act func(e *engine.Engine, rng *rand.Rand)

// FuzzSpec 一次随机对局测试的参数。
type FuzzSpec struct {
	Games    int      // 跑多少局
	MaxSteps int      // 单局最多推进多少步，超了算没结束
	Setup    Setup    // 怎么摆局
	Act      Act      // 怎么出招；nil 表示不出招，只推进阶段
	WantEnd  bool     // 是否要求每一局都必须在 MaxSteps 内结束
	MustSee  []string // 这些标签一个都不能为零，否则说明随机化退化了
}

// RunFuzz 跑一批随机对局，逐步核对不变量。
//
// 种子固定，因此失败可复现：日志里带 seed 与 step。
func RunFuzz(t *testing.T, spec FuzzSpec) {
	t.Helper()

	stats := map[string]int{}
	for seed := 0; seed < spec.Games; seed++ {
		rng := rand.New(rand.NewSource(int64(seed))) //nolint:gosec // 测试用随机
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("seed=%d PANIC: %v", seed, r)
				}
			}()
			for _, label := range playOne(t, seed, rng, spec) {
				stats[label]++
			}
		}()
	}

	for _, k := range sortedKeys(stats) {
		t.Logf("  %-16s %d", k, stats[k])
	}
	if spec.WantEnd {
		if n := stats[labelNotEnded]; n > 0 {
			t.Errorf("有 %d 局在 %d 步内没有结束", n, spec.MaxSteps)
		}
	}
	for _, k := range spec.MustSee {
		if stats[k] == 0 {
			t.Errorf("随机化没有覆盖到「%s」，搜索空间退化了", k)
		}
	}
}

const (
	labelStarted  = "开局"
	labelEnded    = "结束"
	labelNotEnded = "未结束"
)

// playOne 跑一局，返回这一局的特征标签。
func playOne(t *testing.T, seed int, rng *rand.Rand, spec FuzzSpec) []string {
	t.Helper()

	g := spec.Setup(rng)
	e, err := engine.NewEngine(g.Config, g.Options...)
	if err != nil {
		t.Fatalf("seed=%d NewEngine: %v", seed, err)
	}
	for _, s := range g.Seats {
		if err := e.AddPlayer(s.ID, s.Role); err != nil {
			t.Fatalf("seed=%d AddPlayer(%s): %v", seed, s.ID, err)
		}
	}
	if err := e.Start(); err != nil {
		t.Fatalf("seed=%d Start: %v", seed, err)
	}

	labels := append([]string{labelStarted}, g.Labels...)

	// 内核发出来的每一条事件都收下——不变量 D 要看它们。
	var seen []engine.EventType
	e.OnEvent(func(ev *engine.Event) { seen = append(seen, ev.Type) })

	lastRound := e.Status().Round
	for step := 0; step < spec.MaxSteps; step++ {
		if e.Status().Over {
			checkEndedStaysEnded(t, seed, step, e, g)
			return append(labels, labelEnded)
		}

		if spec.Act != nil {
			spec.Act(e, rng)
		}

		checkAllowedMatchesView(t, seed, step, e)
		checkPhaseInfoStable(t, seed, step, e)

		clone := checkSnapshotRoundTrip(t, seed, step, e, g)
		checkSameBehaviour(t, seed, step, "快照往返", e, clone)

		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("seed=%d step=%d EndPhase: %v", seed, step, err)
		}
		if _, err := clone.EndPhase(); err != nil {
			t.Fatalf("seed=%d step=%d clone EndPhase: %v", seed, step, err)
		}
		checkSameState(t, seed, step, e, clone)

		checkStatusCoherent(t, seed, step, e)
		if r := e.Status().Round; r < lastRound {
			t.Fatalf("seed=%d step=%d 回合数倒退: %d -> %d", seed, step, lastRound, r)
		} else {
			lastRound = r
		}

		checkReplay(t, seed, step, e, g)
	}

	checkPrimitivesNeverBroadcast(t, seed, seen)
	return append(labels, labelNotEnded)
}

// checkSnapshotRoundTrip 不变量 A：存档往返之后，两边必须是同一个局面。
//
// 光比阶段与回合不够——快照漏掉一个字段，两边照样能同步地走完一整局，
// 只是规则判定不一样了。逐字节比对导出的快照才挡得住「漏字段」这一类，
// 而那一类真出过两次（守卫的连守记录、结束那一刻的赢家）。
func checkSnapshotRoundTrip(t *testing.T, seed, step int, e *engine.Engine, g Game) *engine.Engine {
	t.Helper()

	raw, err := json.Marshal(e.Snapshot())
	if err != nil {
		t.Fatalf("seed=%d step=%d Marshal: %v", seed, step, err)
	}
	var back engine.Snapshot
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("seed=%d step=%d Unmarshal: %v", seed, step, err)
	}
	clone, err := engine.RestoreEngine(g.Config, &back, g.Options...)
	if err != nil {
		t.Fatalf("seed=%d step=%d RestoreEngine: %v", seed, step, err)
	}
	return clone
}

// checkSameState 两台引擎导出的快照必须逐字节相同。
func checkSameState(t *testing.T, seed, step int, a, b *engine.Engine) {
	t.Helper()
	x, _ := json.Marshal(a.Snapshot())
	y, _ := json.Marshal(b.Snapshot())
	if string(x) != string(y) {
		t.Fatalf("seed=%d step=%d 快照往返后状态不一致:\n  原  %s\n  副本 %s", seed, step, x, y)
	}
	if a.Status() != b.Status() {
		t.Fatalf("seed=%d step=%d Status 不一致: %+v vs %+v", seed, step, a.Status(), b.Status())
	}
}

// checkSameBehaviour 两台引擎对「现在谁能干什么」必须给出同一个答案。
//
// **这一条是变异验证逼出来的。** 此前跨引擎只比快照字节——而快照序列化器
// 自己漏一个字段时，两边一起漏，比对是瞎的：「快照丢掉 Actors」这个变异
// 当场存活了。
//
// 快照的意义不是「导出的 JSON 一样」，是「恢复出来的引擎行为一样」。
// 所以要问行为：谁能行动、还差谁行动。行动者名单丢了，这里立刻就不一样。
func checkSameBehaviour(t *testing.T, seed, step int, how string, a, b *engine.Engine) {
	t.Helper()

	for _, p := range a.View().AllPlayers() {
		x := fmt.Sprint(a.AllowedSkills(p.ID))
		y := fmt.Sprint(b.AllowedSkills(p.ID))
		if x != y {
			t.Fatalf("seed=%d step=%d %s之后 %s 能做的事不一样: 原=%s 副本=%s",
				seed, step, how, p.ID, x, y)
		}
	}

	x, y := a.PhaseReadiness(), b.PhaseReadiness()
	if fmt.Sprint(x) != fmt.Sprint(y) {
		t.Fatalf("seed=%d step=%d %s之后就绪情况不一样:\n  原  %+v\n  副本 %+v",
			seed, step, how, x, y)
	}

	// 上帝视角的那份名单同样要一致——主持人照它组织流程。
	for role, ri := range a.PhaseInfo().RoleInfos {
		other, ok := b.PhaseInfo().RoleInfos[role]
		if !ok {
			t.Fatalf("seed=%d step=%d %s之后少了 %v 的信息", seed, step, how, role)
		}
		if fmt.Sprint(ri.PlayerIDs) != fmt.Sprint(other.PlayerIDs) {
			t.Fatalf("seed=%d step=%d %s之后 %v 该行动的人不一样: 原=%v 副本=%v",
				seed, step, how, role, ri.PlayerIDs, other.PlayerIDs)
		}
	}
}

// checkReplay 不变量 B：效果流回放出同一个局面。
//
// 与快照那条互补：快照是状态，效果流是历史。两者都能重建，重建结果必须一样。
func checkReplay(t *testing.T, seed, step int, e *engine.Engine, g Game) {
	t.Helper()

	replayed, err := engine.ReplayEngine(g.Config, e.EffectLog(), g.Options...)
	if err != nil {
		t.Fatalf("seed=%d step=%d ReplayEngine: %v", seed, step, err)
	}
	if got, want := replayed.Status().Phase, e.Status().Phase; got != want {
		t.Fatalf("seed=%d step=%d 回放后阶段 = %v，原局 %v", seed, step, got, want)
	}
	if got, want := replayed.Status().Round, e.Status().Round; got != want {
		t.Fatalf("seed=%d step=%d 回放后回合 = %d，原局 %d", seed, step, got, want)
	}

	// 逐字节比快照。这里能这么比，是因为 checkReplay 在 EndPhase **之后**
	// 调用——未结算的提交已经清空，而它们本来就不在效果流里
	//（还没变成效果）。
	x, _ := json.Marshal(e.Snapshot())
	y, _ := json.Marshal(replayed.Snapshot())
	if string(x) != string(y) {
		t.Fatalf("seed=%d step=%d 回放后状态不一致:\n  原  %s\n  回放 %s", seed, step, x, y)
	}

	// 与快照那条同一个道理：字节一样不等于行为一样，还要问行为。
	checkSameBehaviour(t, seed, step, "效果流回放", e, replayed)
}

// checkAllowedMatchesView 不变量 C：三条路对「谁能行动」必须给出同一个答案。
//
// Engine.AllowedSkills、PlayerView.AllowedSkills、以及 SubmitSkillUse 的校验。
// 三者答案不同的话，调用方按其中一个组织流程，玩家的提交会被另一个拒掉。
func checkAllowedMatchesView(t *testing.T, seed, step int, e *engine.Engine) {
	t.Helper()
	for _, p := range e.View().AllPlayers() {
		a := len(e.AllowedSkills(p.ID))
		v := e.PlayerView(p.ID)
		if v == nil {
			t.Fatalf("seed=%d step=%d %s 没有视角", seed, step, p.ID)
		}
		if b := len(v.AllowedSkills); a != b {
			t.Fatalf("seed=%d step=%d %s: AllowedSkills=%d PlayerView=%d", seed, step, p.ID, a, b)
		}
	}
}

// checkPhaseInfoStable 不变量 D：同一个局面反复查询，名单顺序必须稳定。
//
// 遍历 map 产出名单的话，同一个局面每次给出的顺序都不一样——效果流的
// 回放与比对就没了确定性。这条真出过一次。
func checkPhaseInfoStable(t *testing.T, seed, step int, e *engine.Engine) {
	t.Helper()
	want := map[engine.RoleType]string{}
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
}

// checkStatusCoherent 不变量 E：Status 的四项必须彼此自洽。
//
// 结束了就必须停在 PhaseEnd 且有赢家；没结束就不能已经有赢家。
// 反方向那一条（结束了却没有赢家）漏了很久——快照不带赢家，恢复出来的
// 对局就是那样。
func checkStatusCoherent(t *testing.T, seed, step int, e *engine.Engine) {
	t.Helper()
	st := e.Status()
	switch {
	case st.Over && st.Phase != engine.PhaseEnd:
		t.Fatalf("seed=%d step=%d 已结束却停在 %v", seed, step, st.Phase)
	case !st.Over && st.Winner != engine.CampUnspecified:
		t.Fatalf("seed=%d step=%d 没结束却已经有赢家 %v", seed, step, st.Winner)
	case st.Round < 1:
		t.Fatalf("seed=%d step=%d 回合数 %d 不合法", seed, step, st.Round)
	}
}

// checkEndedStaysEnded 不变量 F：结束之后局面不再变。
func checkEndedStaysEnded(t *testing.T, seed, step int, e *engine.Engine, g Game) {
	t.Helper()
	before, _ := json.Marshal(e.Snapshot())
	st := e.Status()

	_, _ = e.EndPhase() // 结束之后再推一步：报错或者什么都不做，都可以

	after, _ := json.Marshal(e.Snapshot())
	if string(before) != string(after) {
		t.Fatalf("seed=%d step=%d 结束之后局面还在变:\n  前 %s\n  后 %s", seed, step, before, after)
	}
	if e.Status() != st {
		t.Fatalf("seed=%d step=%d 结束之后 Status 还在变: %+v -> %+v", seed, step, st, e.Status())
	}

	// 结束的局面同样要能存档往返。
	clone := checkSnapshotRoundTrip(t, seed, step, e, g)
	checkSameState(t, seed, step, e, clone)
}

// checkPrimitivesNeverBroadcast 不变量 G：内核的状态原语一条都不该到达 OnEvent。
//
// 宿主原样转发 OnEvent 就把上帝视角发出去了。这一条内核不可配置，
// 但「不可配置」也要有东西验着。
func checkPrimitivesNeverBroadcast(t *testing.T, seed int, seen []engine.EventType) {
	t.Helper()
	primitives := map[engine.EventType]bool{
		engine.EventSetAlive: true, engine.EventSetVar: true,
		engine.EventSetActors: true, engine.EventDetour: true,
		engine.EventGotoPhase: true, engine.EventPlayerAdded: true,
		engine.EventPhaseChanged: true,
	}
	for _, typ := range seen {
		if primitives[typ] {
			t.Fatalf("seed=%d 状态原语 %v 出现在 OnEvent 里", seed, typ)
		}
	}
}

// sortedKeys 统计表的键，排过序——日志因此是确定的。
func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
