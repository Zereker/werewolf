// rules.go 把一夜狼人装进内核。
//
// 这个文件是「加一套规则不该改引擎」这条标准的直接检验：下面每一个入口都是
// 内核的公开构造选项，与第三方注册自定义角色走的是同一批门。内核为这一套
// 规则**一行都没改**。

package onenight

import (
	"github.com/Zereker/werewolf/engine"
)

// MinPlayers 最少人数。规则要求发牌比人数多三张，三人局是最小的一桌。
const MinPlayers = 3

// Options 一夜狼人的全套装配。
//
// center 是留在中央的三张牌——由调用方发牌时定下。发牌发生在**建局之前**，
// 与前两套规则包一致：内核里没有随机，也不需要有。
//
//	e := engine.MustNewEngine(onenight.GameConfig(),
//		onenight.Options([3]engine.RoleType{...})...)
func Options(center [CenterCount]engine.RoleType) []engine.EngineOption {
	opts := []engine.EngineOption{
		// 十个阶段各自的结算。
		engine.WithResolver(PhaseNightWerewolf, werewolfResolver{}),
		engine.WithResolver(PhaseNightMinion, noopResolver{}),
		engine.WithResolver(PhaseNightMason, noopResolver{}),
		engine.WithResolver(PhaseNightSeer, seerResolver{}),
		engine.WithResolver(PhaseNightRobber, robberResolver{}),
		engine.WithResolver(PhaseNightTroublemake, troublemakerResolver{}),
		engine.WithResolver(PhaseNightDrunk, drunkResolver{}),
		engine.WithResolver(PhaseNightInsomniac, insomniacResolver{}),
		engine.WithResolver(PhaseDay, noopResolver{}),
		engine.WithResolver(PhaseVote, voteResolver{}),

		engine.WithVictoryChecker(engine.VictoryFunc(checkVictory)),

		// 开局那一刻把三张中央牌铺好。
		engine.WithGameSetup(engine.GameSetupFunc(func(engine.GameView) []*engine.Effect {
			out := make([]*engine.Effect, 0, CenterCount)
			for i, role := range center {
				out = append(out, setCenterCard(i, role))
			}
			return out
		})),

		engine.WithAudience(audience()),
		engine.WithTeammates(teammates()),
		engine.WithSpeech(speech()),
	}

	// 每个角色入座时都带着「我现在手上是这张牌」——起始值等于发到手的那张。
	//
	// **不写 VarCamp**。内核会把它搬进 SelfInfo.Camp，而这一套规则里
	// 「我现在算哪边」是秘密：酒鬼把自己的牌与中央换掉且不看，被捣蛋鬼换过
	// 的两个人也不知道。填进他自己的视图等于直接告诉他。见 SCARS.md 疤 4。
	for _, role := range AllRoles {
		r := role
		opts = append(opts,
			engine.WithRoleSetup(r, engine.RoleSetupFunc(
				func(_ string, dealt engine.RoleType) map[string]string {
					return map[string]string{varCard: string(dealt)}
				})),
			engine.WithRoleInfo(r, roleInfoFor(r)),
		)
	}
	return opts
}

// AllRoles 这一套规则的全部角色。
//
// 内核不知道有哪些角色——它只在 AddPlayer 时收下一个 RoleType 字符串。
// 这份清单是本包自己的。
var AllRoles = []engine.RoleType{
	RoleWerewolf, RoleMinion, RoleMason, RoleSeer, RoleRobber,
	RoleTroublemaker, RoleDrunk, RoleInsomniac, RoleVillager,
	RoleHunter, RoleTanner,
}
