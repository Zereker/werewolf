// render.go 把引擎的类型翻译成主持人说得出口的话。
//
// 这一层完全属于使用者：库给的是 PhaseNightGuard、SkillAntidote 这样的
// 取值，「守卫请睁眼」「你要用解药吗」是桌面上的说法，没有理由塞进库里。

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Zereker/werewolf"
	"github.com/Zereker/werewolf/engine"
)

// ==================== 输入解析 ====================

var skillNames = map[string]werewolf.SkillType{
	"protect":  werewolf.SkillProtect,
	"守":        werewolf.SkillProtect,
	"kill":     werewolf.SkillKill,
	"刀":        werewolf.SkillKill,
	"check":    werewolf.SkillCheck,
	"验":        werewolf.SkillCheck,
	"antidote": werewolf.SkillAntidote,
	"救":        werewolf.SkillAntidote,
	"poison":   werewolf.SkillPoison,
	"毒":        werewolf.SkillPoison,
	"vote":     werewolf.SkillVote,
	"投":        werewolf.SkillVote,
	"shoot":    werewolf.SkillShoot,
	"枪":        werewolf.SkillShoot,
	"skip":     werewolf.SkillSkip,
	"过":        werewolf.SkillSkip,
}

func parseSkill(s string) (werewolf.SkillType, bool) {
	skill, ok := skillNames[strings.ToLower(s)]
	return skill, ok
}

// ==================== 展示 ====================

var phaseWords = map[werewolf.PhaseType]string{
	werewolf.PhaseNightGuard:   "守卫",
	werewolf.PhaseNightWolf:    "狼人",
	werewolf.PhaseNightWitch:   "女巫",
	werewolf.PhaseNightSeer:    "预言家",
	werewolf.PhaseNightResolve: "天亮结算",
	werewolf.PhaseNightHunter:  "猎人开枪",
	werewolf.PhaseDay:          "白天发言",
	werewolf.PhaseDayHunter:    "猎人开枪",
	werewolf.PhaseVote:         "投票",
	werewolf.PhaseEnd:          "结束",
}

func shortPhase(p werewolf.PhaseType) string {
	if s, ok := phaseWords[p]; ok {
		return s
	}
	return p.String()
}

var roleWords = map[werewolf.RoleType]string{
	werewolf.RoleWerewolf: "狼人",
	werewolf.RoleSeer:     "预言家",
	werewolf.RoleWitch:    "女巫",
	werewolf.RoleHunter:   "猎人",
	werewolf.RoleGuard:    "守卫",
	werewolf.RoleVillager: "平民",
}

func shortRole(r werewolf.RoleType) string {
	if s, ok := roleWords[r]; ok {
		return s
	}
	if r == engine.RoleUnspecified {
		return "全体"
	}
	return r.String()
}

var skillWords = map[werewolf.SkillType]string{
	werewolf.SkillProtect:  "守护",
	werewolf.SkillKill:     "刀",
	werewolf.SkillCheck:    "查验",
	werewolf.SkillAntidote: "解药",
	werewolf.SkillPoison:   "毒药",
	werewolf.SkillVote:     "投票",
	werewolf.SkillShoot:    "开枪",
	werewolf.SkillSkip:     "放弃",
	werewolf.SkillSpeak:    "发言",
}

func shortSkill(s werewolf.SkillType) string {
	if w, ok := skillWords[s]; ok {
		return w
	}
	return s.String()
}

func skillList(skills []werewolf.SkillType) string {
	if len(skills) == 0 {
		return "（无）"
	}
	out := make([]string, 0, len(skills))
	for _, s := range skills {
		out = append(out, shortSkill(s))
	}
	return strings.Join(out, "/")
}

var announcements = map[werewolf.PhaseType]string{
	werewolf.PhaseNightGuard:   "天黑请闭眼。守卫请睁眼，你要守护谁？",
	werewolf.PhaseNightWolf:    "守卫请闭眼。狼人请睁眼，你们要刀谁？",
	werewolf.PhaseNightWitch:   "狼人请闭眼。女巫请睁眼。",
	werewolf.PhaseNightSeer:    "女巫请闭眼。预言家请睁眼，你要查验谁？",
	werewolf.PhaseNightResolve: "预言家请闭眼。天亮了。",
	werewolf.PhaseNightHunter:  "猎人，你可以开枪。",
	werewolf.PhaseDay:          "请依次发言。",
	werewolf.PhaseDayHunter:    "猎人，你可以开枪。",
	werewolf.PhaseVote:         "请投票。",
}

