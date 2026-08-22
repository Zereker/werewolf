package werewolf

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "重写 testdata 下的快照基准文件")

// TestSnapshot_ShapeIsPinnedToVersion 快照的序列化形状与 SnapshotVersion 绑在一起。
//
// SnapshotVersion 的规则是「对快照结构做出不向后兼容的改动时递增」，
// 而 RestoreEngine 用的是精确相等：版本对不上就拒绝。这套机制有一个缺口——
// 改了结构却**忘了**递增，旧存档会被按新结构读出一个看似正常、实则错乱的
// 局面，没有任何东西会报警。这正是版本号想防的事。
//
// 这个测试把形状钉死：任何字段的增删改名、任何枚举序列化方式的变动，
// 都会让它变红。红了之后按这个顺序判断：
//
//   - 旧存档还读得对吗？读得对（比如只是加了个可选字段）——
//     不必动 SnapshotVersion，跑 -update-golden 更新基准即可。
//   - 读不对——递增 SnapshotVersion，再跑 -update-golden，
//     并在 CHANGELOG 里写明旧存档失效。
func TestSnapshot_ShapeIsPinnedToVersion(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), seer("s"), witch("wi"), guard("g"), hunter("h"),
		villagers("v1", "v2", "v3"),
	)...)

	// 走出一个各种作用域都非空的局面：玩家变量、回合变量、玩家回合标记
	g.mustUse("g", SkillProtect, "s")
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.mustUse("w2", SkillKill, "v1")
	g.end(PhaseNightWitch)
	g.mustUse("wi", SkillAntidote, "v1")
	g.end(PhaseNightSeer)
	g.mustUse("s", SkillCheck, "w1")

	data, err := json.MarshalIndent(g.e.Snapshot(), "", "  ")
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	data = append(data, '\n')

	path := filepath.Join("testdata", "snapshot_shape.json")
	if *updateGolden {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("写基准文件: %v", err)
		}
		t.Logf("已更新 %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读基准文件失败（首次生成请跑 go test -run %s -update-golden .）: %v",
			t.Name(), err)
	}
	if string(data) != string(want) {
		t.Errorf("快照形状变了。\n--- 基准 ---\n%s\n--- 现在 ---\n%s\n"+
			"旧存档还读得对就跑 -update-golden；读不对就先递增 SnapshotVersion。",
			want, data)
	}
}
