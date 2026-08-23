package hiddenrole

import (
	"sync"
)

type Engine struct {
	mu sync.RWMutex

	config *Config
	state  *gameState
	phase  *phaseManager

	// logger is fixed at construction and never changes afterwards, so it can
	// be read outside the lock. It used to have a setter of its own, which
	// meant every use outside the lock had to copy it under the lock first;
	// folding it into the construction options made that defence unnecessary.
	logger Logger

	// victory decides the winner. The kernel's default is "never ends" -- it
	// does not know what winning means; a rules package installs its own with
	// WithVictoryChecker.
	victory VictoryChecker

	// roleInfo holds each role's information provider. Built-in and
	// third-party registrations live in one table and are read along one
	// path -- a built-in role holds no privilege here.
	roleInfo map[RoleType]RoleInfoProvider

	// roleSetup holds each role's initial state. As above: the witch's two
	// starting potions and whatever a third-party role starts with go through
	// the same table and the same write path.
	roleSetup map[RoleType]RoleSetup

	// gameSetup is the initialisation at the moment play begins. It pairs
	// with roleSetup: that one covers what one player sits down with, this
	// one covers the board at the start of the whole game -- initialising
	// game-long counters, and naming the first phase's actors (the latter is
	// the direct reason it exists: the actor set is normally computed by the
	// previous phase's resolver, and the first phase has no previous phase).
	gameSetup GameSetup

	// The three questions of the information boundary, all answered by the
	// rules (see boundary.go): who should be told about something, who is on
	// whose side, and who hears a player speak. The kernel guarantees only
	// that its own state primitives never leave the building.
	audience  AudienceProvider
	teammates TeammateProvider
	speech    SpeechProvider

	// winner is who won, or CampUnspecified while the game is undecided.
	//
	// It is recorded rather than recomputed on demand: the checker is
	// replaceable, and "who won this game" should be a fact settled at the
	// moment it ended, not something that changes because someone swapped the
	// checker afterwards.
	winner Camp

	// The skill uses collected in the current phase.
	pendingUses []*SkillUse

	// The complete effect log since the game was created; append-only.
	effectLog []*Effect

	// Event notification (optional).
	eventHandlers []EventHandler

	// Message notification (optional).
	messageHandlers []MessageHandler
}

// NewEngine creates a game engine.
//
// What comes out is a state machine that **recognises nothing**: no
// resolvers, no victory check, no audience rules. Every rule arrives through
// opts -- werewolf's whole set is werewolf.New, which is assembled exactly
// this way and takes no back door.
//
// config is required: the kernel has no default board to offer. It is checked
// by Config.Validate first -- the phase graph is data the user can replace,
// and a dangling NextPhase makes the game end silently halfway through, a
// class of problem that has to surface at construction.
func NewEngine(config *Config, opts ...EngineOption) (*Engine, error) {
	if config == nil {
		return nil, WrapError(CodeInvalidConfig, "config must not be nil")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	e := &Engine{
		config:          config,
		state:           newState(),
		phase:           newPhaseManager(config),
		logger:          newNopLogger(),
		victory:         neverEnds{},
		roleInfo:        make(map[RoleType]RoleInfoProvider, 4),
		roleSetup:       make(map[RoleType]RoleSetup, 8),
		pendingUses:     make([]*SkillUse, 0),
		effectLog:       make([]*Effect, 0),
		eventHandlers:   make([]EventHandler, 0),
		messageHandlers: make([]MessageHandler, 0),
	}
	if err := e.applyOptions(opts); err != nil {
		return nil, err
	}
	return e, nil
}

// MustNewEngine is NewEngine, panicking on an invalid configuration.
//
// For cases where the configuration is a compile-time constant: examples,
// tests, and service start-up paths with a hard-coded default.
func MustNewEngine(config *Config, opts ...EngineOption) *Engine {
	engine, err := NewEngine(config, opts...)
	if err != nil {
		panic("werewolf: invalid game config: " + err.Error())
	}
	return engine
}

// AddPlayer seats one player.
//
// It may only be called before Start. Errors: the game has already started;
// an empty ID; an ID already taken; a role that cannot be assigned to a
// player.
//
// Camp and role category are **not parameters**: they are the rules' way of
// dividing things up, handed out as initial state by that role's RoleSetup at
// seating time (see WithRoleSetup). There used to be an overload here taking
// two more parameters, so that an extension role could state its camp and
// category explicitly -- which made the answer to "which side is this role
// on" depend on the caller filling it in correctly at every seating, rather
// than being written on the role itself.
func (e *Engine) AddPlayer(id string, role RoleType) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Changing the players after the start would desynchronise the role
	// information already sent out from the engine's state.
	if e.state.Phase != PhaseStart {
		return ErrGameAlreadyStarted
	}

	vars := e.setupFor(id, role)
	if err := e.seatPlayer(id, role, vars); err != nil {
		return err
	}
	e.recordEffects(newPlayerAddedEffect(id, role, vars))
	return nil
}

