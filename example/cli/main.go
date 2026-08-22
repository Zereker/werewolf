// Package main 是一个可以真的从头玩完一局的命令行主持台。
//
// 它存在的理由不是演示 API，而是**当这个库的第一个真实使用者**。
// 库此前一直在自言自语：测试是库作者写的、示例是照着 API 抄的，
// 没有任何一段代码是站在「我要用它做一个游戏」的位置上写出来的。
// 这个 CLI 补上那个位置——它得自己解决超时、路由、断线重连这些
// 库刻意不管的事，从而暴露出库到底好不好用。
//
// 用法：
//
//	go run ./example/cli              # 交互
//	go run ./example/cli < script.txt # 照脚本跑（CI 用的就是这个）
//	go run ./example/cli -seed 7      # 固定发牌
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Zereker/werewolf"
	"github.com/Zereker/werewolf/engine"
)

func main() {
	seed := flag.Int64("seed", 0, "发牌随机种子，0 表示按时间")
	flag.Parse()

	if *seed == 0 {
		*seed = time.Now().UnixNano()
	}

	t := newTable(*seed)
	t.banner()
	t.repl(os.Stdin)
}

// ==================== 牌桌 ====================

// table 一张牌桌：引擎，加上库刻意不管、必须由使用者自己拿主意的东西。
type table struct {
	eng   *werewolf.Engine
	seats []string // 座位顺序，用于稳定的展示

	// rng 发牌与 auto 托管共用的随机源。
	//
	// 共用是为了让 -seed 真的能复现一整局：此前只有发牌用了它，
	// 托管挑技能与目标走的是全局 rand，同一个种子跑两次结果不一样，
	// 拿它复现一个 bug 是复现不出来的。
	rng *rand.Rand

	// deadline 本阶段的建议截止时间。
	//
	// 引擎不计时——它只通过 GameConfig.PhaseTimeout 给出建议值，
	// 什么时候真的推进由使用者决定。这里就是那个「使用者」。
	deadline time.Time
}

// 默认 9 人局：3 狼 + 4 神 + 2 民
var defaultBoard = []werewolf.RoleType{
	werewolf.RoleWerewolf, werewolf.RoleWerewolf, werewolf.RoleWerewolf,
	werewolf.RoleSeer, werewolf.RoleWitch, werewolf.RoleGuard, werewolf.RoleHunter,
	werewolf.RoleVillager, werewolf.RoleVillager,
}

func newTable(seed int64) *table {
	rng := rand.New(rand.NewSource(seed))

	roles := append([]werewolf.RoleType(nil), defaultBoard...)
	rng.Shuffle(len(roles), func(i, j int) { roles[i], roles[j] = roles[j], roles[i] })

	eng := werewolf.MustNew(werewolf.DefaultRules())
	seats := make([]string, 0, len(roles))
	for i, role := range roles {
		id := fmt.Sprintf("%d号", i+1)
		if err := eng.AddPlayer(id, role); err != nil {
			fatal("入座失败: %v", err)
		}
		seats = append(seats, id)
	}
	if err := eng.Start(); err != nil {
		fatal("开局失败: %v", err)
	}

	t := &table{eng: eng, seats: seats, rng: rng}
	t.armTimer()
	return t
}

// armTimer 按引擎给出的建议超时定一个本阶段的截止时间。
func (t *table) armTimer() {
	t.deadline = time.Now().Add(werewolf.DefaultGameConfig().PhaseTimeout(t.eng.Status().Phase))
}

// ==================== 主循环 ====================

