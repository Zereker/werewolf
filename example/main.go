// Package main 演示如何使用狼人杀游戏引擎
//
// 本示例展示了:
//   - 创建游戏引擎和配置
//   - 添加玩家（分配角色和阵营）
//   - 游戏流程控制（阶段转换）
//   - 技能提交（狼人击杀、女巫救人/毒人、守卫保护、预言家查验）
//   - 事件处理（监听游戏事件）
//   - 消息系统（狼人交流、白天发言）
package main

import (
	"fmt"
	"log"

	"github.com/Zereker/werewolf"
	"github.com/Zereker/werewolf/engine"
)

func main() {
	fmt.Println("=== 狼人杀游戏引擎示例 ===")
	fmt.Println()

	// 示例1: 基础游戏设置
	basicGameSetup()

	fmt.Println()
	fmt.Println("---")
	fmt.Println()

	// 示例2: 上帝（主持人）引导游戏
	godNarratorDemo()

	fmt.Println()
	fmt.Println("---")
	fmt.Println()

	// 示例3: 完整游戏流程
	fullGameFlow()

	fmt.Println()
	fmt.Println("---")
	fmt.Println()

	// 示例4: 消息系统演示
	messagingDemo()
}

// basicGameSetup 展示基础游戏设置
func basicGameSetup() {
	fmt.Println("【示例1: 基础游戏设置】")

	// 1. 规则开关（可选，DefaultRules 已经是维基那一套）
	rules := werewolf.DefaultRules()
	rules.WitchCanSaveSelf = false    // 女巫不能自救
	rules.GuardCanProtectSelf = true  // 守卫可以自守
	rules.GuardCanRepeat = false      // 守卫不能连续守同一人
	rules.SameGuardKillIsEmpty = true // 同守同杀为空刀

	// 2. 组装一局（日志可选，用构造选项给出）
	eng := werewolf.MustNew(rules,
		engine.WithLogger(&SimpleLogger{}))

	// 3. 添加玩家
	// 6人局配置: 2狼人 + 1女巫 + 1预言家 + 1守卫 + 1村民
	seat(eng, "player1", werewolf.RoleWerewolf)
	seat(eng, "player2", werewolf.RoleWerewolf)
	seat(eng, "player3", werewolf.RoleWitch)
	seat(eng, "player4", werewolf.RoleSeer)
	seat(eng, "player5", werewolf.RoleGuard)
	seat(eng, "player6", werewolf.RoleVillager)

	// 4. 注册事件处理器（可选）
	eng.OnEvent(func(event *engine.Event) {
		fmt.Printf("  [事件] 类型: %s, 目标: %s\n", event.Type, event.TargetID)
	})

	// 5. 开始游戏
	if err := eng.Start(); err != nil {
		log.Fatalf("启动游戏失败: %v", err)
	}

	// 6. 查询当前状态
	fmt.Printf("  当前阶段: %s\n", eng.Phase())
	fmt.Printf("  当前回合: %d\n", eng.Round())

	// 7. 获取阶段信息
	phaseInfo := eng.PhaseInfo()
	fmt.Printf("  需要行动的角色: %v\n", phaseInfo.ActiveRoles)
}