// seatPlayer seats one player carrying the given initial state. The caller
// must hold e.mu.
//
// Normal seating and replay seating share this path; the only difference is
// where vars comes from: normal seating asks RoleSetup, replay reads what the
// effect log recorded.
//
// Recording the initial state in the effect log, rather than asking RoleSetup
// again on replay, is deliberate: "the witch sat down with two potions" is
// something that happened, and an effect log records what happened. Were it
// asked again, a replayer who forgot one WithRoleSetup would rebuild that
// role quietly empty-handed -- a missing resolver is caught by
// validateResolvers, and this cannot be, because "this role has no initial
// state" and "you forgot to pass it" are indistinguishable in the signature.
func (e *Engine) seatPlayer(id string, role RoleType, vars map[string]string) error {
	if err := e.state.addPlayer(id, role); err != nil {
		return err
	}
	e.state.setPlayerVars(id, vars)
	return nil
}

// Start begins the game.
//
// The start event is pushed to OnEvent subscribers through the same channel
// as every other event.
func (e *Engine) Start() error {
	effect, handlers, err := e.startLocked()
	if err != nil {
		return err
	}

	// Dispatch outside the lock: a callback may call back into the Engine.
	dispatchEvent(handlers, e.logger, effect.ToEvent())
	return nil
}

// startLocked does the start under the lock and returns what has to be
// published outside it.
func (e *Engine) startLocked() (*Effect, []EventHandler, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != PhaseStart {
		return nil, nil, ErrGameAlreadyStarted
	}

	// Check the board: a setup that is already decided before play begins is
	// better rejected here than allowed to "start and immediately end".
	//
	// This used to read "there must be werewolves and there must be
	// villagers" -- werewolf's phrasing, and the kernel does not know about
	// camps. It now asks the victory checker instead: since that is the sole
	// authority on "is this decided yet", asking it once before the start is
	// enough, and it incidentally covers a case the old check missed (2
	// wolves against 2 villagers in wipe-out mode, where the wolves win on
	// the first resolution).
	if over, winner := e.victory.CheckVictory(newStateView(e.state)); over {
		return nil, nil, WrapError(CodeInvalidBoard,
			"board is already decided before the game starts: winner=%v", winner)
	}

	// Every phase must have a resolver, or the skills submitted in it are
	// silently dropped when play reaches it. A resolver may be registered
	// after construction, which is why this check lives here and not in
	// NewEngine.
	if err := e.phase.validateResolvers(); err != nil {
		return nil, nil, err
	}

	start := e.config.startPhase()
	e.state.startAt(start)

	effect := newGameStartedEffect(start)
	e.recordEffects(effect)

	// The rules' opening initialisation. It goes through exactly the same
	// write point as every other effect, so it enters the effect log and can
	// be replayed. It runs after GAME_STARTED so that the effect log reads in
	// the order things happened.
	if e.gameSetup != nil {
		setupEffects, _ := e.applyEffects(e.gameSetup.Setup(newStateView(e.state)))
		e.recordEffects(setupEffects...)
	}
	e.logger.Info("game started", roundField(1), phaseField(start))

	return effect, e.snapshotEventHandlersLocked(), nil
}