func (t *table) repl(in *os.File) {
	interactive := isTerminal(in)
	scanner := bufio.NewScanner(in)

	for {
		if interactive {
			fmt.Print(t.prompt())
		}
		if !scanner.Scan() {
			return
		}
		line := strings.TrimSpace(scanner.Text())
		if !interactive && line != "" {
			// 照脚本跑时把命令回显出来，输出本身才读得懂
			fmt.Printf("\n%s%s\n", t.prompt(), line)
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if quit := t.dispatch(line); quit {
			return
		}
	}
}

func (t *table) prompt() string {
	st := t.eng.Status()
	return fmt.Sprintf("[第%d回合 %s] > ", st.Round, shortPhase(st.Phase))
}

func (t *table) dispatch(line string) (quit bool) {
	fields := strings.Fields(line)
	cmd, args := fields[0], fields[1:]

	switch cmd {
	case "help", "?":
		usage()
	case "status":
		t.status()
	case "info":
		t.godView()
	case "who":
		t.who()
	case "view":
		t.playerView(args)
	case "act":
		t.act(args)
	case "say":
		t.say(args)
	case "end":
		t.end()
	case "auto":
		t.auto()
	case "run":
		t.run(args)
	case "log":
		t.showLog()
	case "save":
		t.save(args)
	case "load":
		t.load(args)
	case "quit", "exit":
		return true
	default:
		warn("不认识的命令 %q，输入 help 看可用命令", cmd)
	}
	return false
}

// ==================== 只读的几个视角 ====================

func (t *table) status() {
	e := t.eng
	st := e.Status()
	fmt.Printf("  回合 %d，阶段 %s\n", st.Round, shortPhase(st.Phase))

	if e.Status().Over {
		fmt.Println("  游戏已结束")
		return
	}
	if left := time.Until(t.deadline); left > 0 {
		fmt.Printf("  建议还剩 %s（引擎不计时，超时与否由主持人决定）\n", left.Round(time.Second))
	} else {
		fmt.Println("  已超过建议时间")
	}

	alive := make(map[string]bool, len(t.seats))
	for _, id := range e.AlivePlayerIDs() {
		alive[id] = true
	}
	fmt.Print("  座位: ")
	for _, id := range t.seats {
		if alive[id] {
			fmt.Printf("%s ", id)
			continue
		}
		fmt.Printf("%s(出局) ", id)
	}
	fmt.Println()
}

// godView 上帝视角：本阶段该谁行动、狼队名单、女巫可见的刀口。
// 这些内容不可以整体转发给玩家。
func (t *table) godView() {
	info := t.eng.PhaseInfo()

	if info.NeedsGodAnnouncement() {
		fmt.Printf("  [公告] %s\n", announce(info.Phase))
	}
	if len(info.ActiveRoles) == 0 {
		fmt.Println("  本阶段无人需要行动，直接 end 即可")
		return
	}

	for _, role := range info.ActiveRoles {
		ri := info.RoleInfos[role]
		if ri == nil || len(ri.PlayerIDs) == 0 {
			continue
		}
		fmt.Printf("  %s: %v  可用技能 %s\n",
			shortRole(role), ri.PlayerIDs, skillList(ri.AllowedSkills))
		for _, id := range sortedKeys(ri.Teammates) {
			fmt.Printf("    %s 的狼队友: %v\n", id, ri.Teammates[id])
		}
		// 角色专属信息由角色自己填，主持台不认识具体角色，照单打印
		for _, id := range sortedKeys(ri.RoleInfo) {
			for _, k := range sortedKeys(ri.RoleInfo[id]) {
				fmt.Printf("    %s 的 %s: %s\n", id, k, ri.RoleInfo[id][k])
			}
		}
	}
}

// who 还差谁行动。
//
// 引擎把「必须动」和「可以动」分成两份：Pending 是超时就该推进的依据，
// Optional 是该催一催的依据。主持人两份都要看——只看前者的话，
// 默认配置下守卫、女巫、预言家、猎人一整局都不会被叫到。
func (t *table) who() {
	r := t.eng.PhaseReadiness()

	for _, p := range r.Pending {
		fmt.Printf("  必须等: %s(%s) 的 %s\n", p.PlayerID, shortRole(p.Role), shortSkill(p.Skill))
	}
	for _, p := range r.Optional {
		fmt.Printf("  可以催: %s(%s) 的 %s（不动也合法）\n",
			p.PlayerID, shortRole(p.Role), shortSkill(p.Skill))
	}
	if r.Ready && len(r.Optional) == 0 {
		fmt.Println("  该动的都动过了")
	}
	if len(r.Acted) > 0 {
		fmt.Printf("  已提交: %v\n", r.Acted)
	}
}

// playerView 某个玩家看到的东西——这一份可以原样发给他。
func (t *table) playerView(args []string) {
	if len(args) != 1 {
		warn("用法: view <玩家>")
		return
	}
	v := t.eng.PlayerView(args[0])
	if v == nil {
		warn("没有这个玩家: %s", args[0])
		return
	}

	fmt.Printf("  你是 %s，%s，%s\n", v.Self.ID, shortRole(v.Self.Role), aliveWord(v.Self.Alive))
	if len(v.Teammates) > 0 {
		fmt.Printf("  你的狼队友: %v\n", v.Teammates)
	}
	// 角色专属信息统一由 RoleInfo 来，主持台不需要知道谁是女巫——
	// 认识的键给个中文说法，不认识的（扩展角色自己定的）原样打出来。
	for _, line := range roleInfoLines(v.RoleInfo) {
		fmt.Printf("  %s\n", line)
	}
	if len(v.AllowedSkills) == 0 {
		fmt.Println("  现在还轮不到你")
	} else {
		fmt.Printf("  你现在可以: %s\n", skillList(v.AllowedSkills))
	}

	fmt.Print("  场上: ")
	for _, p := range v.Players {
		fmt.Printf("%s%s%s ", p.ID, revealed(p.Role), deadMark(p.Alive))
	}
	fmt.Println()
}

func (t *table) showLog() {
	for i, ef := range t.eng.EffectLog() {
		fmt.Printf("  %3d %s\n", i, describe(ef))
	}
}

// ==================== 推进 ====================

func (t *table) act(args []string) {
	if len(args) < 2 {
		warn("用法: act <玩家> <技能> [目标]")
		return
	}
	skill, ok := parseSkill(args[1])
	if !ok {
		warn("不认识的技能 %q", args[1])
		return
	}
	var target string
	if len(args) > 2 {
		target = args[2]
	}

	err := t.eng.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: args[0], Skill: skill, Targets: []string{target},
	})
	if err != nil {
		// 库把「为什么不行」分了类，主持人照着分类给出人话
		warn("%s 用不了这个技能: %s", args[0], reason(err))
		return
	}
	fmt.Printf("  已记下: %s %s %s\n", args[0], shortSkill(skill), target)
}

