package hiddenrole

import (
	"errors"
	"testing"
)

// provider_test.go 四个信息边界扩展点，装上去之后**真的被调用**。
//
// extpoint_test.go 验的是「八个扩展点都能用一个普通函数装上」——那是装配。
// 这一批验的是另一半：装上之后引擎真的会去问它们，而且答案真的到达玩家。
// 两者缺一不可，而后者此前没有：`TeammateFunc.Teammates`、
// `RoleInfoFunc.RoleInfo` 这些适配器方法的覆盖率是 0%——装得上，没人问。

// providerGame 装了全套信息边界的一局。
func providerGame(t *testing.T) *Engine {
	t.Helper()

	opts := append(withNoopResolvers(),
		WithTeammates(TeammateFunc(func(playerID string, view GameView) []string {
			if playerID != "w1" {
				return nil
			}
			return []string{"w2"} // 只有 w1 认得 w2，反过来不成立
		})),
		WithRoleInfo(roleWitch, RoleInfoFunc(func(playerID string, view GameView) map[string]string {
			return map[string]string{"probe.round": string(view.Phase())}
		})),
		WithSpeech(SpeechFunc(func(senderID string, view GameView) []string {
			if senderID == "wi" {
				return nil // 女巫说话没人听得见
			}
			return []string{"w1"}
		})),
		WithAudience(AudienceFunc(func(event *Event, view GameView) ([]string, bool) {
			if event.Type == EventType("PROBE") {
				return []string{"w1"}, true
			}
			return nil, false
		})),
	)

	e, err := NewEngine(testConfig(), opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "w2", roleWerewolf)
	mustAdd(t, e, "wi", roleWitch)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return e
}

// TestProviders_AreActuallyAsked 装上去的四个 provider 真的被问到，答案真的到达玩家。
func TestProviders_AreActuallyAsked(t *testing.T) {
	e := providerGame(t)

	t.Run("队友：允许不对称", func(t *testing.T) {
		if got := e.Teammates("w1"); len(got) != 1 || got[0] != "w2" {
			t.Errorf("w1 该认得 w2，实际 %v", got)
		}
		if got := e.Teammates("w2"); len(got) != 0 {
			t.Errorf("反过来不成立，实际 %v", got)
		}
		// 同一个 provider 也要喂到玩家视角里去，不能两条路两个答案。
		if got := e.PlayerView("w1").Teammates; len(got) != 1 || got[0] != "w2" {
			t.Errorf("PlayerView 里的队友与 Engine.Teammates 该一致，实际 %v", got)
		}
	})

	t.Run("角色专属信息：投射到玩家视角", func(t *testing.T) {
		got := e.PlayerView("wi").RoleInfo
		if got["probe.round"] == "" {
			t.Error("RoleInfoProvider 装上了却没被问到")
		}
		if len(e.PlayerView("w1").RoleInfo) != 0 {
			t.Error("只给女巫注册的信息不该出现在狼人视角里")
		}
	})

	t.Run("发言：provider 说了算", func(t *testing.T) {
		if got := e.MessageReceivers("w2"); len(got) != 1 || got[0] != "w1" {
			t.Errorf("provider 说只有 w1 听得见，实际 %v", got)
		}
		if got := e.MessageReceivers("wi"); len(got) != 0 {
			t.Errorf("provider 说女巫说话没人听得见，实际 %v", got)
		}
		// 没有接收者时发言被拒——这是内核唯一会拦发言的地方。
		if err := e.SendMessage("wi", "有人吗"); !errors.Is(err, ErrMessageNotAllowed) {
			t.Errorf("没有接收者该拒成 %v，实际 %v", ErrMessageNotAllowed, err)
		}
		if err := e.SendMessage("w2", "在"); err != nil {
			t.Errorf("有接收者就该发得出去，实际 %v", err)
		}
		if err := e.SendMessage("ghost", "?"); !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("不存在的玩家该拒成 %v，实际 %v", ErrPlayerNotFound, err)
		}
	})

	t.Run("受众：规则表态的听规则，没表态的答「不知道」", func(t *testing.T) {
		probe := NewEffect(EventType("PROBE"), "", "").ToEvent()
		got, known := e.AudienceOf(probe)
		if !known || len(got) != 1 || got[0] != "w1" {
			t.Errorf("provider 表过态，答案该是 [w1]，实际 %v（known=%v）", got, known)
		}
		other := NewEffect(EventType("NOT_DECLARED"), "", "").ToEvent()
		if _, known := e.AudienceOf(other); known {
			t.Error("provider 没表态的事件，答案该是「不知道」")
		}
	})
}

