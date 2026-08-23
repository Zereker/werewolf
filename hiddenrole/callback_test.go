package hiddenrole

import (
	"testing"
	"time"
)

// TestCallbacks_MayCallBackIntoTheEngine OnEvent / OnMessage 的处理器可以回调 Engine。
//
// 这是**被支持的**写法，也是把引擎接进一个服务端的唯一办法：收到事件，
// 问一句「这该发给谁」，再往那几条连接上写。example/netserver 整个推送
// 链路就压在这条性质上。
//
// 它成立的原因是事件与消息都在**锁外**发布（见 endPhaseInternal 与
// SendMessage）。这件事此前只写在代码注释里：谁要是把 dispatchEvent
// 挪回锁内，netserver 会当场死锁，而整套测试一条都不会红。
//
// 与之相对的是八个扩展点（Resolver、VictoryChecker、三个信息边界
// provider、RoleInfoProvider、RoleSetup）——它们在持锁期间被调用，
// 回调 Engine 的后果是**挂住，不是报错**。那条禁令写在各自的接口文档上。
//
// 超时兜底是必需的：真出问题时这个测试会挂住而不是失败，一个卡死的
// CI 比一条红线难查得多。
func TestCallbacks_MayCallBackIntoTheEngine(t *testing.T) {
	var events, messages int

	done := make(chan struct{})
	go func() {
		defer close(done)

		opts := append(withNoopResolvers(),
			WithResolver(phaseNightGuard, effectProducer{tag: "回调"}),
			WithAudience(AudienceFunc(func(*Event, GameView) ([]string, bool) {
				return []string{"w1"}, true
			})),
			WithSpeech(SpeechFunc(func(string, GameView) []string {
				return []string{"w1", "g"}
			})))
		e := newTestEngine(t, opts...)
		mustAdd(t, e, "w1", roleWerewolf)
		mustAdd(t, e, "g", roleGuard)

		// 处理器里把公开的读法挨个调一遍
		e.OnEvent(func(ev *Event) {
			events++
			_, _ = e.AudienceOf(ev)
			_ = e.Status().Phase
			_ = e.Status().Round
			_ = e.PlayerView("w1")
			_, _ = e.PlayerInfo("w1")
			_ = e.PhaseReadiness()
			_ = e.EffectLog()
			_ = e.View()
			_ = e.Teammates("w1")
			_ = e.Snapshot()
		})
		e.OnMessage(func(*Message, []string) {
			messages++
			_ = e.MessageReceivers("w1")
			_ = e.AllowedSkills("w1")
			_ = e.PhaseInfo()
		})

		if err := e.Start(); err != nil {
			t.Errorf("Start: %v", err)
			return
		}
		if _, err := e.EndPhase(); err != nil {
			t.Errorf("EndPhase: %v", err)
			return
		}
		if err := e.SendMessage("w1", "还活着吗"); err != nil {
			t.Errorf("SendMessage: %v", err)
			return
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("处理器回调 Engine 时死锁了——事件与消息必须在锁外发布，" +
			"检查 endPhaseInternal 与 SendMessage 的发布位置是不是被挪进了锁内")
	}

	if events == 0 {
		t.Error("OnEvent 处理器一次都没被调用——这个测试什么都没验到")
	}
	if messages == 0 {
		t.Error("OnMessage 处理器一次都没被调用——这个测试什么都没验到")
	}
}