// godNarratorDemo 展示上帝（主持人）如何引导游戏
func godNarratorDemo() {
	fmt.Println("【示例2: 上帝（主持人）引导游戏】")

	eng := werewolf.MustNew(werewolf.DefaultRules())

	// 添加玩家
	seat(eng, "wolf1", werewolf.RoleWerewolf)
	seat(eng, "wolf2", werewolf.RoleWerewolf)
	seat(eng, "witch", werewolf.RoleWitch)
	seat(eng, "seer", werewolf.RoleSeer)
	seat(eng, "guard", werewolf.RoleGuard)
	seat(eng, "villager", werewolf.RoleVillager)

	// 开始游戏
	begin(eng)

	fmt.Println("\n  === 第一夜开始 ===")
	fmt.Printf("  回合: %d\n", eng.Round())

	// 上帝根据阶段信息生成公告
	announcePhase := func() {
		info := eng.PhaseInfo()

		// 检查是否需要上帝公告
		if info.NeedsGodAnnouncement() {
			// 根据阶段类型生成公告内容
			announcement := getGodAnnouncement(info.Phase, info)
			fmt.Printf("\n  [上帝] %s\n", announcement)
		}

		// 显示需要行动的玩家
		for _, role := range info.ActiveRoles {
			if roleInfo, ok := info.RoleInfos[role]; ok {
				fmt.Printf("  → %s 请行动: %v\n", role, roleInfo.PlayerIDs)
				fmt.Printf("    可用技能: %v\n", roleInfo.AllowedSkills)

				// 狼人特殊信息：队友
				if role == werewolf.RoleWerewolf && len(roleInfo.Teammates) > 0 {
					for playerID, mates := range roleInfo.Teammates {
						fmt.Printf("    %s 的狼队友: %v\n", playerID, mates)
					}
				}

				// 女巫特殊信息：被杀目标
				for id, info := range roleInfo.RoleInfo {
					if t := info[werewolf.RoleInfoKillTarget]; t != "" {
						fmt.Printf("    %s 可见的刀口: %s\n", id, t)
					}
				}
			}
		}
	}

	// === 守卫阶段 ===
	announcePhase()
	act(eng, &werewolf.SkillUse{
		PlayerID: "guard",
		Skill:    werewolf.SkillProtect,
		Targets:  []string{"seer"},
	})
	step(eng)

	// === 狼人阶段 ===
	announcePhase()
	act(eng, &werewolf.SkillUse{
		PlayerID: "wolf1",
		Skill:    werewolf.SkillKill,
		Targets:  []string{"villager"},
	})
	act(eng, &werewolf.SkillUse{
		PlayerID: "wolf2",
		Skill:    werewolf.SkillKill,
		Targets:  []string{"villager"},
	})
	step(eng)

	// === 女巫阶段 ===
	announcePhase()
	// 女巫选择不使用药水
	step(eng)

	// === 预言家阶段 ===
	announcePhase()
	act(eng, &werewolf.SkillUse{
		PlayerID: "seer",
		Skill:    werewolf.SkillCheck,
		Targets:  []string{"wolf1"},
	})
	step(eng)

	// === 夜晚结算阶段 ===
	fmt.Printf("\n  [上帝] 夜晚结算中...\n")
	step(eng)

	// === 白天阶段 ===
	fmt.Printf("\n  [上帝] 天亮了！")

	// 宣布昨晚死亡情况
	villagerInfo, _ := eng.PlayerInfo("villager")
	if !villagerInfo.Alive {
		fmt.Printf(" 昨晚 villager 死亡。\n")
	} else {
		fmt.Printf(" 昨晚是平安夜。\n")
	}

	// 白天发言走 SendMessage，不占技能步骤，因此 RoleInfos 里没有对应条目；
	// 存活名单直接问引擎要
	fmt.Printf("  → 所有玩家请发言: %v\n", eng.AlivePlayerIDs())
}

// getGodAnnouncement 根据阶段生成上帝公告
func getGodAnnouncement(phase werewolf.PhaseType, info *engine.PhaseInfo) string {
	switch phase {
	case werewolf.PhaseNightGuard:
		return "天黑请闭眼。守卫请睁眼，请选择今晚要守护的玩家。"
	case werewolf.PhaseNightWolf:
		return "守卫请闭眼。狼人请睁眼，请选择今晚要杀害的玩家。"
	case werewolf.PhaseNightWitch:
		killTarget := ""
		if witchInfo, ok := info.RoleInfos[werewolf.RoleWitch]; ok {
			for _, one := range witchInfo.RoleInfo {
				if t := one[werewolf.RoleInfoKillTarget]; t != "" {
					killTarget = t
				}
			}
		}
		if killTarget != "" {
			return fmt.Sprintf("狼人请闭眼。女巫请睁眼，今晚 %s 被杀害，你要使用解药吗？你要使用毒药吗？", killTarget)
		}
		return "狼人请闭眼。女巫请睁眼，今晚无人被杀害。你要使用毒药吗？"
	case werewolf.PhaseNightSeer:
		return "女巫请闭眼。预言家请睁眼，请选择今晚要查验的玩家。"
	case werewolf.PhaseNightResolve:
		return "预言家请闭眼。"
	case werewolf.PhaseDay:
		return "天亮了，请大家睁眼。"
	case werewolf.PhaseVote:
		return "发言结束，请投票选出你认为的狼人。"
	case werewolf.PhaseNightHunter, werewolf.PhaseDayHunter:
		return "猎人死亡，请选择是否开枪带走一名玩家。"
	default:
		return "请继续游戏。"
	}
}

