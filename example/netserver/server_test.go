package main

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/Zereker/werewolf"
)

// 这些测试是这个示例存在的理由。命令行主持台碰不到的那半边——
// 事件推送、每条连接一份视图、并发、断线重连、超时真的触发——
// 都在这里压。

// testClient 一条测试用的客户端连接。
type testClient struct {
	t   *testing.T
	raw net.Conn
	in  *bufio.Scanner
}

func dial(t *testing.T, addr, player string) *testClient {
	t.Helper()
	raw, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })

	sc := bufio.NewScanner(raw)
	sc.Buffer(make([]byte, 0, 4096), maxLine)
	c := &testClient{t: t, raw: raw, in: sc}
	if player != "" {
		c.send(clientMsg{Type: "join", Player: player})
	}
	return c
}

func (c *testClient) send(m clientMsg) {
	c.t.Helper()
	if err := json.NewEncoder(c.raw).Encode(m); err != nil {
		c.t.Fatalf("发送失败: %v", err)
	}
}

// await 等一条满足条件的报文，超时即失败。
func (c *testClient) await(what string, ok func(serverMsg) bool) serverMsg {
	c.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if err := c.raw.SetReadDeadline(deadline); err != nil {
			c.t.Fatalf("设置超时失败: %v", err)
		}
		if !c.in.Scan() {
			c.t.Fatalf("等 %s 时连接就断了: %v", what, c.in.Err())
		}
		var m serverMsg
		if err := json.Unmarshal(c.in.Bytes(), &m); err != nil {
			c.t.Fatalf("收到非法 JSON: %s", c.in.Bytes())
		}
		if ok(m) {
			return m
		}
		if time.Now().After(deadline) {
			c.t.Fatalf("等 %s 超时", what)
		}
	}
}

func (c *testClient) awaitView() *werewolf.PlayerView {
	c.t.Helper()
	return c.await("view", func(m serverMsg) bool { return m.Type == "view" }).View
}

// newTestServer 起一个服务端。tick 给得很小，让超时推进在测试里跑得快。
func newTestServer(t *testing.T, tick time.Duration) *server {
	t.Helper()
	srv, err := newServer("127.0.0.1:0", tick)
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	go srv.serve()
	t.Cleanup(srv.close)
	return srv
}

// TestServer_PushesViewOnJoin 连上来就该收到自己的阶段与视图。
//
// 这条顺带验证了整个库最主要、却一直没有真实使用者的那条通道：
// 服务端是「推」的，不是等谁来查。
func TestServer_PushesViewOnJoin(t *testing.T) {
	srv := newTestServer(t, time.Hour) // 不让它自己超时推进

	c := dial(t, srv.addr(), "p1")
	phase := c.await("phase", func(m serverMsg) bool { return m.Type == "phase" })
	if phase.Phase != werewolf.PhaseNightGuard.String() || phase.Round != 1 {
		t.Errorf("首个阶段应当是第 1 回合的守卫阶段，实际 %s/%d", phase.Phase, phase.Round)
	}
	if phase.Deadline == 0 {
		t.Error("应当带上本阶段的截止时间")
	}

	view := c.awaitView()
	if view == nil || view.Self.ID != "p1" {
		t.Fatalf("应当收到 p1 自己的视图，实际 %+v", view)
	}
}

// TestServer_EachConnectionGetsItsOwnView 每条连接拿到的必须是他自己那一份。
//
// 这是命令行主持台碰不到的：它是一个人按需 view <player>，
// 而服务端要同时对着九条连接，每条推的内容都不一样。
func TestServer_EachConnectionGetsItsOwnView(t *testing.T) {
	srv := newTestServer(t, time.Hour)

	views := make(map[string]*werewolf.PlayerView)
	for _, id := range srv.room.seats {
		c := dial(t, srv.addr(), id)
		views[id] = c.awaitView()
	}

	for id, v := range views {
		if v == nil || v.Self.ID != id {
			t.Fatalf("%s 拿到的视图不是自己的: %+v", id, v)
		}
		// 除了自己和狼队友，谁的身份都不该出现
		for _, p := range v.Players {
			if p.ID == id || p.Role == werewolf.RoleUnspecified {
				continue
			}
			if v.Self.Camp != werewolf.CampEvil {
				t.Errorf("%s（%v）看到了 %s 的身份", id, v.Self.Camp, p.ID)
			}
		}
	}
}

// TestServer_EventsGoOnlyToTheirAudience 私密事件不能推给不该看的人。
//
// 预言家查验的结果只有他自己知道。这条走的是完整的推送链路：
// EndPhase -> OnEvent -> AudienceOf -> 只写那几条连接。
func TestServer_EventsGoOnlyToTheirAudience(t *testing.T) {
	srv := newTestServer(t, time.Hour)

	clients := make(map[string]*testClient)
	for _, id := range srv.room.seats {
		clients[id] = dial(t, srv.addr(), id)
		clients[id].awaitView()
	}

	seer := seatOf(t, srv, werewolf.RoleSeer)
	other := seatOf(t, srv, werewolf.RoleVillager)

	// 推到预言家阶段
	for srv.room.engine.Phase() != werewolf.PhaseNightSeer {
		srv.room.cmds <- command{kind: "view", player: seer} // 借道排队，确保串行
		advance(t, srv)
	}

	clients[seer].send(clientMsg{Type: "act", Skill: "check", Target: other})

	ev := clients[seer].await("check 事件", func(m serverMsg) bool {
		return m.Type == "event" && m.Event.Type == werewolf.EventCheck
	})
	if ev.Event.SourceID != seer {
		t.Errorf("查验事件的来源应当是预言家，实际 %s", ev.Event.SourceID)
	}

	// 别人这条连接上不该出现查验事件。给一小段时间让错误的推送有机会到达。
	assertNoEvent(t, clients[other], werewolf.EventCheck)
}

