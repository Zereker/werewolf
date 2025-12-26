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
	pb "github.com/Zereker/werewolf/proto"
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

	// 1. 创建游戏配置（可选，使用默认配置）
	config := werewolf.DefaultGameConfig()

	// 自定义规则
	config.WitchCanSaveSelf = false    // 女巫不能自救
	config.GuardCanProtectSelf = true  // 守卫可以自守
	config.GuardCanRepeat = false      // 守卫不能连续守同一人
	config.SameGuardKillIsEmpty = true // 同守同杀为空刀

	// 2. 创建游戏引擎
	engine := werewolf.NewEngine(config)

	// 3. 设置日志（可选）
	engine.SetLogger(&SimpleLogger{})

	// 4. 添加玩家
	// 6人局配置: 2狼人 + 1女巫 + 1预言家 + 1守卫 + 1村民
	engine.AddPlayer("player1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("player2", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("player3", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("player4", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("player5", pb.RoleType_ROLE_TYPE_GUARD, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("player6", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	// 5. 注册事件处理器（可选）
	engine.OnEvent(func(event *pb.Event) {
		fmt.Printf("  [事件] 类型: %s, 目标: %s\n", event.Type, event.TargetId)
	})

	// 6. 开始游戏
	if err := engine.Start(); err != nil {
		log.Fatalf("启动游戏失败: %v", err)
	}

	// 7. 查询当前状态
	fmt.Printf("  当前阶段: %s\n", engine.GetCurrentPhase())
	fmt.Printf("  当前回合: %d\n", engine.GetCurrentRound())

	// 8. 获取阶段信息
	phaseInfo := engine.GetPhaseInfo()
	fmt.Printf("  需要行动的角色: %v\n", phaseInfo.ActiveRoles)
}

// godNarratorDemo 展示上帝（主持人）如何引导游戏
func godNarratorDemo() {
	fmt.Println("【示例2: 上帝（主持人）引导游戏】")

	engine := werewolf.NewEngine(nil)

	// 添加玩家
	engine.AddPlayer("wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("witch", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("seer", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("guard", pb.RoleType_ROLE_TYPE_GUARD, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("villager", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	// 开始游戏
	engine.Start()

	fmt.Println("\n  === 第一夜开始 ===")
	fmt.Printf("  回合: %d\n", engine.GetCurrentRound())

	// 上帝根据阶段信息生成公告
	announcePhase := func() {
		info := engine.GetPhaseInfo()

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
				if role == pb.RoleType_ROLE_TYPE_WEREWOLF && len(roleInfo.Teammates) > 0 {
					for playerID, mates := range roleInfo.Teammates {
						fmt.Printf("    %s 的狼队友: %v\n", playerID, mates)
					}
				}

				// 女巫特殊信息：被杀目标
				if role == pb.RoleType_ROLE_TYPE_WITCH && roleInfo.KillTarget != "" {
					fmt.Printf("    今晚被杀: %s\n", roleInfo.KillTarget)
				}
			}
		}
	}

	// === 守卫阶段 ===
	announcePhase()
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "seer",
	})
	engine.EndSubStep()

	// === 狼人阶段 ===
	announcePhase()
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "wolf1",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "villager",
	})
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "wolf2",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "villager",
	})
	engine.EndSubStep()

	// === 女巫阶段 ===
	announcePhase()
	// 女巫选择不使用药水
	engine.EndSubStep()

	// === 预言家阶段 ===
	announcePhase()
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "seer",
		Skill:    pb.SkillType_SKILL_TYPE_CHECK,
		TargetID: "wolf1",
	})
	engine.EndSubStep()

	// === 夜晚结算阶段 ===
	info := engine.GetPhaseInfo()
	fmt.Printf("\n  [上帝] 夜晚结算中...\n")
	engine.EndSubStep()

	// === 白天阶段 ===
	info = engine.GetPhaseInfo()
	fmt.Printf("\n  [上帝] 天亮了！")

	// 宣布昨晚死亡情况
	villagerInfo, _ := engine.GetPlayerInfo("villager")
	if !villagerInfo.Alive {
		fmt.Printf(" 昨晚 villager 死亡。\n")
	} else {
		fmt.Printf(" 昨晚是平安夜。\n")
	}

	fmt.Printf("  → 所有玩家请发言: %v\n", info.RoleInfos[pb.RoleType_ROLE_TYPE_UNSPECIFIED].PlayerIDs)
}

