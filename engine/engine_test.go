package engine

import (
	"sync"
	"testing"
)

// TestStatus_SurvivesSnapshot 存档往返之后 Status 的四项仍然自洽。
//
// 这一条是补的：Status 号称四项来自同一个瞬间，而**恢复**这条路上它们
// 从来就对不上——快照不带赢家，一局已经结束的对局恢复出来是
// Over=true 而 Winner 为空。
//
// TestStatus_IsAtomic 只测了「没结束却有赢家」那个方向，反方向
// 「结束了却没有赢家」一直没人管，于是漏了很久。两个方向现在都有。
func TestStatus_SurvivesSnapshot(t *testing.T) {
	opts := append(withNoopResolvers(),
		WithVictoryChecker(VictoryFunc(func(view GameView) (bool, Camp) {
			return view.Round() > 1, Camp("PROBE")
		})))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "p1", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	for i := 0; i < 30 && !e.Status().Over; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
	}

	before := e.Status()
	if !before.Over || before.Winner == CampUnspecified {
		t.Fatalf("这个测试要一局**已经结束且有赢家**的对局，实际 %+v", before)
	}

	restored, err := RestoreEngine(testConfig(), e.Snapshot(), opts...)
	if err != nil {
		t.Fatalf("RestoreEngine: %v", err)
	}
	if got := restored.Status(); got != before {
		t.Errorf("存档往返之后 Status 变了：%+v -> %+v", before, got)
	}
}

// TestStatus_IsAtomic Status 的四项必须来自同一个瞬间。
//
// 这是 Phase / Round / IsGameOver / Winner 合成一个方法的**唯一**理由：
// 四个方法各取一次读锁，宿主要渲染「第 3 回合的白天」得连问两次，中间
// 另一个 goroutine 结算掉一个阶段的话，读到的是一组从来不曾同时成立的值。
//
// 这里一边不停推进阶段，一边不停读 Status，断言读到的组合永远是合法的：
// 结束了就必须停在 PhaseEnd，没结束就不能已经有赢家。
func TestStatus_IsAtomic(t *testing.T) {
	e := newTestEngine(t, append(withNoopResolvers(),
		WithVictoryChecker(VictoryFunc(func(view GameView) (bool, Camp) {
			// 跑够几个回合再结束，好让读者有足够多的机会撞上中间态。
			return view.Round() > 3, Camp("PROBE")
		})))...)
	mustAdd(t, e, "p1", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	done := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		for i := 0; i < 200; i++ {
			if _, err := e.EndPhase(); err != nil {
				return
			}
		}
	}()

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				st := e.Status()
				if st.Over && st.Winner == CampUnspecified {
					t.Error("读到「已结束」却没有赢家——四项来自不同的瞬间")
					return
				}
				if st.Over && st.Phase != PhaseEnd {
					t.Errorf("读到「已结束」但阶段是 %v——四项来自不同的瞬间", st.Phase)
					return
				}
				if !st.Over && st.Winner != CampUnspecified {
					t.Errorf("读到「没结束」却已经有赢家 %v——四项来自不同的瞬间", st.Winner)
					return
				}
				if st.Round < 1 {
					t.Errorf("回合数 %d 不合法", st.Round)
					return
				}
			}
		}()
	}
	wg.Wait()
}
