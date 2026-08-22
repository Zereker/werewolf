// rules.go 狼人杀的规则开关。
//
// 这几项此前住在 GameConfig 里，与阶段机的配置（起始阶段、阶段图、超时）
// 混在同一个结构体上，而 Resolver.Resolve 每次结算都把整个 GameConfig
// 递进去。于是内核认得「女巫能不能自救」这种事，第三方解析器也得从一个
// 自己看不懂的结构体里挑字段。
//
// 现在分成两半：阶段机的配置是内核的（GameConfig），规则开关是狼人杀的
// （Rules），解析器在**构造时**拿到它。解析器的配置是「它是什么」的一部分，
// 本来就该在构造时定死，而不是每次结算重新递一遍。

package werewolf

// Rules 狼人杀的规则变体。
//
// 桌面上有分歧的规则做成开关，而不是替使用者选一个：同守同救到底死不死、
// 女巫能不能自救，各地打法不同，引擎没有资格替谁做主。
type Rules struct {
	WitchCanSaveSelf       bool // 女巫能否自救
	WitchCanUseBothPotions bool // 女巫能否在同一夜同时使用解药和毒药
	GuardCanProtectSelf    bool // 守卫能否自守
	GuardCanRepeat         bool // 守卫能否连续守同一人
	SameGuardKillIsEmpty   bool // 守卫守住刀口时是否空刀（守护是否生效）
	GuardSaveTogetherDies  bool // 同守同救（守卫守护 + 女巫解药）目标是否依然死亡

	// VictoryMode 胜负判定方式：屠边或屠城。
	// 换掉 VictoryChecker 之后这一项就不再起作用——它只喂给内置判定。
	VictoryMode VictoryMode
}

// DefaultRules 默认规则：以维基百科「狼人殺」条目为基准的那一套。
func DefaultRules() Rules {
	return Rules{
		WitchCanSaveSelf:       false,
		WitchCanUseBothPotions: false,
		GuardCanProtectSelf:    true,
		GuardCanRepeat:         false,
		SameGuardKillIsEmpty:   true,
		GuardSaveTogetherDies:  true,
		VictoryMode:            VictoryModeSideWipe,
	}
}

// Validate 校验规则开关。
func (r Rules) Validate() error {
	if r.VictoryMode < VictoryModeSideWipe || r.VictoryMode > VictoryModeTownWipe {
		return WrapError(CodeInvalidConfig, "unknown victory mode %d", int(r.VictoryMode))
	}
	return nil
}

// builtinResolvers 内置阶段的解析器。
//
// 做成表而不是一串赋值：加内置阶段时只需要在这里加一行，也让
// 「哪些阶段有解析器」一眼可见。第三方阶段经 WithResolver 注册。
//
// 表是规则的一部分，不是内核的：内核不知道 NIGHT_WITCH 该由谁结算。
func builtinResolvers(rules Rules) map[PhaseType]Resolver {
	return map[PhaseType]Resolver{
		PhaseDay:          NewDayResolver(),
		PhaseVote:         NewVoteResolver(),
		PhaseNightGuard:   NewGuardResolver(rules),
		PhaseNightWolf:    NewWolfResolver(),
		PhaseNightWitch:   NewWitchResolver(rules),
		PhaseNightSeer:    NewSeerResolver(),
		PhaseNightResolve: NewNightResolveResolver(rules),
		PhaseNightHunter:  NewHunterResolver(),
		PhaseDayHunter:    NewHunterResolver(),
	}
}

// Options 把这套规则组装成引擎构造选项。
//
// 这是「内核可复用」的证明，也是狼人杀这个规则包的全部内容：九个阶段、
// 七个解析器、胜负判定、受众/队友/发言三个 provider、六个角色的初始状态
// 与专属信息，**全部经公开选项传给内核**。内核里没有一处认得它们。
//
// 想在默认之上改一处，把返回值接着往下传即可：
//
//	engine, _ := werewolf.NewEngine(werewolf.DefaultGameConfig(),
//		append(werewolf.Options(rules), werewolf.WithResolver(myPhase, myResolver))...)
func Options(rules Rules) []EngineOption {
	opts := []EngineOption{
		WithVictoryChecker(DefaultVictoryChecker{Mode: rules.VictoryMode}),
		WithAudience(builtinAudience),
		WithTeammates(builtinTeammates),
		WithSpeech(builtinSpeech),
	}
	for phase, r := range builtinResolvers(rules) {
		opts = append(opts, WithResolver(phase, r))
	}
	for role, p := range builtinRoleInfo {
		opts = append(opts, WithRoleInfo(role, p))
	}
	for role, su := range builtinRoleSetup {
		opts = append(opts, WithRoleSetup(role, su))
	}
	return opts
}

// New 按给定规则组装一局狼人杀。
//
// 这是这个规则包的门。内核的 NewEngine 装的是一台什么都不认识的状态机；
// 狼人杀的一切由 Options 作为选项交给它。
//
// extra 里的选项排在后面，因此可以覆盖默认的任何一项——换掉女巫的解析器、
// 换掉胜负判定、给自定义角色登记初始状态，都在这里。
func New(rules Rules, extra ...EngineOption) (*Engine, error) {
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	return NewEngine(DefaultGameConfig(), append(Options(rules), extra...)...)
}

// NewWith 同 New，但使用给定的阶段配置。
//
// 加自定义阶段时用它：先 DefaultGameConfig() 拿到默认阶段图，改完再传进来。
func NewWith(config *GameConfig, rules Rules, extra ...EngineOption) (*Engine, error) {
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	return NewEngine(config, append(Options(rules), extra...)...)
}

// MustNewWith 同 NewWith，配置不合法时 panic。
func MustNewWith(config *GameConfig, rules Rules, extra ...EngineOption) *Engine {
	e, err := NewWith(config, rules, extra...)
	if err != nil {
		panic("werewolf: " + err.Error())
	}
	return e
}

// MustNew 同 New，配置不合法时 panic。
//
// 适用于配置是编译期常量的场合（示例、测试、写死默认配置的服务启动路径）。
func MustNew(rules Rules, extra ...EngineOption) *Engine {
	e, err := New(rules, extra...)
	if err != nil {
		panic("werewolf: " + err.Error())
	}
	return e
}

// Restore 按快照重建一局狼人杀。
//
// 与 New 同理：内核的 RestoreEngine 造出来的是一台什么都不认识的状态机，
// 规则经 Options 交给它。config 为 nil 时用默认阶段配置。
//
// 快照存的是**状态**不是规则——同一份存档配上不同的 Rules 会得到不同的
// 后续判定，这是刻意的：规则的版本由调用方掌握。
func Restore(config *GameConfig, rules Rules, snap *Snapshot, extra ...EngineOption) (*Engine, error) {
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	if config == nil {
		config = DefaultGameConfig()
	}
	return RestoreEngine(config, snap, append(Options(rules), extra...)...)
}

// Replay 按效果流重建一局狼人杀。config 为 nil 时用默认阶段配置。
func Replay(config *GameConfig, rules Rules, log []*Effect, extra ...EngineOption) (*Engine, error) {
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	if config == nil {
		config = DefaultGameConfig()
	}
	return ReplayEngine(config, log, append(Options(rules), extra...)...)
}
