// roleinfo.go 狼人杀的角色专属信息。
//
// 此前它们是引擎里一个认得所有内置角色的 switch：狼人给队友、女巫给刀口，
// 别的角色什么都没有。第三方角色因此发不出任何专属信息——加一个盗贼就得
// 改引擎，这不合理。内置女巫现在走的也是这条路，它没有特权。

package werewolf

// 女巫专属信息在 RoleInfo 里的键名。
//
// RoleInfoAntidote / RoleInfoPoison 是药剂存量的**投射**，存储在
// Vars 里（VarWitchAntidote / VarWitchPoison）。存储与投射分开是刻意的：
// 存储只有 Vars 一种，谁都能写；要给玩家看成什么样由角色自己决定。
// 它们此前是 SelfInfo 上两个具名 bool 字段，等于内置女巫在面向玩家的
// 视图上比第三方角色多一等公民的待遇。
const (
	RoleInfoKillTarget = "kill_target"
	RoleInfoAntidote   = "antidote"
	RoleInfoPoison     = "poison"
)

// builtinRoleInfo 内置角色的专属信息。
//
// 做成表而不是 switch：加内置角色时只需在这里加一行，第三方经
// WithRoleInfo 注册的走的是同一张表、同一条读取路径，没有先后之分。
var builtinRoleInfo = map[RoleType]RoleInfoProvider{
	RoleWitch: RoleInfoFunc(builtinWitchInfo),
}

// builtinWitchInfo 女巫看得到自己的药剂存量，以及解药尚在手时的刀口。
//
// 刀口按规则「解藥未使用時可以得知狼人的殺害對象」给。已出局的女巫
// 不再是行动者，天亮公布之前不该拿到今晚的刀口——但药剂存量照给：
// 那是她自己的东西，死了也不该凭空看不见。
func builtinWitchInfo(playerID string, view GameView) map[string]string {
	self, ok := view.Player(playerID)
	if !ok {
		return nil
	}

	info := make(map[string]string, 3)
	if v := view.PlayerVar(playerID, VarWitchAntidote); v != "" {
		info[RoleInfoAntidote] = v
	}
	if v := view.PlayerVar(playerID, VarWitchPoison); v != "" {
		info[RoleInfoPoison] = v
	}

	if self.Alive && info[RoleInfoAntidote] != "" {
		if target := nightKillTarget(view); target != "" {
			info[RoleInfoKillTarget] = target
		}
	}

	if len(info) == 0 {
		return nil
	}
	return info
}
