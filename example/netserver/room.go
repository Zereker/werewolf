// room.go 一个房间：一局游戏，若干条连接，一个计时器。
//
// # 为什么用单 goroutine 串行化，而不是加锁
//
// 引擎的回调（OnEvent / OnMessage）是在 EndPhase 内部、释放引擎锁之后
// 同步触发的。如果房间自己也加一把锁，就会出现这样的顺序：
//
//	某个 goroutine: 持房间锁 -> EndPhase -> 回调 -> 想再拿房间锁  ✗ 自锁
//
// 换成 actor：所有对引擎的访问都发生在 run 这一个 goroutine 上，命令从
// channel 进来；回调也在这条 goroutine 上触发，它只往各连接的发送队列里
// 塞东西，不回头拿任何锁。既没有锁序问题，也天然满足了引擎
// 「回调在锁外执行，里面可以安全调用 Engine」的前提。

package main

import (
	"fmt"
	"log"
	"time"

	"github.com/Zereker/hiddenrole"
	"github.com/Zereker/werewolf"
)

// command 房间收到的一条指令。
type command struct {
	kind   string // attach / detach / act / say / view / advance / expire
	player string
	conn   *conn
	skill  werewolf.SkillType
	target string
	text   string

	// ack 非 nil 时，房间处理完这条命令就关掉它。
	// 让调用方能等到「这条命令真的执行完了」，而不是只知道它进了队列。
	ack chan struct{}
}

// room 一局游戏。
type room struct {
	name  string
	eng   *werewolf.Engine
	seats []string

	// conns 玩家 ID -> 他当前的连接。
	//
	// 断线就从这里摘掉，重连再放回来，引擎那边毫无感知——
	// 局面一直在引擎里，重连只需要重新算一份 PlayerView 推过去。
	conns map[string]*conn

	cmds chan command

	// deadline 本阶段的截止时间。引擎不计时，只通过 PhaseTimeout 给建议值，
	// 真正决定什么时候结束的是这里。
	deadline time.Time
	over     bool
}

// board 默认 9 人局：3 狼 + 预言家/女巫/守卫 + 3 民。
var board = []werewolf.RoleType{
	werewolf.RoleWerewolf, werewolf.RoleWerewolf, werewolf.RoleWerewolf,
	werewolf.RoleSeer, werewolf.RoleWitch, werewolf.RoleGuard,
	werewolf.RoleVillager, werewolf.RoleVillager, werewolf.RoleVillager,
}

func newRoom(name string, tick time.Duration) (*room, error) {
	r := &room{
		name:  name,
		conns: make(map[string]*conn),
		cmds:  make(chan command, 64),
	}

	eng, err := werewolf.New(werewolf.DefaultRules(), hiddenrole.WithLogger(roomLogger{room: name}))
	if err != nil {
		return nil, err
	}
	r.eng = eng

	for i, role := range board {
		id := fmt.Sprintf("p%d", i+1)
		if err := eng.AddPlayer(id, role); err != nil {
			return nil, err
		}
		r.seats = append(r.seats, id)
	}

	// 事件与消息都走推送。这是服务端与命令行主持台最大的区别：
	// 主持台是回合制地「拉」（看 EndPhase 的返回值），
	// 服务端必须「推」，而推给谁由 AudienceOf 决定。
	eng.OnEvent(r.onEvent)
	eng.OnMessage(r.onMessage)

	if err := eng.Start(); err != nil {
		return nil, err
	}
	r.armDeadline()

	go r.run(tick)
	return r, nil
}

// run 房间的唯一一条 goroutine。对引擎的所有访问都在这里。
//
// 游戏结束后不退出：连接还在，客户端仍可以查看最终局面，
// 退出会让 cmds 无人消费，发送方一路堵到队列满。
func (r *room) run(tick time.Duration) {
	t := time.NewTicker(tick)
	defer t.Stop()

	for {
		select {
		case c := <-r.cmds:
			r.handle(c)
		case <-t.C:
			r.checkDeadline()
		}
	}
}

func (r *room) handle(c command) {
	if c.ack != nil {
		defer close(c.ack)
	}
	switch c.kind {
	case "attach":
		r.attach(c.player, c.conn)
	case "detach":
		r.detach(c.player, c.conn)
	case "act":
		r.act(c.player, c.skill, c.target)
	case "say":
		r.say(c.player, c.text)
	case "view":
		r.pushView(c.player)
	case "expire":
		// 仅供测试：把本阶段的截止时间提前到过去，让计时器立刻到点。
		// 放在房间的 goroutine 上改，才不会与 run 抢同一个字段。
		r.deadline = time.Now().Add(-time.Second)
	case "advance":
		// 主持人强行推进，不等超时也不等所有人行动。
		// 引擎不会因为未就绪而拒绝——是否等下去本来就是调用方的判断。
		r.advance()
	}
}

