// Package main 是一个 TCP 长连接的狼人杀服务端，也是这个库的第二个真实使用者。
//
// 命令行主持台（example/cli）验证的是「一个人在一台机器上主持一局」，
// 有一整类东西它碰不到：事件推送、每条连接一份视图、多局并发、断线重连、
// 超时真的触发。这个服务端专门压那半边。
//
// 协议：TCP，一行一条 JSON。
//
//	-> {"type":"join","player":"p1"}
//	<- {"type":"phase","phase":"NIGHT_GUARD","round":1,"deadline_ms":...}
//	<- {"type":"view","view":{...}}
//	-> {"type":"act","skill":"protect","target":"p4"}
//	-> {"type":"say","text":"我是好人"}
//	<- {"type":"event","event":{...}}      // 只推给该看到的人
//	<- {"type":"chat","from":"p2","text":"..."}
//
// 用 nc 就能玩：
//
//	go run ./example/netserver &
//	nc localhost 9000
//	{"type":"join","player":"p1"}
package main

import (
	"flag"
	"log"
	"net"
	"time"
)

func main() {
	addr := flag.String("addr", ":9000", "监听地址")
	flag.Parse()

	srv, err := newServer(*addr, 200*time.Millisecond)
	if err != nil {
		log.Fatalf("启动失败: %v", err)
	}
	log.Printf("狼人杀服务端已启动，监听 %s", srv.addr())
	log.Printf("试试: nc %s  然后发 {\"type\":\"join\",\"player\":\"p1\"}", srv.addr())
	srv.serve()
}

// server 一个监听器 + 一个房间。
//
// 刻意只做一个房间：多房间只是加一张 map，对检验这个库没有新意，
// 而每多一层就少一分「这段代码在讲什么」的清晰度。
// 真正值得压的是同一局里的多条连接。
type server struct {
	ln   net.Listener
	room *room
}

func newServer(addr string, tick time.Duration) (*server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	r, err := newRoom("room-1", tick)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}
	return &server{ln: ln, room: r}, nil
}

func (s *server) addr() string { return s.ln.Addr().String() }

func (s *server) serve() {
	for {
		raw, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(newConn(raw))
	}
}

func (s *server) close() { _ = s.ln.Close() }

// handle 一条连接的生命周期。
//
// 每条连接一个 goroutine，它只负责读与翻译；对引擎的操作一律投递到
// 房间的 goroutine 上执行。
func (s *server) handle(c *conn) {
	defer c.close()

	player := ""
	defer func() {
		if player != "" {
			s.room.cmds <- command{kind: "detach", player: player, conn: c}
		}
	}()

	for {
		msg, ok := c.read()
		if !ok {
			return // 客户端断开。局面留在引擎里，等他重连
		}

		switch msg.Type {
		case "join":
			if msg.Player == "" {
				c.send(serverMsg{Type: "error", Message: "join 需要 player"})
				continue
			}
			player = msg.Player
			s.room.cmds <- command{kind: "attach", player: player, conn: c}

		case "act":
			if player == "" {
				c.send(serverMsg{Type: "error", Message: "请先 join"})
				continue
			}
			skill, ok := skillByName[msg.Skill]
			if !ok {
				c.send(serverMsg{Type: "error", Message: "不认识的技能: " + msg.Skill})
				continue
			}
			s.room.cmds <- command{
				kind: "act", player: player, skill: skill, target: msg.Target,
			}

		case "say":
			if player == "" {
				c.send(serverMsg{Type: "error", Message: "请先 join"})
				continue
			}
			s.room.cmds <- command{kind: "say", player: player, text: msg.Text}

		case "view":
			if player == "" {
				c.send(serverMsg{Type: "error", Message: "请先 join"})
				continue
			}
			s.room.cmds <- command{kind: "view", player: player}

		default:
			c.send(serverMsg{Type: "error", Message: "不认识的报文类型: " + msg.Type})
		}
	}
}
