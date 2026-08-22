// conn.go 传输层：TCP 长连接，一行一条 JSON。
//
// 一开始写的是 WebSocket，手写了 180 行 RFC 6455 的握手与帧编解码——
// 因为库是零依赖的，不能为了示例把 gorilla/websocket 塞进 go.mod。
// 但 WebSocket 在这里唯一多出来的能力是「浏览器能连」，而这个示例
// 要检验的是引擎在推送、每连接一份视图、并发、断线重连下好不好用，
// 与浏览器无关。换成裸 TCP 之后那 180 行全部消失，nc 就能玩。

package main

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
)

// conn 一条客户端连接。
//
// 读由连接自己的读循环独占；写集中在 writeLoop 一个 goroutine 上，
// 因此 send 可以被任意 goroutine 调用。
type conn struct {
	raw net.Conn
	in  *bufio.Scanner
	out chan serverMsg

	closeOnce sync.Once
	closed    chan struct{}
}

// maxLine 单行上限。防住恶意的超长行，也顺便说明这个协议是行分隔的。
const maxLine = 1 << 20

func newConn(raw net.Conn) *conn {
	sc := bufio.NewScanner(raw)
	sc.Buffer(make([]byte, 0, 4096), maxLine)

	c := &conn{
		raw:    raw,
		in:     sc,
		out:    make(chan serverMsg, 64),
		closed: make(chan struct{}),
	}
	go c.writeLoop()
	return c
}

// read 读下一条客户端报文。连接关闭时返回 false。
func (c *conn) read() (clientMsg, bool) {
	for c.in.Scan() {
		line := c.in.Bytes()
		if len(line) == 0 {
			continue
		}
		var m clientMsg
		if err := json.Unmarshal(line, &m); err != nil {
			c.send(serverMsg{Type: "error", Message: "报文不是合法 JSON: " + err.Error()})
			continue
		}
		return m, true
	}
	return clientMsg{}, false
}

// send 把一条报文放进发送队列。
//
// 队列满了就丢：一个读得慢的客户端不该把整局游戏拖住。
// 对局面本身没有影响——引擎里的状态是权威，客户端随时可以重连拿一份新的视图。
func (c *conn) send(m serverMsg) {
	select {
	case c.out <- m:
	case <-c.closed:
	default:
	}
}

func (c *conn) writeLoop() {
	enc := json.NewEncoder(c.raw)
	for {
		select {
		case <-c.closed:
			return
		case m := <-c.out:
			if err := enc.Encode(m); err != nil {
				c.close()
				return
			}
		}
	}
}

func (c *conn) close() {
	c.closeOnce.Do(func() {
		close(c.closed)
		_ = c.raw.Close()
	})
}