// do 投递一条命令并等它执行完。
func (r *room) do(c command) {
	c.ack = make(chan struct{})
	r.cmds <- c
	<-c.ack
}

// attach 一条连接入座——首次进来与断线重连走的是同一条路。
func (r *room) attach(player string, c *conn) {
	if old, ok := r.conns[player]; ok && old != c {
		// 同一个玩家又连了一条：踢掉旧的
		old.send(serverMsg{Type: "error", Message: "该玩家在别处重新接入"})
		old.close()
	}
	r.conns[player] = c

	r.sendTo(player, r.phaseMsg())
	r.pushView(player)
}

func (r *room) detach(player string, c *conn) {
	if cur, ok := r.conns[player]; ok && cur == c {
		delete(r.conns, player)
	}
}

func (r *room) act(player string, skill werewolf.SkillType, target string) {
	err := r.eng.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: player, Skill: skill, Targets: []string{target},
	})
	if err != nil {
		r.sendTo(player, serverMsg{Type: "error", Message: err.Error()})
		return
	}
	// 提交之后自己的视图会变（可用技能少了），推一份新的
	r.pushView(player)

	// 必需行动都齐了就立刻推进，不必干等到超时
	if r.eng.PhaseReadiness().Ready {
		r.advance()
	}
}

func (r *room) say(player, text string) {
	if err := r.eng.SendMessage(player, text); err != nil {
		r.sendTo(player, serverMsg{Type: "error", Message: err.Error()})
	}
}

func (r *room) checkDeadline() {
	if r.over || r.deadline.IsZero() || time.Now().Before(r.deadline) {
		return
	}
	r.advance()
}

func (r *room) advance() {
	if r.over {
		return
	}
	if _, err := r.eng.EndPhase(); err != nil {
		log.Printf("[%s] 推进失败: %v", r.name, err)
		return
	}
	r.armDeadline()

	// 阶段变了，每个人的视图都可能跟着变（轮到谁行动、女巫还看不看得到刀口）
	msg := r.phaseMsg()
	for _, id := range r.seats {
		r.sendTo(id, msg)
		r.pushView(id)
	}
}

func (r *room) armDeadline() {
	if r.eng.Status().Over {
		r.over = true
		r.deadline = time.Time{}
		return
	}
	r.deadline = time.Now().Add(werewolf.DefaultGameConfig().PhaseTimeout(r.eng.Status().Phase))
}

func (r *room) phaseMsg() serverMsg {
	m := serverMsg{
		Type:  "phase",
		Phase: r.eng.Status().Phase.String(),
		Round: r.eng.Status().Round,
	}
	if !r.deadline.IsZero() {
		m.Deadline = r.deadline.UnixMilli()
	}
	return m
}

// onEvent 引擎产生的事件，按 AudienceOf 路由。
//
// 在房间的 goroutine 上被调用（EndPhase 就在这条 goroutine 上），
// 因此这里读引擎是安全的。
func (r *room) onEvent(ev *hiddenrole.Event) {
	audience, known := r.eng.AudienceOf(ev)
	if !known {
		// 第三方角色自定义的事件类型，引擎无从判断可见性
		log.Printf("[%s] 引擎不认得事件类型 %v，未路由", r.name, ev.Type)
		return
	}
	for _, id := range audience {
		r.sendTo(id, serverMsg{Type: "event", Event: ev})
	}
}

// onMessage 玩家发言，按引擎给出的接收者路由。
func (r *room) onMessage(msg *hiddenrole.Message, receivers []string) {
	for _, id := range receivers {
		r.sendTo(id, serverMsg{Type: "chat", From: msg.SenderID, Text: msg.Content})
	}
}

func (r *room) pushView(player string) {
	r.sendTo(player, serverMsg{Type: "view", View: r.eng.PlayerView(player)})
}

func (r *room) sendTo(player string, m serverMsg) {
	if c, ok := r.conns[player]; ok {
		c.send(m)
	}
}

// roomLogger 把引擎日志带上房间名。
type roomLogger struct{ room string }

func (l roomLogger) Debug(string, ...hiddenrole.Field) {}
func (l roomLogger) Info(msg string, f ...hiddenrole.Field) {
	log.Printf("[%s] %s%s", l.room, msg, fields(f))
}
func (l roomLogger) Warn(msg string, f ...hiddenrole.Field) {
	log.Printf("[%s] WARN %s%s", l.room, msg, fields(f))
}
func (l roomLogger) Error(msg string, f ...hiddenrole.Field) {
	log.Printf("[%s] ERROR %s%s", l.room, msg, fields(f))
}

func fields(f []hiddenrole.Field) string {
	out := ""
	for _, x := range f {
		out += fmt.Sprintf(" %s=%v", x.Key, x.Value)
	}
	return out
}
