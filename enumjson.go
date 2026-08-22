// enumjson.go 枚举的 JSON 形态：输出名字而不是编号。
//
// 此前枚举按编号序列化，于是任何客户端都得自己维护一张
// 「2 是狼人、21 是守卫阶段」的对照表——example/netserver 推给客户端的
// 就是 {"role":2,"phase":21}，一眼看不出是什么。编号是给存储用的，
// 不该是给人和跨语言客户端用的。
//
// # 自定义取值怎么办
//
// 这个库支持第三方角色，取值从 1000 起，它们没有名字。只输出名字的话
// 那些取值就往返不回来了。所以规则是：**已知取值输出名字，自定义取值
// 输出编号，读的时候两种都收**。
//
//	werewolf.RoleWerewolf   -> "WEREWOLF"
//	werewolf.RoleType(1000) -> 1000

package werewolf

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// enum 是本包所有枚举的共同底型。
type enum interface {
	~int32 | ~int
}

// marshalEnum 已知取值输出名字，其余输出编号。
func marshalEnum[T enum](v T, names map[T]string) ([]byte, error) {
	if name, ok := names[v]; ok {
		return json.Marshal(name)
	}
	return json.Marshal(int64(v))
}

// unmarshalEnum 名字与编号都接受。
//
// 名字不认识时直接报错而不是退回零值：零值在这个库里大多是
// UNSPECIFIED，静默变成它比报错更难查。
func unmarshalEnum[T enum](data []byte, typeName string, names map[T]string) (T, error) {
	var zero T

	// 先按编号试。自定义取值走的是这条。
	var n int64
	if err := json.Unmarshal(data, &n); err == nil {
		return T(n), nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return zero, fmt.Errorf("%s 必须是名字或编号，实际 %s", typeName, data)
	}
	for v, name := range names {
		if name == s {
			return v, nil
		}
	}
	// 形如 "PhaseType(999)" 的兜底输出也认，让 String 与 JSON 闭环
	var num int
	if _, err := fmt.Sscanf(s, typeName+"(%d)", &num); err == nil {
		return T(num), nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return T(n), nil
	}
	return zero, fmt.Errorf("%s 不认识的取值 %q", typeName, s)
}

func (v PhaseType) MarshalJSON() ([]byte, error) { return marshalEnum(v, phaseTypeNames) }
func (v *PhaseType) UnmarshalJSON(b []byte) error {
	x, err := unmarshalEnum(b, "PhaseType", phaseTypeNames)
	*v = x
	return err
}

func (v Camp) MarshalJSON() ([]byte, error) { return marshalEnum(v, campNames) }
func (v *Camp) UnmarshalJSON(b []byte) error {
	x, err := unmarshalEnum(b, "Camp", campNames)
	*v = x
	return err
}

func (v RoleType) MarshalJSON() ([]byte, error) { return marshalEnum(v, roleTypeNames) }
func (v *RoleType) UnmarshalJSON(b []byte) error {
	x, err := unmarshalEnum(b, "RoleType", roleTypeNames)
	*v = x
	return err
}

func (v SkillType) MarshalJSON() ([]byte, error) { return marshalEnum(v, skillTypeNames) }
func (v *SkillType) UnmarshalJSON(b []byte) error {
	x, err := unmarshalEnum(b, "SkillType", skillTypeNames)
	*v = x
	return err
}

func (v EventType) MarshalJSON() ([]byte, error) { return marshalEnum(v, eventTypeNames) }
func (v *EventType) UnmarshalJSON(b []byte) error {
	x, err := unmarshalEnum(b, "EventType", eventTypeNames)
	*v = x
	return err
}

func (v ErrorCode) MarshalJSON() ([]byte, error) { return marshalEnum(v, errorCodeNames) }
func (v *ErrorCode) UnmarshalJSON(b []byte) error {
	x, err := unmarshalEnum(b, "ErrorCode", errorCodeNames)
	*v = x
	return err
}

// roleCategoryNames 角色类别的名字。与其他枚举不同，它的 String 是手写的
// switch，这里补一张表让 JSON 走同一套路径。
var roleCategoryNames = map[RoleCategory]string{
	RoleCategoryUnknown:  "UNKNOWN",
	RoleCategoryWolf:     "WOLF",
	RoleCategoryGod:      "GOD",
	RoleCategoryVillager: "VILLAGER",
}

func (c RoleCategory) MarshalJSON() ([]byte, error) { return marshalEnum(c, roleCategoryNames) }
func (c *RoleCategory) UnmarshalJSON(b []byte) error {
	x, err := unmarshalEnum(b, "RoleCategory", roleCategoryNames)
	*c = x
	return err
}