// getGodAnnouncement 根据阶段生成上帝公告
func getGodAnnouncement(phase pb.PhaseType, info *werewolf.PhaseInfo) string {
	switch phase {
	case pb.PhaseType_PHASE_TYPE_NIGHT_GUARD:
		return "天黑请闭眼。守卫请睁眼，请选择今晚要守护的玩家。"
	case pb.PhaseType_PHASE_TYPE_NIGHT_WOLF:
		return "守卫请闭眼。狼人请睁眼，请选择今晚要杀害的玩家。"
	case pb.PhaseType_PHASE_TYPE_NIGHT_WITCH:
		killTarget := ""
		if witchInfo, ok := info.RoleInfos[pb.RoleType_ROLE_TYPE_WITCH]; ok {
			killTarget = witchInfo.KillTarget
		}
		if killTarget != "" {
			return fmt.Sprintf("狼人请闭眼。女巫请睁眼，今晚 %s 被杀害，你要使用解药吗？你要使用毒药吗？", killTarget)
		}
		return "狼人请闭眼。女巫请睁眼，今晚无人被杀害。你要使用毒药吗？"
	case pb.PhaseType_PHASE_TYPE_NIGHT_SEER:
		return "女巫请闭眼。预言家请睁眼，请选择今晚要查验的玩家。"
	case pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE:
		return "预言家请闭眼。"
	case pb.PhaseType_PHASE_TYPE_DAY:
		return "天亮了，请大家睁眼。"
	case pb.PhaseType_PHASE_TYPE_VOTE:
		return "发言结束，请投票选出你认为的狼人。"
	case pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER, pb.PhaseType_PHASE_TYPE_DAY_HUNTER:
		return "猎人死亡，请选择是否开枪带走一名玩家。"
	default:
		return "请继续游戏。"
	}
}

// fullGameFlow 展示完整游戏流程
func fullGameFlow() {
	fmt.Println("【示例3: 完整游戏流程】")

	// 创建引擎和玩家
	engine := werewolf.NewEngine(nil) // 使用默认配置

	// 添加玩家
	engine.AddPlayer("wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("witch", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("seer", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("guard", pb.RoleType_ROLE_TYPE_GUARD, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("villager", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	// 注册事件处理器
	engine.OnEvent(func(event *pb.Event) {
		switch event.Type {
		case pb.EventType_EVENT_TYPE_KILL:
			fmt.Printf("  [击杀] %s 被狼人杀死\n", event.TargetId)
		case pb.EventType_EVENT_TYPE_SAVE:
			fmt.Printf("  [救人] %s 被女巫救活\n", event.TargetId)
		case pb.EventType_EVENT_TYPE_POISON:
			fmt.Printf("  [毒杀] %s 被女巫毒死\n", event.TargetId)
		case pb.EventType_EVENT_TYPE_PROTECT:
			fmt.Printf("  [保护] %s 被守卫保护\n", event.TargetId)
		case pb.EventType_EVENT_TYPE_CHECK:
			fmt.Printf("  [查验] %s 查验了 %s\n", event.SourceId, event.TargetId)
		case pb.EventType_EVENT_TYPE_ELIMINATE:
			fmt.Printf("  [投票] %s 被投票出局\n", event.TargetId)
		case pb.EventType_EVENT_TYPE_GAME_ENDED:
			fmt.Printf("  [游戏结束] 获胜方: %s\n", event.Data["winner"])
		}
	})

	// 开始游戏
	if err := engine.Start(); err != nil {
		log.Fatalf("启动游戏失败: %v", err)
	}

	fmt.Println("  游戏开始！")

	// ==================== 第一夜 ====================
	fmt.Println("\n  --- 第一夜 ---")

	// 守卫阶段 (NIGHT_GUARD)
	fmt.Printf("  当前阶段: %s\n", engine.GetCurrentPhase())

	// 守卫保护村民
	err := engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "villager",
	})
	if err != nil {
		fmt.Printf("  守卫技能提交失败: %v\n", err)
	}

	// 结束守卫阶段，进入狼人阶段
	engine.EndSubStep()

	// 狼人阶段 (NIGHT_WOLF)
	fmt.Printf("  当前阶段: %s\n", engine.GetCurrentPhase())

	// 狼人获取队友信息
	teammates := engine.GetWolfTeammates("wolf1")
	fmt.Printf("  wolf1 的狼队友: %v\n", teammates)

	// 两只狼都投票杀村民
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "wolf1",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "villager",
	})
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "wolf2",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "villager",
	})

	// 结束狼人阶段，进入女巫阶段
	engine.EndSubStep()

	// 女巫阶段 (NIGHT_WITCH)
	fmt.Printf("  当前阶段: %s\n", engine.GetCurrentPhase())

	// 女巫查看谁被杀
	killTarget := engine.GetNightKillTarget()
	fmt.Printf("  女巫得知: %s 今晚被狼人杀害\n", killTarget)

	// 女巫使用解药救人
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "witch",
		Skill:    pb.SkillType_SKILL_TYPE_ANTIDOTE,
		TargetID: killTarget,
	})

	// 结束女巫阶段，进入预言家阶段
	engine.EndSubStep()

	// 预言家阶段 (NIGHT_SEER)
	fmt.Printf("  当前阶段: %s\n", engine.GetCurrentPhase())

	// 预言家查验 wolf1
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "seer",
		Skill:    pb.SkillType_SKILL_TYPE_CHECK,
		TargetID: "wolf1",
	})

	// 结束预言家阶段，进入夜晚结算
	engine.EndSubStep()

	// 夜晚结算阶段 (NIGHT_RESOLVE)
	fmt.Printf("  当前阶段: %s\n", engine.GetCurrentPhase())
	engine.EndSubStep()

	// ==================== 白天 ====================
	fmt.Println("\n  --- 白天 ---")
	fmt.Printf("  当前阶段: %s\n", engine.GetCurrentPhase())

	// 白天发言（技能提交可选）
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "seer",
		Skill:    pb.SkillType_SKILL_TYPE_SPEAK,
		Content:  "我是预言家，wolf1 是狼人！",
	})

	// 结束白天，进入投票
	engine.EndSubStep()

	// ==================== 投票 ====================
	fmt.Println("\n  --- 投票 ---")
	fmt.Printf("  当前阶段: %s\n", engine.GetCurrentPhase())

	// 所有好人投票 wolf1
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "witch",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "wolf1",
	})
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "seer",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "wolf1",
	})
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "wolf1",
	})
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "villager",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "wolf1",
	})

	// 狼人投票预言家
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "wolf1",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "seer",
	})
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "wolf2",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "seer",
	})

	// 结束投票
	engine.EndSubStep()

	// 检查 wolf1 是否被投票出局
	wolf1Info, _ := engine.GetPlayerInfo("wolf1")
	fmt.Printf("  wolf1 存活状态: %v\n", wolf1Info.Alive)

	// 检查游戏是否结束
	if engine.IsGameOver() {
		fmt.Println("  游戏已结束！")
	} else {
		fmt.Printf("  游戏继续，当前阶段: %s, 回合: %d\n",
			engine.GetCurrentPhase(), engine.GetCurrentRound())
	}
}