// SubmitSkillUse submits one use of a skill.
func (e *Engine) SubmitSkillUse(use *SkillUse) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Validate the submission.
	if err := e.phase.validateSkillUse(use, e.state); err != nil {
		e.logger.Debug("skill validation failed",
			playerField(use.PlayerID),
			skillField(use.Skill),
			logField("error", err.Error()))
		return err
	}

	// Queue it for this phase's resolution.
	use.Phase = e.state.Phase
	use.Round = e.state.Round
	e.pendingUses = append(e.pendingUses, use)

	e.logger.Debug("skill submitted",
		playerField(use.PlayerID),
		skillField(use.Skill),
		targetField(use.Target()))

	return nil
}

// phaseOutcome is the result of one phase transition, for use outside the
// lock.
type phaseOutcome struct {
	effects  []*Effect      // every effect this phase produced, internal ones included
	events   []*Event       // the events to publish outward
	handlers []EventHandler // the handlers snapshotted under the lock
}

// endPhaseInternal ends a phase: advance the state under the lock, then
// dispatch the events outside it.
//
// The split is deliberate: dispatch has to happen outside the lock (a user
// callback may call back into the Engine) while the advance has to hold it
// throughout. Written as one function it would need a manual Unlock, and
// whoever adds an early return later would miss it.
func (e *Engine) endPhaseInternal() ([]*Effect, error) {
	out, err := e.advancePhase()
	if err != nil {
		return nil, err
	}

	for _, event := range out.events {
		dispatchEvent(out.handlers, e.logger, event)
	}

	return out.effects, nil
}