// TestServer_ChatIsRoutedByPhase 狼人夜里的发言只有狼队听得到。
func TestServer_ChatIsRoutedByPhase(t *testing.T) {
	srv := newTestServer(t, time.Hour)

	clients := make(map[string]*testClient)
	for _, id := range srv.room.seats {
		clients[id] = dial(t, srv.addr(), id)
		clients[id].awaitView()
	}

	for srv.room.engine.Phase() != werewolf.PhaseNightWolf {
		advance(t, srv)
	}

	wolf := seatOf(t, srv, werewolf.RoleWerewolf)
	good := seatOf(t, srv, werewolf.RoleSeer)

	clients[wolf].send(clientMsg{Type: "say", Text: "刀预言家"})

	got := clients[wolf].await("chat", func(m serverMsg) bool { return m.Type == "chat" })
	if got.From != wolf || got.Text != "刀预言家" {
		t.Errorf("狼人应当听到自己的发言，实际 %+v", got)
	}
	assertNoChat(t, clients[good])
}

// TestServer_Reconnect 断线重连之后拿回完整局面。
//
// 引擎完全不知道有连接这回事——局面一直在它那里，重连只是重新算一份
// PlayerView 推过去。这条测的是「引擎不管连接」这个边界是不是真的成立。
func TestServer_Reconnect(t *testing.T) {
	srv := newTestServer(t, time.Hour)

	c := dial(t, srv.addr(), "p1")
	before := c.awaitView()

	// 推进两个阶段，期间这条连接是断的
	_ = c.raw.Close()
	advance(t, srv)
	advance(t, srv)

	again := dial(t, srv.addr(), "p1")
	phase := again.await("phase", func(m serverMsg) bool { return m.Type == "phase" })
	after := again.awaitView()

	if after == nil || after.Self.ID != "p1" {
		t.Fatalf("重连后应当拿到 p1 的视图，实际 %+v", after)
	}
	if after.Self.Role != before.Self.Role {
		t.Errorf("重连后身份变了: %v -> %v", before.Self.Role, after.Self.Role)
	}
	if phase.Phase == werewolf.PhaseNightGuard.String() {
		t.Error("推进过两个阶段，重连拿到的不该还是守卫阶段")
	}
}

// TestServer_DeadlineAdvancesPhase 超时真的会把阶段推走。
//
// 引擎不计时，只给建议值；「到点了怎么办」是使用者的决定。
// 命令行主持台只把剩余时间打印出来，从不真的因超时推进——这条补上。
func TestServer_DeadlineAdvancesPhase(t *testing.T) {
	srv := newTestServer(t, 5*time.Millisecond)

	c := dial(t, srv.addr(), "p1")
	c.awaitView()

	// 把截止时间提前，让它立刻到点。改 deadline 也要走房间的 goroutine，
	// 否则就是从测试线程直接改房间状态，-race 会报。
	srv.room.do(command{kind: "expire"})

	got := c.await("阶段被超时推走", func(m serverMsg) bool {
		return m.Type == "phase" && m.Phase != werewolf.PhaseNightGuard.String()
	})
	t.Logf("超时后进入 %s", got.Phase)
}

// TestServer_ConcurrentClients 九条连接同时读写，-race 下不许有竞态。
//
// 引擎的并发保证是「所有导出方法都可以并发调用」，但真正压它的场景
// 此前一个都没有：单元测试是顺序的，主持台是单线程的。
func TestServer_ConcurrentClients(t *testing.T) {
	srv := newTestServer(t, 10*time.Millisecond)

	var wg sync.WaitGroup
	for _, id := range srv.room.seats {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			c := dial(t, srv.addr(), id)
			for i := 0; i < 20; i++ {
				c.send(clientMsg{Type: "view"})
				c.send(clientMsg{Type: "say", Text: "hi"})
				c.send(clientMsg{Type: "act", Skill: "vote", Target: "p1"})
			}
			// 把这条连接上堆积的报文读掉，避免写端阻塞
			_ = c.raw.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			for c.in.Scan() {
			}
		}(id)
	}
	wg.Wait()
}

// ==================== 辅助 ====================

// advance 让房间推进一个阶段，并等它真的走完。
func advance(t *testing.T, srv *server) {
	t.Helper()
	before := srv.room.engine.Phase()
	srv.room.do(command{kind: "advance"})
	if got := srv.room.engine.Phase(); got == before {
		t.Fatalf("阶段没有推进，仍在 %v", before)
	}
}

func seatOf(t *testing.T, srv *server, role werewolf.RoleType) string {
	t.Helper()
	for _, id := range srv.room.seats {
		if p, ok := srv.room.engine.PlayerInfo(id); ok && p.Role == role {
			return id
		}
	}
	t.Fatalf("板子里没有 %v", role)
	return ""
}

func assertNoEvent(t *testing.T, c *testClient, typ werewolf.EventType) {
	t.Helper()
	_ = c.raw.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	for c.in.Scan() {
		var m serverMsg
		if json.Unmarshal(c.in.Bytes(), &m) == nil &&
			m.Type == "event" && m.Event.Type == typ {
			t.Fatalf("不该收到 %v 事件: %+v", typ, m.Event)
		}
	}
}

func assertNoChat(t *testing.T, c *testClient) {
	t.Helper()
	_ = c.raw.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	for c.in.Scan() {
		var m serverMsg
		if json.Unmarshal(c.in.Bytes(), &m) == nil && m.Type == "chat" {
			t.Fatalf("不该收到发言: %+v", m)
		}
	}
}