func (t *table) say(args []string) {
	if len(args) < 2 {
		warn("用法: say <玩家> <内容>")
		return
	}
	sender, text := args[0], strings.Join(args[1:], " ")

	// 谁能听到由引擎按阶段决定：狼人夜里只有狼队互通，白天全场可闻
	receivers := t.eng.MessageReceivers(sender)
	if err := t.eng.SendMessage(sender, text); err != nil {
		warn("%s 现在发不了言: %s", sender, reason(err))
		return
	}
	fmt.Printf("  %s 说「%s」-> 听到的人: %v\n", sender, text, receivers)
}

func (t *table) end() {
	if t.eng.Status().Over {
		warn("游戏已经结束了")
		return
	}
	if r := t.eng.PhaseReadiness(); !r.Ready {
		// 引擎不会因为没就绪而拒绝推进，是否等下去是主持人的判断
		fmt.Printf("  （还差 %d 项必需行动，按主持人意愿强行推进）\n", len(r.Pending))
	}

	from := t.eng.Status().Phase
	effects, err := t.eng.EndPhase()
	if err != nil {
		warn("推进失败: %s", reason(err))
		return
	}

	fmt.Printf("  %s 结束 -> %s\n", shortPhase(from), shortPhase(t.eng.Status().Phase))
	t.deliver(effects)
	t.armTimer()

	if t.eng.Status().Over {
		t.reveal()
	}
}

// deliver 把本阶段产生的事情按引擎给出的受众分发下去。
//
// 这是 AudienceOf 的用武之地，也是这个 CLI 唯一真正「像个服务端」的部分：
// 同一件事，全场可见的公告一份，只给行动者本人的私信一份。
func (t *table) deliver(effects []*werewolf.Effect) {
	for _, ef := range effects {
		// EndPhase 给的是内部的 Effect，AudienceOf 问的是对外的事件
		audience, known := t.eng.AudienceOf(ef.ToEvent())
		if !known {
			// 第三方角色自定义的事件类型，引擎无从判断可见性
			fmt.Printf("  [?] %s（引擎不认得这个类型，需自行路由）\n", describe(ef))
			continue
		}
		if len(audience) == 0 {
			continue // 内部效果，不该出现在任何玩家面前
		}
		if len(audience) == len(t.seats) {
			fmt.Printf("  [全场] %s\n", describe(ef))
			continue
		}
		fmt.Printf("  [私信 %v] %s\n", audience, describe(ef))
	}
}