// fullGameFlow 展示完整游戏流程
func fullGameFlow() {
	fmt.Println("【示例3: 完整游戏流程】")

	// 创建引擎和玩家
	eng := werewolf.MustNew(werewolf.DefaultRules())

	// 添加玩家
	seat(eng, "wolf1", werewolf.RoleWerewolf)
	seat(eng, "wolf2", werewolf.RoleWerewolf)
	seat(eng, "witch", werewolf.RoleWitch)
	seat(eng, "seer", werewolf.RoleSeer)
	seat(eng, "guard", werewolf.RoleGuard)
	seat(eng, "villager", werewolf.RoleVillager)
	// 第二名平民：默认按屠边判定，平民全灭游戏会在夜晚结算就结束，
	// 演示走不到白天
	seat(eng, "villager2", werewolf.RoleVillager)

	// 注册事件处理器
	eng.OnEvent(func(event *engine.Event) {
		switch event.Type {
		case werewolf.EventKill:
			fmt.Printf("  [击杀] %s 被狼人杀死\n", event.TargetID)
		case werewolf.EventSave:
			fmt.Printf("  [救人] %s 被女巫救活\n", event.TargetID)
		case werewolf.EventPoison:
			fmt.Printf("  [毒杀] %s 被女巫毒死\n", event.TargetID)
		case werewolf.EventProtect:
			fmt.Printf("  [保护] %s 被守卫保护\n", event.TargetID)
		case werewolf.EventCheck:
			fmt.Printf("  [查验] %s 查验了 %s\n", event.SourceID, event.TargetID)
		case werewolf.EventEliminate:
			fmt.Printf("  [投票] %s 被投票出局\n", event.TargetID)
		case engine.EventGameEnded:
			fmt.Printf("  [游戏结束] 获胜方: %s\n", event.Data["winner"])
		}
	})

	// 开始游戏
	if err := eng.Start(); err != nil {
		log.Fatalf("启动游戏失败: %v", err)
	}

	fmt.Println("  游戏开始！")

	// ==================== 第一夜 ====================
	fmt.Println("\n  --- 第一夜 ---")

	// 守卫阶段 (NIGHT_GUARD)
	fmt.Printf("  当前阶段: %s\n", eng.Phase())

	// 守卫保护村民
	err := eng.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "guard",
		Skill:    werewolf.SkillProtect,
		// 守的不是狼刀目标：守护叠加女巫解药即「同守同救」，
		// 按默认规则目标依然死亡，与本段想演示的「被解药救回」相反
		Targets: []string{"seer"},
	})
	if err != nil {
		fmt.Printf("  守卫技能提交失败: %v\n", err)
	}

	// 结束守卫阶段，进入狼人阶段
	step(eng)

	// 狼人阶段 (NIGHT_WOLF)
	fmt.Printf("  当前阶段: %s\n", eng.Phase())

	// 狼人获取队友信息
	teammates := eng.Teammates("wolf1")
	fmt.Printf("  wolf1 的狼队友: %v\n", teammates)

	// 两只狼都投票杀村民
	act(eng, &werewolf.SkillUse{
		PlayerID: "wolf1",
		Skill:    werewolf.SkillKill,
		Targets:  []string{"villager"},
	})
	act(eng, &werewolf.SkillUse{
		PlayerID: "wolf2",
		Skill:    werewolf.SkillKill,
		Targets:  []string{"villager"},
	})

	// 结束狼人阶段，进入女巫阶段
	step(eng)

	// 女巫阶段 (NIGHT_WITCH)
	fmt.Printf("  当前阶段: %s\n", eng.Phase())

	// 女巫查看谁被杀
	killTarget := werewolf.NightKillTarget(eng)
	fmt.Printf("  女巫得知: %s 今晚被狼人杀害\n", killTarget)

	// 女巫使用解药救人
	act(eng, &werewolf.SkillUse{
		PlayerID: "witch",
		Skill:    werewolf.SkillAntidote,
		Targets:  []string{killTarget},
	})

	// 结束女巫阶段，进入预言家阶段
	step(eng)

	// 预言家阶段 (NIGHT_SEER)
	fmt.Printf("  当前阶段: %s\n", eng.Phase())

	// 预言家查验 wolf1
	act(eng, &werewolf.SkillUse{
		PlayerID: "seer",
		Skill:    werewolf.SkillCheck,
		Targets:  []string{"wolf1"},
	})

	// 结束预言家阶段，进入夜晚结算
	step(eng)

	// 夜晚结算阶段 (NIGHT_RESOLVE)
	fmt.Printf("  当前阶段: %s\n", eng.Phase())
	step(eng)

	// ==================== 白天 ====================
	fmt.Println("\n  --- 白天 ---")
	fmt.Printf("  当前阶段: %s\n", eng.Phase())

	// 白天发言走消息通道，由引擎按阶段路由给该听到的人
	if err := eng.SendMessage("seer", "我是预言家，wolf1 是狼人！"); err != nil {
		log.Printf("发言失败: %v", err)
	}

	// 结束白天，进入投票
	step(eng)

	// ==================== 投票 ====================
	fmt.Println("\n  --- 投票 ---")
	fmt.Printf("  当前阶段: %s\n", eng.Phase())

	// 所有好人投票 wolf1
	act(eng, &werewolf.SkillUse{
		PlayerID: "witch",
		Skill:    werewolf.SkillVote,
		Targets:  []string{"wolf1"},
	})
	act(eng, &werewolf.SkillUse{
		PlayerID: "seer",
		Skill:    werewolf.SkillVote,
		Targets:  []string{"wolf1"},
	})
	act(eng, &werewolf.SkillUse{
		PlayerID: "guard",
		Skill:    werewolf.SkillVote,
		Targets:  []string{"wolf1"},
	})
	act(eng, &werewolf.SkillUse{
		PlayerID: "villager",
		Skill:    werewolf.SkillVote,
		Targets:  []string{"wolf1"},
	})

	// 狼人投票预言家
	act(eng, &werewolf.SkillUse{
		PlayerID: "wolf1",
		Skill:    werewolf.SkillVote,
		Targets:  []string{"seer"},
	})
	act(eng, &werewolf.SkillUse{
		PlayerID: "wolf2",
		Skill:    werewolf.SkillVote,
		Targets:  []string{"seer"},
	})

	// 结束投票
	step(eng)

	// 检查 wolf1 是否被投票出局
	wolf1Info, _ := eng.PlayerInfo("wolf1")
	fmt.Printf("  wolf1 存活状态: %v\n", wolf1Info.Alive)

	// 检查游戏是否结束
	if eng.IsGameOver() {
		fmt.Println("  游戏已结束！")
	} else {
		fmt.Printf("  游戏继续，当前阶段: %s, 回合: %d\n",
			eng.Phase(), eng.Round())
	}
}

