// protocol.go 客户端与服务端之间的报文，一行一条 JSON。
//
// 字段是刻意贴着引擎的形状定的——PlayerView 与 Event 本来就是
// 「可以直接发给玩家」的东西，这一层几乎不做转换。
// 那正是这个库想要的效果：信息边界已经在库里划好了。

package main

import "github.com/Zereker/werewolf"

// clientMsg 客户端发来的。
//
//	{"type":"join","player":"p1"}
//	{"type":"act","skill":"kill","target":"p4"}
//	{"type":"say","text":"我是好人"}
//	{"type":"view"}
type clientMsg struct {
	Type string `json:"type"`

	Player string `json:"player,omitempty"`
	Skill  string `json:"skill,omitempty"`
	Target string `json:"target,omitempty"`
	Text   string `json:"text,omitempty"`
}

// serverMsg 服务端推给某一条连接的。
type serverMsg struct {
	Type string `json:"type"`

	// view：这个玩家此刻有权知道的一切，直接来自 Engine.PlayerView
	View *werewolf.PlayerView `json:"view,omitempty"`

	// event：发生了什么。只推给 Engine.AudienceOf 划出来的那些人
	Event *werewolf.Event `json:"event,omitempty"`

	// chat：谁说了什么。接收者由 Engine 按阶段决定
	From string `json:"from,omitempty"`
	Text string `json:"text,omitempty"`

	// phase：阶段流转与本阶段的截止时间
	Phase    string `json:"phase,omitempty"`
	Round    int    `json:"round,omitempty"`
	Deadline int64  `json:"deadline_ms,omitempty"` // Unix 毫秒，0 表示不计时

	Message string `json:"message,omitempty"` // error / info 的正文
}

// skillByName 客户端用的技能名。
var skillByName = map[string]werewolf.SkillType{
	"protect":  werewolf.SkillProtect,
	"kill":     werewolf.SkillKill,
	"check":    werewolf.SkillCheck,
	"antidote": werewolf.SkillAntidote,
	"poison":   werewolf.SkillPoison,
	"vote":     werewolf.SkillVote,
	"shoot":    werewolf.SkillShoot,
	"skip":     werewolf.SkillSkip,
}