func announce(p werewolf.PhaseType) string {
	if s, ok := announcements[p]; ok {
		return s
	}
	return "请行动。"
}

// describe 把一个效果讲成一句话。
//
// 被否决的效果要讲清楚「试了但没成」以及原因——少了这一层，
// 主持人会把一次失败的用毒当成真的毒死了人。
func describe(ef *werewolf.Effect) string {
	verb := map[werewolf.EventType]string{
		werewolf.EventKill:       "被刀",
		werewolf.EventPoison:     "被毒",
		werewolf.EventEliminate:  "被投票出局",
		werewolf.EventShoot:      "被枪打死",
		werewolf.EventProtect:    "被守护",
		werewolf.EventSave:       "被解药救回",
		werewolf.EventCheck:      "被查验",
		werewolf.EventSkip:       "放弃行动",
		werewolf.EventVoteTied:   "平票，无人出局",
		engine.EventGameStarted:  "游戏开始",
		engine.EventGameEnded:    "游戏结束",
		engine.EventPlayerAdded:  "入座",
		engine.EventPhaseChanged: "阶段流转",

		// 内部效果只会出现在效果流里，不会发给任何玩家。
		// log 看的是原始流水，把它们也译出来才读得懂。
		//
		// 这几条是内核的状态原语，与角色无关：谁死了、谁身上多了个标记。
		// 「记下今晚的刀口」「消耗解药」这类说法此前也是事件类型，
		// 现在它们只是原语里的一个键名，读法见 varLabel。
		engine.EventSetAlive: "存活状态变更",
		engine.EventSetVar:   "改状态",
		engine.EventDetour:   "待结算的绕道",
	}[ef.Type]
	if verb == "" {
		verb = ef.Type.String()
	}

	var b strings.Builder
	if ef.Canceled {
		b.WriteString("（未生效）")
	}
	if ef.SourceID != "" {
		fmt.Fprintf(&b, "%s -> ", ef.SourceID)
	}
	if ef.TargetID != "" {
		b.WriteString(ef.TargetID + " ")
	}
	b.WriteString(verb)

	if ef.Type == werewolf.EventCheck {
		if good, ok := ef.Data["isGood"].(bool); ok {
			b.WriteString("：" + campWord(good))
		}
	}
	if ef.Canceled {
		b.WriteString("，原因: " + phrase(ef.Reason))
	} else if r, ok := ef.Data["reason"].(string); ok {
		b.WriteString("，" + phrase(r))
	}
	if w, ok := ef.Data["winner"].(werewolf.Camp); ok {
		b.WriteString("，胜方: " + campSide(w))
	}
	if p, ok := ef.Data["phase"].(werewolf.PhaseType); ok {
		b.WriteString(" -> " + shortPhase(p))
	}
	if r, ok := ef.Data["role"].(werewolf.RoleType); ok {
		b.WriteString("，" + shortRole(r))
	}
	return b.String()
}

// reasonPhrases 把库给出的原因翻成人话。
//
// 这是使用者绕不开的一层：Effect.Reason 与 Data["reason"] 是英文自由文本，
// 不是可枚举的取值，想本地化就只能按字符串对表。对不上的原样透出，
// 至少不会把信息吃掉。
var reasonPhrases = map[string]string{
	"guard and antidote used on the same target":     "同守同救，依然死亡",
	"protected by guard":                             "被守卫守住",
	"saved by witch antidote":                        "被女巫解药救回",
	"cannot protect same target consecutively":       "不能连续两晚守同一个人",
	"guard cannot protect self":                      "守卫不能自守",
	"cannot use both potions in one night":           "同一晚不能既用解药又用毒药",
	"no antidote":                                    "解药已经用掉了",
	"no poison":                                      "毒药已经用掉了",
	"witch cannot save self":                         "女巫不能自救",
	"witch cannot poison self":                       "女巫不能毒自己",
	"no one is dying tonight":                        "今晚没有人被刀",
	"target is not dying":                            "他今晚没被刀",
	"target phase is not present in the game config": "配置里没有对应的阶段",
}

func phrase(reason string) string {
	if s, ok := reasonPhrases[reason]; ok {
		return s
	}
	return reason
}

func campWord(good bool) string {
	if good {
		return "好人"
	}
	return "狼人"
}

func campSide(c werewolf.Camp) string {
	if c == werewolf.CampEvil {
		return "狼人阵营"
	}
	return "好人阵营"
}

