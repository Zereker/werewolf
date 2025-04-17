package room

import (
	"encoding/json"
	"io"
	"log"
	"sync"

	"github.com/Zereker/werewolf/pkg/arean/werewolf"
	"github.com/Zereker/werewolf/pkg/game"
)

// Connection 连接接口
type Connection interface {
	// Read 读取消息
	Read() ([]byte, error)
	// Write 写入消息
	Write([]byte) error
	// Close 关闭连接
	Close() error
}

// StdConnection 标准输入输出连接实现
type StdConnection struct {
	reader io.Reader
	writer io.Writer
}

func NewStdConnection(reader io.Reader, writer io.Writer) *StdConnection {
	return &StdConnection{
		reader: reader,
		writer: writer,
	}
}

func (c *StdConnection) Read() ([]byte, error) {
	buf := make([]byte, 1024)
	n, err := c.reader.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (c *StdConnection) Write(data []byte) error {
	_, err := c.writer.Write(append(data, '\n'))
	return err
}

func (c *StdConnection) Close() error {
	return nil
}

// Message 玩家消息
type Message struct {
	Type    string      `json:"type"`    // 消息类型
	Content interface{} `json:"content"` // 消息内容
}

// Player 玩家
type Player struct {
	sync.RWMutex
	id        int64
	role      game.Role
	conn      Connection
	room      *Room
	done      chan struct{}
	eventChan chan *werewolf.Event
}

// NewPlayer 创建新玩家
func NewPlayer(id int64, role game.Role, conn Connection, room *Room) *Player {
	return &Player{
		id:        id,
		role:      role,
		conn:      conn,
		room:      room,
		done:      make(chan struct{}),
		eventChan: make(chan *werewolf.Event, 100),
	}
}

// Start 开始处理玩家消息
func (p *Player) Start() {
	go p.readLoop()
	go p.eventLoop()
}

// Stop 停止处理玩家消息
func (p *Player) Stop() {
	close(p.done)
	p.conn.Close()
}

// readLoop 读取玩家消息循环
func (p *Player) readLoop() {
	for {
		select {
		case <-p.done:
			return
		default:
			data, err := p.conn.Read()
			if err != nil {
				log.Printf("Player %d read error: %v", p.id, err)
				continue
			}

			var msg Message
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Printf("Player %d unmarshal message error: %v", p.id, err)
				continue
			}

			p.handleMessage(&msg)
		}
	}
}

// eventLoop 处理游戏事件循环
func (p *Player) eventLoop() {
	for {
		select {
		case <-p.done:
			return
		case evt := <-p.eventChan:
			p.handleEvent(evt)
		}
	}
}

// handleMessage 处理玩家消息
func (p *Player) handleMessage(msg *Message) {
	switch msg.Type {
	case "skill":
		p.handleSkillMessage(msg)
	case "vote":
		p.handleVoteMessage(msg)
	case "speak":
		p.handleSpeakMessage(msg)
	}
}

// handleEvent 处理游戏事件
func (p *Player) handleEvent(evt *werewolf.Event) {
	// 根据事件类型处理
	switch evt.Type {
	case werewolf.EventSystemGameStart:
		p.handleGameStart(evt)
	case werewolf.EventSystemPhaseStart:
		p.handlePhaseStart(evt)
	case werewolf.EventSystemSkillResult:
		p.handleSkillResult(evt)
	case werewolf.EventSystemGameEnd:
		p.handleGameEnd(evt)
	}
}

// 处理各种消息的具体方法
func (p *Player) handleSkillMessage(msg *Message) {
	var data struct {
		SkillType game.SkillType `json:"skill_type"`
		TargetID  int64          `json:"target_id"`
	}
	if err := json.Unmarshal(msg.Content.([]byte), &data); err != nil {
		log.Printf("Player %d unmarshal skill message error: %v", p.id, err)
		return
	}

	p.room.runtime.UseSkill(p.id, data.TargetID, data.SkillType)
}

func (p *Player) handleVoteMessage(msg *Message) {
	var data struct {
		TargetID int64 `json:"target_id"`
	}
	if err := json.Unmarshal(msg.Content.([]byte), &data); err != nil {
		log.Printf("Player %d unmarshal vote message error: %v", p.id, err)
		return
	}

	p.room.runtime.Vote(p.id, data.TargetID)
}

func (p *Player) handleSpeakMessage(msg *Message) {
	var data struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(msg.Content.([]byte), &data); err != nil {
		log.Printf("Player %d unmarshal speak message error: %v", p.id, err)
		return
	}

	p.room.runtime.Speak(p.id, data.Message)
}

// 处理各种事件的具体方法
func (p *Player) handleGameStart(evt *werewolf.Event) {
	data := evt.Data.(*werewolf.SystemGameStartData)
	response := Message{
		Type:    "game_start",
		Content: data,
	}
	p.sendMessage(&response)
}

func (p *Player) handlePhaseStart(evt *werewolf.Event) {
	data := evt.Data.(*werewolf.SystemPhaseData)
	response := Message{
		Type:    "phase_start",
		Content: data,
	}
	p.sendMessage(&response)
}

func (p *Player) handleSkillResult(evt *werewolf.Event) {
	// 只处理与自己相关的技能结果
	if evt.PlayerID == p.id || evt.TargetID == p.id {
		data := evt.Data.(*werewolf.SystemSkillResultData)
		response := Message{
			Type:    "skill_result",
			Content: data,
		}
		p.sendMessage(&response)
	}
}

func (p *Player) handleGameEnd(evt *werewolf.Event) {
	data := evt.Data.(*werewolf.SystemGameEndData)
	response := Message{
		Type:    "game_end",
		Content: data,
	}
	p.sendMessage(&response)
}

// sendMessage 发送消息给玩家
func (p *Player) sendMessage(msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Player %d marshal message error: %v", p.id, err)
		return
	}

	if err := p.conn.Write(data); err != nil {
		log.Printf("Player %d write message error: %v", p.id, err)
	}
}

// OnEvent 接收游戏事件
func (p *Player) OnEvent(evt *werewolf.Event) {
	select {
	case p.eventChan <- evt:
	default:
		log.Printf("Player %d event channel full", p.id)
	}
}
