package engine

import (
	"sync"
	"testing"
)

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