func aliveWord(alive bool) string {
	if alive {
		return "存活"
	}
	return "已出局"
}

// roleInfoKeyLabels 认识的 RoleInfo 键的中文说法。
//
// 引擎不认识任何具体角色，这份对照表是**主持台**的事，不是库的事：
// 扩展角色自己定的键不在表里，会原样打出来，不会被吞掉。
var roleInfoKeyLabels = map[string]string{
	werewolf.RoleInfoAntidote:   "解药",
	werewolf.RoleInfoPoison:     "毒药",
	werewolf.RoleInfoKillTarget: "今晚被刀的是",
}

// roleInfoLines 把角色专属信息渲染成若干行，键序稳定。
func roleInfoLines(info map[string]string) []string {
	if len(info) == 0 {
		return nil
	}

	keys := make([]string, 0, len(info))
	for k := range info {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		label, ok := roleInfoKeyLabels[k]
		if !ok {
			label = k
		}
		// 存量这类布尔状态只在「有」的时候出现在 RoleInfo 里，
		// 值本身（"1"）没有信息量，打标签就够了。
		if info[k] == werewolf.VarPresent {
			lines = append(lines, label)
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, info[k]))
	}
	return lines
}

func revealed(r werewolf.RoleType) string {
	if r == engine.RoleUnspecified {
		return ""
	}
	return "(" + shortRole(r) + ")"
}

func deadMark(alive bool) string {
	if alive {
		return ""
	}
	return "✝"
}

// reason 把库返回的错误讲成主持人能直接念出来的话。
//
// 库按 errors.Is 可比对的哨兵给出错误，所以这里能逐类翻译，
// 而不是把英文原文甩给玩家。
func reason(err error) string {
	switch {
	case errors.Is(err, engine.ErrPlayerNotFound):
		return "没有这个玩家"
	case errors.Is(err, engine.ErrPlayerDead):
		return "他已经出局了"
	case errors.Is(err, engine.ErrTargetNotFound):
		return "没有这个目标"
	case errors.Is(err, engine.ErrTargetDead):
		return "目标已经出局了"
	case errors.Is(err, engine.ErrSkillNotAllowed):
		return "现在轮不到他用这个技能"
	case errors.Is(err, engine.ErrMessageNotAllowed):
		return "这个阶段他不能发言"
	case errors.Is(err, engine.ErrGameEnded):
		return "游戏已经结束"
	case errors.Is(err, engine.ErrGameNotStarted):
		return "游戏还没开始"
	case errors.Is(err, engine.ErrInvalidSnapshot):
		return "存档不合法或版本不匹配"
	default:
		return err.Error()
	}
}

// ==================== 存档的序列化 ====================
//
// 库只负责把局面导出成可序列化的结构，用什么格式存是使用者的事。

func marshalSnapshot(s *werewolf.Snapshot) ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

func unmarshalSnapshot(data []byte) (*werewolf.Snapshot, error) {
	var s werewolf.Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ==================== 帮助 ====================

func (t *table) banner() {
	fmt.Println("狼人杀 · 命令行主持台")
	fmt.Printf("9 人局：3 狼 + 预言家/女巫/守卫/猎人 + 2 民，座位 %v\n", t.seats)
	fmt.Println("输入 help 看命令，run 让它自己跑完一局。")
	fmt.Println()
	t.godView()
}

func usage() {
	rows := [][2]string{
		{"status", "阶段、回合、谁还活着、建议还剩多久"},
		{"info", "上帝视角：本阶段该谁行动、狼队名单、女巫可见的刀口"},
		{"who", "还差谁没行动"},
		{"view <玩家>", "某个玩家看到的东西（这一份可以原样发给他）"},
		{"act <玩家> <技能> [目标]", "提交技能，技能可用 守/刀/验/救/毒/投/枪/过"},
		{"say <玩家> <内容>", "发言，谁能听到由当前阶段决定"},
		{"end", "结束本阶段"},
		{"auto", "替所有该行动的人随便动一下再结束本阶段"},
		{"run [n]", "连续 auto，直到游戏结束"},
		{"log", "自开局以来的完整效果流"},
		{"save <文件> / load <文件>", "存档与恢复"},
		{"quit", "退出"},
	}
	width := 0
	for _, r := range rows {
		if len(r[0]) > width {
			width = len(r[0])
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return false })
	for _, r := range rows {
		fmt.Printf("  %-*s  %s\n", width, r[0], r[1])
	}
}