// messagingDemo 展示消息系统
func messagingDemo() {
	fmt.Println("【示例4: 消息系统演示】")

	eng := werewolf.MustNew(werewolf.DefaultRules())

	// 添加玩家（需要足够多的好人防止游戏过早结束）
	seat(eng, "wolf1", werewolf.RoleWerewolf)
	seat(eng, "wolf2", werewolf.RoleWerewolf)
	seat(eng, "villager1", werewolf.RoleVillager)
	seat(eng, "villager2", werewolf.RoleVillager)
	seat(eng, "villager3", werewolf.RoleVillager)
	seat(eng, "villager4", werewolf.RoleVillager)

	// 注册消息处理器
	eng.OnMessage(func(msg *engine.Message, receiverIDs []string) {
		fmt.Printf("  [消息] 发送者: %s, 内容: %s\n", msg.SenderID, msg.Content)
		fmt.Printf("         接收者: %v\n", receiverIDs)
	})

	// 开始游戏
	begin(eng)

	// 进入狼人阶段
	step(eng) // 跳过守卫阶段

	fmt.Printf("\n  当前阶段: %s (狼人交流阶段)\n", eng.Phase())

	// 狼人之间交流（只有狼人能收到）
	err := eng.SendMessage("wolf1", "杀 villager1 吧")
	if err != nil {
		fmt.Printf("  发送消息失败: %v\n", err)
	}

	// 非狼人在狼人阶段发言会失败
	err = eng.SendMessage("villager1", "我想说话")
	if err != nil {
		fmt.Printf("  村民发言失败: %v\n", err)
	}

	// 查看消息接收者
	receivers := eng.MessageReceivers("wolf1")
	fmt.Printf("\n  wolf1 消息可发送给: %v\n", receivers)

	// 跳到白天
	act(eng, &werewolf.SkillUse{
		PlayerID: "wolf1",
		Skill:    werewolf.SkillKill,
		Targets:  []string{"villager1"},
	})
	step(eng) // 狼人阶段结束
	step(eng) // 女巫阶段结束
	step(eng) // 预言家阶段结束
	step(eng) // 夜晚结算结束

	fmt.Printf("\n  当前阶段: %s (白天发言阶段)\n", eng.Phase())

	// 白天所有存活玩家都能收到消息
	err = eng.SendMessage("wolf2", "我是好人")
	if err != nil {
		fmt.Printf("  发送消息失败: %v\n", err)
	}

	receivers = eng.MessageReceivers("wolf2")
	fmt.Printf("\n  白天消息接收者: %v\n", receivers)
}

