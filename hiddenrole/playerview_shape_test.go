package hiddenrole

import (
	"reflect"
	"strings"
	"testing"
)

// deliberateProjections 面向玩家的结构里，允许出现的自由格式状态。
//
// 只有一条：PlayerView.RoleInfo。它是角色**显式**投射出来的东西
// （见 RoleInfoProvider），投什么由角色自己决定，因此是一次有意的公开。
var deliberateProjections = map[string]bool{
	"PlayerView.RoleInfo": true,
}

// TestPlayerView_CarriesNoFreeFormState 面向玩家的结构里不许有自由格式的状态口袋。
//
// 「往里放什么由角色决定，默认把它交给玩家等于让每个角色自己去想『这一项能不能
// 给他看』」——这条写在 PlayerInfo.Vars 的注释里，是三副面孔（上帝视角的
// PlayerInfo、自己那份 SelfInfo、别人那份 PublicPlayerInfo）分开的全部理由。
//
// 而它此前**只是一句注释**：谁给 SelfInfo 或 PublicPlayerInfo 加一个
// `Vars map[string]string`，女巫还剩几瓶药、守卫上回合守了谁就一起发给全场了，
// 没有任何东西会响。这个测试把 PlayerView 的类型图整个走一遍，
// 任何 map[string]string（RoleInfo 那一个例外）都当作泄漏。
//
// 它不检查值，只检查形状：形状对了，泄漏就得是有人**显式**填进去的，
// 而那一步已经被 player_view_test.go 那批测试盯着了。
func TestPlayerView_CarriesNoFreeFormState(t *testing.T) {
	var leaks []string
	walkFields(reflect.TypeOf(PlayerView{}), "PlayerView", map[reflect.Type]bool{}, func(path string, f reflect.StructField) {
		if deliberateProjections[path] {
			return
		}
		if isFreeFormState(f.Type) {
			leaks = append(leaks, path+" "+f.Type.String())
		}
	})

	if len(leaks) > 0 {
		t.Errorf("面向玩家的结构里出现了自由格式的状态口袋：\n  %s\n"+
			"这类字段的内容由规则自己定，内核无从判断哪一项该给玩家看。"+
			"要给玩家的东西请走 RoleInfoProvider 显式投射；"+
			"确实是一次有意的公开，就往 deliberateProjections 里加一行并说明理由。",
			strings.Join(leaks, "\n  "))
	}
}

// TestPlayerView_ShapeTestActuallyWalks 这个测试自己得真的走到过东西。
//
// 反射走类型图很容易因为一个提前 return 而什么都没查，然后永远是绿的。
func TestPlayerView_ShapeTestActuallyWalks(t *testing.T) {
	seen := map[string]bool{}
	walkFields(reflect.TypeOf(PlayerView{}), "PlayerView", map[reflect.Type]bool{}, func(path string, _ reflect.StructField) {
		seen[path] = true
	})

	for _, want := range []string{
		"PlayerView.RoleInfo",      // 允许的那一个
		"PlayerView.Self.Camp",     // 嵌套一层
		"PlayerView.Players.Role",  // 穿过切片
		"PlayerView.AllowedSkills", // 切片本身
	} {
		if !seen[want] {
			t.Errorf("类型图没走到 %s——这个形状测试可能什么都没查", want)
		}
	}
}

// walkFields 递归走一个结构体的字段，对每个字段调用 visit。
// path 形如 PlayerView.Self.Camp，穿过切片与指针时不额外加标记。
func walkFields(t reflect.Type, path string, seen map[reflect.Type]bool, visit func(string, reflect.StructField)) {
	t = deref(t)
	if t.Kind() != reflect.Struct || seen[t] {
		return
	}
	seen[t] = true
	defer delete(seen, t)

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue // 未导出字段出不了包，不在边界上
		}
		sub := path + "." + f.Name
		visit(sub, f)
		walkFields(f.Type, sub, seen, visit)
	}
}

// deref 剥掉指针与切片，拿到底下的元素类型。
func deref(t reflect.Type) reflect.Type {
	for {
		switch t.Kind() {
		case reflect.Ptr, reflect.Slice, reflect.Array:
			t = t.Elem()
		default:
			return t
		}
	}
}

// isFreeFormState 这个类型是不是一个「规则爱放什么放什么」的口袋。
func isFreeFormState(t reflect.Type) bool {
	t = deref(t)
	return t.Kind() == reflect.Map
}