// advancePhase performs one phase transition under the lock and returns what
// has to be published outside it.
func (e *Engine) advancePhase() (phaseOutcome, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	currentPhase := e.state.Phase
	if currentPhase == PhaseEnd {
		return phaseOutcome{}, ErrGameEnded
	}
	// Advancing before the start would bypass every precondition Start
	// checks -- whether the board is already decided, whether each phase has
	// a resolver -- and Start would from then on return "already started", so
	// those checks could never run.
	if currentPhase == PhaseStart {
		return phaseOutcome{}, ErrGameNotStarted
	}

	e.logger.Debug("ending phase", phaseField(currentPhase), roundField(e.state.Round))

	out := phaseOutcome{}

	// 1. Resolve the skills into effects.
	if resolver := e.phase.resolver(currentPhase); resolver != nil {
		out.effects = resolver.Resolve(e.pendingUses, newStateView(e.state))
		e.logger.Debug("resolved effects", phaseField(currentPhase), logField("effect_count", len(out.effects)))
	}

	// 2. Apply the effects, collecting the outwardly visible events.
	out.effects, out.events = e.applyEffects(out.effects)
	e.recordEffects(out.effects...)

	// 3. Clear the pending list.
	e.pendingUses = nil

	// 4. Work out the next phase.
	//    A detour can change the outcome -- the hunter, killed, shoots the
	//    last wolf and the villagers win after all -- so the victory check is
	//    deferred while the detour queue is not yet drained.
	//    Leave this phase: the actor list and the detour at the head of the
	//    queue are both consumed (see leavePhase).
	e.state.leavePhase()

	nextPhase := e.calculateNextPhase(currentPhase, out.effects)

	gameOver, winner := e.victory.CheckVictory(newStateView(e.state))
	endNow := gameOver && !e.state.hasPendingDetour()
	if endNow {
		nextPhase = PhaseEnd
	}

	// 5. Transition. END goes through nextPhase too rather than assigning
	//    Phase directly -- every change to state takes one path, so nothing
	//    elsewhere can miss the logic that goes with it.
	//
	//    The round boundary and the victory check guard the same condition:
	//    **neither may fall while a detour is still pending**. The reasons
	//    differ but both are hard: victory because a detour may turn the game
	//    around; the round boundary because the pending queue itself lives in
	//    the round context, so clearing round state erases the queue and the
	//    exiled hunter's shot vanishes into thin air.
	//
	//    The end of the game is not a new round: nothing follows END, and an
	//    extra increment would make replay disagree (on the replay path
	//    GAME_ENDED goes through nextPhase(PhaseEnd, false, false)).
	//
	//    "Round number +1" and "round variables cleared" are two things,
	//    computed separately: most boards mark them on the same phase
	//    (EndsRound implies clearing), and a finer variable lifetime is
	//    marked with ClearsRoundVars on its own -- the missions package's
	//    team markers live until the next nomination while the round number
	//    tracks which mission it is, and the two do not coincide.
	//
	//    Counting looks at the phase **just ended**, clearing looks at the
	//    phase **being entered** -- the former says "my ending is a round",
	//    the latter says "I begin from a clean board".
	settled := !endNow && !e.state.hasPendingDetour()
	e.state.nextPhase(nextPhase,
		settled && e.config.endsRound(currentPhase),
		settled && e.config.clearsRoundVars(nextPhase))

	if endNow {
		// The end event is built along the same path as every other event,
		// Effect -> ToEvent, so that one event does not end up with two
		// separate constructions that drift apart later.
		//
		// All three exits have to get it: EndPhase's return value, OnEvent's
		// event stream, and the effect log. Without the return value, a
		// caller routing along EndPhase -> AudienceOf would miss the single
		// most important thing in the game -- who won.
		endEffect := NewEffect(EventGameEnded, "", "").
			WithData(winnerKey, winner)
		out.effects = append(out.effects, endEffect)
		e.recordEffects(endEffect)
		out.events = append(out.events, endEffect.ToEvent())

		e.winner = winner
		e.logger.Info("game ended", logField("winner", winner.String()))
	} else {
		e.recordEffects(newPhaseChangedEffect(nextPhase))
		e.logger.Debug("phase transition",
			logField("from", currentPhase.String()),
			logField("to", nextPhase.String()))
	}

	// Snapshot the handlers under the lock: the callbacks run outside it, and
	// reading e.eventHandlers outside it would race with OnEvent.
	out.handlers = e.snapshotEventHandlersLocked()

	return out, nil
}

// EndPhase ends the current phase: resolve the skills, apply the effects,
// check for victory, and transition to the next phase.
//
// This is the sole entry point that drives the game forward. Transitions
// follow the phase configuration (PhaseConfig.NextPhase) and handle the
// dynamic phases a detour brings (the hunter's shot after being killed).
func (e *Engine) EndPhase() ([]*Effect, error) {
	return e.endPhaseInternal()
}

// Status is the game at a glance: where it is, whether it is over, who won.
//
// This used to be four methods, Phase / Round / IsGameOver / Winner. Each
// took its own read lock, so **the four answers could disagree**: a host
// rendering "the day of round 3" had to ask twice, and if another goroutine
// resolved a phase in between, it read a combination of values that never
// held at the same time. Reading them once removes that.
//
// All four are scalars and allocate nothing -- the "it's cheap" argument (no
// cloning of the whole board the way View does) still holds, it is just no
// longer spread across four names. For the player roster use AlivePlayerIDs;
// for a full board you can query repeatedly, use View.
//
// Winner is settled by the VictoryChecker at the moment the game ends and
// does not change afterwards -- swapping the checker later does not rewrite a
// game that is already over.
type Status struct {
	// Phase is the current phase.
	Phase PhaseType

	// Round is the current round, counting from 1.
	Round int

	// Over says whether this game is over.
	Over bool

	// Winner is who won, CampUnspecified while it is undecided.
	Winner Camp
}

// Status reads the summary once. All four come out under one read lock, so
// they are consistent with each other.
func (e *Engine) Status() Status {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return Status{
		Phase:  e.state.Phase,
		Round:  e.state.Round,
		Over:   e.state.Phase == PhaseEnd,
		Winner: e.winner,
	}
}