// messagingDemo 展示消息系统
func messagingDemo() {
	fmt.Println("【示例4: 消息系统演示】")

	engine := werewolf.NewEngine(nil)

	// 添加玩家（需要足够多的好人防止游戏过早结束）
	engine.AddPlayer("wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("villager1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("villager2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("villager3", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("villager4", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	// 注册消息处理器
	engine.OnMessage(func(msg *werewolf.Message, receiverIDs []string) {
		fmt.Printf("  [消息] 发送者: %s, 内容: %s\n", msg.SenderID, msg.Content)
		fmt.Printf("         接收者: %v\n", receiverIDs)
	})

	// 开始游戏
	engine.Start()

	// 进入狼人阶段
	engine.EndSubStep() // 跳过守卫阶段

	fmt.Printf("\n  当前阶段: %s (狼人交流阶段)\n", engine.GetCurrentPhase())

	// 狼人之间交流（只有狼人能收到）
	err := engine.SendMessage("wolf1", "杀 villager1 吧")
	if err != nil {
		fmt.Printf("  发送消息失败: %v\n", err)
	}

	// 非狼人在狼人阶段发言会失败
	err = engine.SendMessage("villager1", "我想说话")
	if err != nil {
		fmt.Printf("  村民发言失败: %v\n", err)
	}

	// 查看消息接收者
	receivers := engine.GetMessageReceivers("wolf1")
	fmt.Printf("\n  wolf1 消息可发送给: %v\n", receivers)

	// 跳到白天
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "wolf1",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "villager1",
	})
	engine.EndSubStep() // 狼人阶段结束
	engine.EndSubStep() // 女巫阶段结束
	engine.EndSubStep() // 预言家阶段结束
	engine.EndSubStep() // 夜晚结算结束

	fmt.Printf("\n  当前阶段: %s (白天发言阶段)\n", engine.GetCurrentPhase())

	// 白天所有存活玩家都能收到消息
	err = engine.SendMessage("wolf2", "我是好人")
	if err != nil {
		fmt.Printf("  发送消息失败: %v\n", err)
	}

	receivers = engine.GetMessageReceivers("wolf2")
	fmt.Printf("\n  白天消息接收者: %v\n", receivers)
}

// SimpleLogger 简单日志实现
type SimpleLogger struct{}

func (l *SimpleLogger) Debug(msg string, fields ...werewolf.Field) {
	// 调试信息可以忽略或打印
}

func (l *SimpleLogger) Info(msg string, fields ...werewolf.Field) {
	fmt.Printf("  [INFO] %s\n", msg)
}

func (l *SimpleLogger) Warn(msg string, fields ...werewolf.Field) {
	fmt.Printf("  [WARN] %s\n", msg)
}

func (l *SimpleLogger) Error(msg string, fields ...werewolf.Field) {
	fmt.Printf("  [ERROR] %s\n", msg)
}