// reveal 结束后翻牌。谁是什么身份属于桌面规则，引擎不替主持人决定。
func (t *table) reveal() {
	fmt.Println("  === 游戏结束，翻牌 ===")
	for _, id := range t.seats {
		p, _ := t.eng.PlayerInfo(id)
		fmt.Printf("  %s: %s %s\n", id, shortRole(p.Role), aliveWord(p.Alive))
	}
}

// auto 替所有该行动的人随便动一下，然后结束本阶段。
//
// 驱动的依据是 PhaseInfo：它给的是本阶段每个角色的完整可用技能表，
// 而 PhaseReadiness 给的是「还欠哪一次行动」——想让每个人都动一动，
// 前者才是问对了问题。
func (t *table) auto() {
	if t.eng.Status().Over {
		warn("游戏已经结束了")
		return
	}

	alive := t.eng.AlivePlayerIDs()
	info := t.eng.PhaseInfo()
	for _, role := range info.ActiveRoles {
		ri := info.RoleInfos[role]
		if ri == nil || len(ri.AllowedSkills) == 0 {
			continue
		}
		for _, id := range ri.PlayerIDs {
			skill := ri.AllowedSkills[t.rng.Intn(len(ri.AllowedSkills))]
			target := ""
			if skill != werewolf.SkillSkip {
				target = alive[t.rng.Intn(len(alive))]
			}
			// 提交失败是正常的：随机挑的目标可能不合规（女巫救了没被刀的人、
			// 守卫连守同一个人）。规则由引擎裁决，这里不预判。
			_ = t.eng.SubmitSkillUse(&werewolf.SkillUse{
				PlayerID: id, Skill: skill, Targets: []string{target},
			})
		}
	}
	t.end()
}

// run 连续 auto 直到游戏结束。上限只是防跑飞，不是预期的结束方式。
func (t *table) run(args []string) {
	limit := 200
	if len(args) > 0 {
		if v, err := strconv.Atoi(args[0]); err == nil {
			limit = v
		}
	}
	for i := 0; i < limit && !t.eng.Status().Over; i++ {
		t.auto()
	}
	if !t.eng.Status().Over {
		warn("跑了 %d 个阶段还没结束，停下来了", limit)
	}
}

// ==================== 存档 ====================

// save / load 演示的是「服务重启了怎么办」。
//
// 引擎不做存储，只负责把局面导出成可序列化的结构；存到哪、怎么存
// 是使用者的事。这里就写成一个文件。
func (t *table) save(args []string) {
	if len(args) != 1 {
		warn("用法: save <文件>")
		return
	}
	data, err := marshalSnapshot(t.eng.Snapshot())
	if err != nil {
		warn("导出失败: %v", err)
		return
	}
	if err := os.WriteFile(args[0], data, 0o600); err != nil {
		warn("写文件失败: %v", err)
		return
	}
	fmt.Printf("  已存档到 %s\n", args[0])
}

func (t *table) load(args []string) {
	if len(args) != 1 {
		warn("用法: load <文件>")
		return
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		warn("读文件失败: %v", err)
		return
	}
	snap, err := unmarshalSnapshot(data)
	if err != nil {
		warn("解析失败: %v", err)
		return
	}
	// 恢复时必须给回同一套规则配置：快照只记局面，不记规则
	eng, err := engine.RestoreEngine(nil, snap)
	if err != nil {
		warn("恢复失败: %s", reason(err))
		return
	}
	t.eng = eng
	t.armTimer()
	st := eng.Status()
	fmt.Printf("  已从 %s 恢复：第%d回合 %s\n", args[0], st.Round, shortPhase(st.Phase))
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "错误: "+format+"\n", args...)
	os.Exit(1)
}

func warn(format string, args ...any) {
	fmt.Printf("  ! "+format+"\n", args...)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