// View returns a read-only view of the current board.
//
// It is the same thing a Resolver is handed. A host uses it to work something
// out for itself ("who is winning by my own reckoning", "how many special
// roles are still alive") without reading the board out field by field.
//
// A view holds the values of that moment: later play does not change a copy
// already taken.
func (e *Engine) View() GameView {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return newStateView(e.state.clone())
}

// Apply applies a batch of effects directly, bypassing phase resolution.
//
// This is a tool with an edge, and a necessary one: a host really does meet
// state changes that belong to no phase -- "the player disconnected, count
// them dead", "an admin kicked someone", "correct a misjudgement from the
// back office" -- and a rules package needs it to unit-test its own
// resolvers.
//
// It still goes through the **same write point**: effects enter the effect
// log, vetoed ones do not take effect, kernel state primitives are not sent
// out, and the rest are pushed to OnEvent. So saves, replays and audits do
// not lose fidelity because someone used it -- which is exactly what makes it
// better than reaching in and editing a playerState.
//
// What it does not do: it does not check for victory and it does not
// transition phases. To make the engine reconsider the outcome, call
// EndPhase.
//
// It returns the effects that actually took hold (nils are dropped).
func (e *Engine) Apply(effects ...*Effect) []*Effect {
	e.mu.Lock()
	kept, events := e.applyEffects(effects)
	e.recordEffects(kept...)
	handlers := e.snapshotEventHandlersLocked()
	e.mu.Unlock()

	for _, event := range events {
		dispatchEvent(handlers, e.logger, event)
	}
	return kept
}

// PlayerInfo reads one player's information from the **god's view**; the
// second result is false when there is no such player.
//
// It returns a copy, Vars and RoundVars included -- which is for the host and
// the rules, **not** for the player. For what to send a player, use
// PlayerView.
func (e *Engine) PlayerInfo(playerID string) (PlayerInfo, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.PlayerInfo(playerID)
}

// AllowedSkills are the skills this player may submit right now; empty means
// it is not their turn.
//
// It takes the same path as PlayerView(id).AllowedSkills and agrees with
// SubmitSkillUse's validation: were the three to differ, a caller running the
// phase by one of them would have the player's submission rejected by
// another.
func (e *Engine) AllowedSkills(playerID string) []SkillType {
	e.mu.RLock()
	defer e.mu.RUnlock()

	info, ok := e.state.PlayerInfo(playerID)
	if !ok {
		return nil
	}
	return e.allowedSkillsForPlayer(playerID, info)
}

// AlivePlayerIDs returns the IDs of every living player, sorted
// lexicographically.
//
// Who is still alive is public information. A caller wanting this list used
// to have to go through PhaseInfo().RoleInfos[UNSPECIFIED] -- an entry point
// that depends on the current phase happening to declare a step for all
// players, and which stopped working once the day had no player skill step.
func (e *Engine) AlivePlayerIDs() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return sortedStrings(e.state.getAlivePlayerIDs())
}

// Var reads one piece of custom state in the given scope, or the empty string
// (see VarScope).
//
// The rules use it to offer readers of their own: werewolf's "tonight's kill"
// is Var(ScopeRound, ...), the missions package's "which mission" is
// Var(ScopeGame, ...), and the kernel knows only that some such key exists.
func (e *Engine) Var(scope VarScope, key string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.varOf(scope, key)
}

// RoundContext returns a read-only copy of the round context.
func (e *Engine) RoundContext() *RoundContext {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.RoundContext()
}

// Teammates are the players this one is told are on their side, excluding
// themselves.
//
// It shares one TeammateProvider with PlayerView.Teammates and the copy in
// PhaseInfo -- replace the provider and all three change together.
func (e *Engine) Teammates(playerID string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.teammatesOf(playerID)
}

