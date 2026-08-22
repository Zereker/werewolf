package werewolf

import "testing"

// 这几个基准不是为了调优，而是为了让「性能怎么样」这个问题有据可依。
//
// 此前一个基准都没有：EndPhase 有没有意外的 O(n²)、每阶段分配多少内存、
// 面向玩家的视图查一次多贵，全都答不上来。答不上来本身就是缺口——
// 一次不小心的重构把某处从线性变成平方，谁也不会发现。
//
// 跑法：go test -bench=. -benchmem

// benchBoard 一副 12 人的标准板子。
func benchBoard() []RoleType {
	return []RoleType{
		RoleWerewolf, RoleWerewolf, RoleWerewolf, RoleWerewolf,
		RoleSeer, RoleWitch, RoleGuard, RoleHunter,
		RoleVillager, RoleVillager, RoleVillager, RoleVillager,
	}
}

// benchEngine 建一局并推进到指定阶段。
func benchEngine(b *testing.B, until PhaseType) *Engine {
	b.Helper()

	e := MustNewEngine(nil)
	for i, r := range benchBoard() {
		if err := e.AddPlayer(playerID(i), r); err != nil {
			b.Fatal(err)
		}
	}
	if err := e.Start(); err != nil {
		b.Fatal(err)
	}
	for e.Phase() != until {
		if _, err := e.EndPhase(); err != nil {
			b.Fatal(err)
		}
	}
	return e
}

func playerID(i int) string {
	return string(rune('a'+i/10)) + string(rune('0'+i%10))
}

// BenchmarkFullGame 一整局：建局、发牌、打到分出胜负。
// 这是使用者最关心的那个数字——托管一局游戏的总成本。
func BenchmarkFullGame(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		e := MustNewEngine(nil)
		for j, r := range benchBoard() {
			if err := e.AddPlayer(playerID(j), r); err != nil {
				b.Fatal(err)
			}
		}
		if err := e.Start(); err != nil {
			b.Fatal(err)
		}
		alive := e.AlivePlayerIDs()
		for steps := 0; steps < 400 && !e.IsGameOver(); steps++ {
			for _, p := range e.PhaseReadiness().Pending {
				_ = e.SubmitSkillUse(&SkillUse{
					PlayerID: p.PlayerID, Skill: p.Skill, TargetID: alive[0],
				})
			}
			if _, err := e.EndPhase(); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkEndPhase 单个阶段的结算成本：狼人商刀，四只狼各投一票。
func BenchmarkEndPhase(b *testing.B) {
	b.ReportAllocs()
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		e := benchEngine(b, PhaseNightWolf)
		for j := 0; j < 4; j++ {
			if err := e.SubmitSkillUse(&SkillUse{
				PlayerID: playerID(j), Skill: SkillKill, TargetID: playerID(8),
			}); err != nil {
				b.Fatal(err)
			}
		}
		b.StartTimer()
		if _, err := e.EndPhase(); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
	}
}

// BenchmarkPlayerView 面向玩家的视图。服务端每次广播都要给每个人算一份，
// 是这个库被调用得最频繁的方法。
func BenchmarkPlayerView(b *testing.B) {
	e := benchEngine(b, PhaseNightWitch)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if v := e.PlayerView("a5"); v == nil {
			b.Fatal("视图不该为 nil")
		}
	}
}

// BenchmarkPhaseInfo 上帝视角，主持人每个阶段查一次。
func BenchmarkPhaseInfo(b *testing.B) {
	e := benchEngine(b, PhaseNightWolf)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = e.PhaseInfo()
	}
}

// BenchmarkSnapshot 导出局面。存档频率取决于使用者，但它是深拷贝，
// 值得知道一次多贵。
func BenchmarkSnapshot(b *testing.B) {
	e := benchEngine(b, PhaseNightWitch)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = e.Snapshot()
	}
}
