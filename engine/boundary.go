// boundary.go 信息边界：谁能知道什么。
//
// 这是这类游戏最难的部分，也是这个库最值得替调用方收掉的东西。但「难」
// 不等于「该由内核决定」——内核此前认得三件狼人杀的事：
//
//	AudienceOf        查验只给预言家、狼刀全场可见……一张写死的类型表
//	PlayerView.Teammates  同阵营互相可见，而「阵营」只有好人与狼人两值
//	MessageReceivers  夜里只有狼队能说话，白天全体能听
//
// 换一套规则，这三样全都不成立：阿瓦隆的梅林看得到坏人但坏人不知道他是谁，
// 血染钟楼的恶魔与爪牙互见是单向的，谁能在什么时候说话更是每套规则自己的事。
//
// 现在这三个问题都由规则回答，内核只保证一件事：**自己的状态原语永远
// 不外发**。狼人杀的答案在 wolfboundary.go 里，它没有特权，可以整个换掉。

package engine

// AudienceProvider 回答「一件事该告诉哪些玩家」。
//
// 与 Resolver、VictoryChecker 同构：拿只读的 GameView，返回结论，不碰状态。
// 在引擎持锁期间被调用，实现中不要回调 Engine 的任何方法。
//
// 第二个返回值是「认不认得这个事件类型」，它与「不给任何人看」是两件事，
// 必须分得开：前者要求调用方自己路由，后者是明确的判定。返回 false 时
// 第一个返回值会被忽略。
type AudienceProvider interface {
	Audience(event *Event, view GameView) ([]string, bool)
}

// AudienceFunc 让普通函数满足 AudienceProvider。
type AudienceFunc func(event *Event, view GameView) ([]string, bool)

// Audience 实现 AudienceProvider。
func (f AudienceFunc) Audience(event *Event, view GameView) ([]string, bool) {
	return f(event, view)
}

// WithAudience 换掉「一件事该告诉谁」的判定。
//
// 内核的状态原语拦在这之前，永远不会问到这里——它们是状态机的记账，
// 不该出现在任何玩家面前，这一条不可配置。
func WithAudience(provider AudienceProvider) EngineOption {
	return func(e *Engine) error {
		if provider == nil {
			return WrapError(CodeInvalidConfig, "audience provider must not be nil")
		}
		e.audience = provider
		return nil
	}
}

// TeammateProvider 回答「这名玩家被告知谁和他是一边的」。
//
// 返回的 ID 会出现在 PlayerView.Teammates 与 RolePhaseInfo.Teammates 上，
// 并且这些人的身份对他公开。不含自己；返回 nil 表示他不知道任何同伴。
//
// 这个问题**不对称**是允许的：血染钟楼的恶魔认得爪牙，反过来不成立。
// 内核不检查两边是否一致。
type TeammateProvider interface {
	Teammates(playerID string, view GameView) []string
}

// TeammateFunc 让普通函数满足 TeammateProvider。
type TeammateFunc func(playerID string, view GameView) []string

// Teammates 实现 TeammateProvider。
func (f TeammateFunc) Teammates(playerID string, view GameView) []string {
	return f(playerID, view)
}

// WithTeammates 换掉「谁和谁是一边的」的判定。
func WithTeammates(provider TeammateProvider) EngineOption {
	return func(e *Engine) error {
		if provider == nil {
			return WrapError(CodeInvalidConfig, "teammate provider must not be nil")
		}
		e.teammates = provider
		return nil
	}
}

// SpeechProvider 回答「此刻这名玩家说话，谁能听到」。
//
// 返回的列表习惯上包含发送者自己，方便调用方直接拿去广播。
// 返回 nil 表示此刻他说不了话。
type SpeechProvider interface {
	Receivers(senderID string, view GameView) []string
}

// SpeechFunc 让普通函数满足 SpeechProvider。
type SpeechFunc func(senderID string, view GameView) []string

// Receivers 实现 SpeechProvider。
func (f SpeechFunc) Receivers(senderID string, view GameView) []string {
	return f(senderID, view)
}

// WithSpeech 换掉发言的可听范围。
func WithSpeech(provider SpeechProvider) EngineOption {
	return func(e *Engine) error {
		if provider == nil {
			return WrapError(CodeInvalidConfig, "speech provider must not be nil")
		}
		e.speech = provider
		return nil
	}
}

// teammatesOf 算出某个玩家的同伴。调用前需持有 e.mu。
func (e *Engine) teammatesOf(playerID string) []string {
	if e.teammates == nil {
		return nil
	}
	return e.teammates.Teammates(playerID, newStateView(e.state))
}