// applyEffects applies the effects one by one and returns the cleaned-up
// effects together with the events to publish outward. The caller must hold
// e.mu.
func (e *Engine) applyEffects(effects []*Effect) ([]*Effect, []*Event) {
	kept := make([]*Effect, 0, len(effects))
	events := make([]*Event, 0, len(effects))

	for _, effect := range effects {
		// A third-party Resolver's slice may contain a nil; drop it here
		// rather than bringing the game down. This check has to come first:
		// the nil guard inside applyEffect cannot reach vetDetour or the log
		// fields below.
		if effect == nil {
			continue
		}
		kept = append(kept, effect)

		e.vetDetour(effect)
		e.state.applyEffect(effect)

		e.logger.Debug("effect applied",
			eventField(effect.Type),
			playerField(effect.SourceID),
			targetField(effect.TargetID),
			logField("canceled", effect.Canceled))

		if !isInternalEvent(effect.Type) {
			events = append(events, effect.ToEvent())
		}
	}

	return kept, events
}

// vetDetour vetoes a detour pointing at an unconfigured phase.
//
// A detour's transition is an edge that only takes shape at runtime: a
// Resolver emits a NewDetourEffect naming a phase, and calculateNextPhase
// obeys unconditionally. If the configuration has no such phase (a board with
// a hunter but the hunter phase deleted, say), the engine transitions to a
// phase with no configuration and no resolver, players are allowed to submit
// nothing, and the next transition goes straight to END -- the game ends
// silently on the first night, without even a GAME_ENDED. Config.Validate
// cannot see this edge, so it has to be caught here.
//
// The caller must hold e.mu.
func (e *Engine) vetDetour(effect *Effect) {
	if effect.Canceled || effect.Type != EventDetour {
		return
	}
	phase, ok := effect.detourPhase()
	if !ok {
		effect.Cancel("ability trigger carries no target phase")
		e.logger.Error("ability trigger carries no target phase",
			playerField(effect.SourceID))
		return
	}
	if e.phase.phaseConfig(phase) == nil {
		effect.Cancel("target phase is not present in the game config")
		e.logger.Error("ability trigger points to an unconfigured phase",
			playerField(effect.SourceID), phaseField(phase))
	}
}

// calculateNextPhase works out the next phase, handling the dynamic
// transitions a detour brings. The caller must hold e.mu.
func (e *Engine) calculateNextPhase(currentPhase PhaseType, effects []*Effect) PhaseType {
	// Leaving this phase has already been accounted for by leavePhase: the
	// detour at the head of the queue pointing at the phase just ended has
	// been dequeued, and this phase's actor list has been discarded.

	// A pending detour comes first (there may be several; take them one at a
	// time).
	//
	// It outranks GOTO_PHASE: the queue has to drain -- the victory check and
	// the round boundary are both waiting on it, and jumping away mid-queue
	// would drop a debt that was never settled.
	if t, ok := e.state.peekDetour(); ok {
		return t.Phase
	}

	// The rules may override the exit: if this phase produced a GOTO_PHASE,
	// obey it. With several, the last one wins -- the same convention as
	// "registering the same role twice keeps the last registration".
	if p, ok := e.gotoFrom(effects); ok {
		return p
	}

	// Neither: take the default exit from the declarative configuration.
	return e.phase.nextSubPhase(currentPhase)
}

// gotoFrom finds the next phase the rules named among the effects this phase
// produced.
//
// A vetoed effect does not count: the rules cancelled it themselves, which
// says that directive should not take hold.
func (e *Engine) gotoFrom(effects []*Effect) (PhaseType, bool) {
	var out PhaseType
	var found bool
	for _, ef := range effects {
		if ef == nil || ef.Canceled || ef.Type != EventGotoPhase {
			continue
		}
		p, ok := ef.gotoPhase()
		if !ok {
			continue
		}
		if e.config.Phases[p] == nil {
			// A mistyped destination should not bring the game down, but
			// neither may it quietly jump somewhere nobody expected.
			e.logger.Error("goto phase not in config, falling back to NextPhase",
				phaseField(p))
			continue
		}
		out, found = p, true
	}
	return out, found
}