// SimpleLogger 简单日志实现
type SimpleLogger struct{}

func (l *SimpleLogger) Debug(msg string, fields ...engine.Field) {
	// 调试信息可以忽略或打印
}

func (l *SimpleLogger) Info(msg string, fields ...engine.Field) {
	fmt.Printf("  [INFO] %s\n", msg)
}

func (l *SimpleLogger) Warn(msg string, fields ...engine.Field) {
	fmt.Printf("  [WARN] %s\n", msg)
}

func (l *SimpleLogger) Error(msg string, fields ...engine.Field) {
	fmt.Printf("  [ERROR] %s\n", msg)
}

// ==================== 示例用的小包装 ====================
//
// 示例代码也该示范正确的错误处理。这几个包装让主线保持可读，
// 同时不吞掉任何错误——真实调用方应当按业务需要处理，而不是忽略。

// seat 入座
func seat(e *werewolf.Engine, id string, role werewolf.RoleType) {
	if err := e.AddPlayer(id, role); err != nil {
		log.Fatalf("添加玩家 %s 失败: %v", id, err)
	}
}

// begin 开局
func begin(e *werewolf.Engine) {
	if err := e.Start(); err != nil {
		log.Fatalf("开局失败: %v", err)
	}
}

// step 推进一个阶段
func step(e *werewolf.Engine) []*werewolf.Effect {
	effects, err := e.EndPhase()
	if err != nil {
		log.Fatalf("推进阶段失败: %v", err)
	}
	return effects
}

// act 提交技能
func act(e *werewolf.Engine, use *werewolf.SkillUse) {
	if err := e.SubmitSkillUse(use); err != nil {
		log.Fatalf("提交技能失败 (%s/%v): %v", use.PlayerID, use.Skill, err)
	}
}