// TestGameView_ReadsThroughEverything 只读视图上的每一条读法都读得到真东西。
//
// 视图是 Resolver 看世界的**唯一**窗口——少一条读法读错，规则就得靠猜。
func TestGameView_ReadsThroughEverything(t *testing.T) {
	e := providerGame(t)
	e.Apply(NewSetAliveEffect("w2", false))
	view := e.View()

	if got := view.Phase(); got != e.Status().Phase {
		t.Errorf("Phase = %v，引擎说 %v", got, e.Status().Phase)
	}
	if got := view.Round(); got != e.Status().Round {
		t.Errorf("Round = %d，引擎说 %d", got, e.Status().Round)
	}
	if got := len(view.AllPlayers()); got != 3 {
		t.Errorf("AllPlayers 要含已出局的，实际 %d 人", got)
	}
	if got := len(view.AlivePlayers()); got != 2 {
		t.Errorf("AlivePlayers 只数活人，实际 %d 人", got)
	}
	if got := view.AlivePlayerIDsByRole(roleWerewolf); len(got) != 1 || got[0] != "w1" {
		t.Errorf("按角色数活人：期望 [w1]，实际 %v", got)
	}
	if p, ok := view.Player("w1"); !ok || p.Role != roleWerewolf {
		t.Errorf("Player 读错了：%+v", p)
	}
	if _, ok := view.Player("ghost"); ok {
		t.Error("不存在的玩家不该读得到")
	}
	if rc := view.RoundContext(); rc.Vars == nil && len(rc.Detours) != 0 {
		t.Errorf("回合上下文读错了：%+v", rc)
	}

	t.Run("视图是快照式的：之后改状态不影响它", func(t *testing.T) {
		before := len(view.AlivePlayers())
		e.Apply(NewSetAliveEffect("w1", false))
		if got := len(view.AlivePlayers()); got != before {
			t.Errorf("取过的视图不该跟着变：%d -> %d", before, got)
		}
	})
}

// TestWithLogger_IsActuallyWired 装上去的日志真的收得到东西。
func TestWithLogger_IsActuallyWired(t *testing.T) {
	rec := &recordingLogger{}
	e := newTestEngine(t, append(withNoopResolvers(), WithLogger(rec))...)
	mustAdd(t, e, "v", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if rec.infos == 0 {
		t.Error("开局该留下 Info 级日志")
	}

	t.Run("nil 不该让构造失败", func(t *testing.T) {
		if _, err := NewEngine(testConfig(), append(withNoopResolvers(), WithLogger(nil))...); err != nil {
			t.Errorf("nil 日志该被忽略，实际 %v", err)
		}
	})
}

// TestOptions_RejectNil 扩展点不接受 nil——装了个空的比不装更糟。
func TestOptions_RejectNil(t *testing.T) {
	cases := []struct {
		name string
		opt  EngineOption
	}{
		{"Resolver", WithResolver(phaseDay, nil)},
		{"VictoryChecker", WithVictoryChecker(nil)},
		{"Audience", WithAudience(nil)},
		{"Teammates", WithTeammates(nil)},
		{"Speech", WithSpeech(nil)},
		{"RoleInfo", WithRoleInfo(roleWitch, nil)},
		{"RoleSetup", WithRoleSetup(roleWitch, nil)},
		{"GameSetup", WithGameSetup(nil)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := NewEngine(testConfig(), append(withNoopResolvers(), c.opt)...); err == nil {
				t.Error("nil 扩展点该被拒——装了个空的比不装更糟")
			}
		})
	}
}
