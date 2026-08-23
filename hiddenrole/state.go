package hiddenrole

import (
	"sort"
)

// RoundContext is the round context, rebuilt each round.
// It holds the temporary state shared between the phases of one round: valid
// within this round, cleared automatically across rounds.
type RoundContext struct {
	// Detours are the pending detours, first in first out.
	//
	// This used to be two fields belonging to one specific role, so every
	// role that shoots on death meant two more fields and one more branch in
	// the engine's phase transitions. As a queue, the engine recognises no
	// specific role.
	Detours []Detour

	// Vars is this round's custom state, cleared automatically each round,
	// belonging to no player.
	//
	// Werewolf's "tonight's kill" is stored here. It used to be a field above
	// called KillTarget which, together with three other maps, wrote "which
	// round state one particular ruleset has" into the kernel -- change the
	// ruleset and not one of the four is any use.
	//
	// The four scope cells (see VarScope): playerState.Vars follows a player
	// for the whole game, this one is cleared each round, and
	// playerState.RoundVars is "a marker on one player this round". Write
	// with NewSetVarEffect(ScopeRound, ...), read with
	// GameView.Var(ScopeRound, ...).
	Vars map[string]string
}

// Detour is one pending detour: **for the sake of someone, take a trip
// through some phase**.
//
// It used to be called PendingTrigger, documented as "a pending death
// ability". That is werewolf's phrasing -- the hunter shooting after being
// killed. What the kernel recognises was never death and was never a skill,
// only "who, and to which phase": what triggered it and what they do once
// there is entirely the rules' business.
//
// It governs three things, the last two of which nothing else can provide:
//
//  1. routing the phase to where the debt is   -- GOTO_PHASE can do this too
//  2. holding off the victory check and the round boundary until it drains
//     -- a detour can turn the game around (that shot takes the last wolf)
//  3. taking them one at a time from the head  -- two people owing on the
//     same night each get their own trip
//
// It does **not** answer "who may act": on entering the phase owed to, it
// writes an actor list (see gameState.nameDetourActor), and everything after
// that takes exactly the same path as NewSetActorsEffect.
type Detour struct {
	PlayerID string    // whose sake this trip is for
	Phase    PhaseType // which phase to visit
}

// newRoundContext builds a new round context.
func newRoundContext() *RoundContext {
	return &RoundContext{}
}

// playerState is one player's state.
type playerState struct {
	ID    string
	Role  RoleType
	Alive bool

	// Vars is the role's private state, the state the rules judge on.
	//
	// The rules keep a role's private state here: werewolf's two witch
	// potions and the knight's single duel are the same thing. The kernel
	// used to have dedicated bool fields for the built-in roles, so a
	// third-party role could neither change its own state nor give itself an
	// initial one -- which is exactly what "adding a role should not require
	// editing the engine" sets out to eliminate.
	//
	// The initial value is handed out by RoleSetup (see WithRoleSetup), and
	// afterwards written with NewSetVarEffect(ScopeGame.Of(id), ...) and read
	// with GameView.Var; it travels with the snapshot and can be rebuilt by
	// replay.
	//
	// Records that outlive a round live here too (werewolf's "who the guard
	// protected last round" is one): the rules do the judging, the kernel
	// only stores.
	Vars map[string]string

	// RoundVars are this player's markers for the current round, cleared
	// automatically each round.
	//
	// Who was guarded, healed or poisoned tonight are all of this kind. They
	// used to be three map[string]bool fields on RoundContext, which a
	// third-party role could neither write nor read -- and "a marker on
	// someone this round" is a shape every social-deduction ruleset uses.
	// Write with NewSetVarEffect(ScopeRound.Of(id), ...), read with
	// GameView.Var.
	RoundVars map[string]string
}

// gameState is the game's state.
//
// # Concurrency
//
// This type does no locking of its own. It is the Engine's internal state,
// unexported and absent from every exported signature, and every access
// happens while the Engine holds its lock; a Resolver is handed a read-only
// GameView, likewise built and used under the lock.
//
// There used to be an RWMutex of its own here, nesting two locks with the
// Engine's, on the grounds that "State can be used independently" -- a
// premise that stopped holding once it moved inside the package, leaving the
// extra lock as pure cost, in cycles and in reasoning.
type gameState struct {
	Phase   PhaseType               // the current phase
	Round   int                     // the current round
	players map[string]*playerState // player state; private, reached through methods

	// Vars is state that lives for the whole game and belongs to no player.
	//
	// A variable's scope is a 2x2 table -- lifetime (whole game / this round)
	// crossed with ownership (unowned / owned by a player). There used to be
	// only three cells:
	//
	//	              unowned        owned by a player
	//	  whole game   (missing)      playerState.Vars
	//	  this round   RoundCtx.Vars  playerState.RoundVars
	//
	// The missing cell was not a deliberate gap, it was an oversight:
	// werewolf's game-long state all happens to hang off a person (the
	// witch's potions, who the guard protected last round), so nobody ran
	// into it. The mission-based games did -- "which mission", "how many
	// succeeded", "how many consecutive rejects", "whose turn to lead" are
	// all game-long and belong to nobody, and could only be filed under some
	// player's private state as a ledger, which made four fields that had
	// nothing to do with them appear out of nowhere in that player's
	// PlayerView.
	//
	// Write with NewSetVarEffect(ScopeGame, ...), read with GameView.Var or
	// Engine.Var.
	Vars map[string]string

	// Actors is "which players may act in which phase", named by the rules at
	// runtime.
	//
	// The kernel used to have one way of deciding actors: match
	// PhaseStep.Role against a player's role. A role is fixed at seating
	// time, so any actor set chosen at runtime was inexpressible. That
	// abstraction had been escaped three times: werewolf's hunter shot (for
	// which the kernel opened the detour queue as a one-player special case),
	// the missions package's leader nomination, and its mission team. The
	// latter two could only let everyone submit and have the resolver throw
	// away what should not count, at the cost of AllowedSkills lying to
	// unqualified players and PhaseReadiness waiting on a crowd who cannot
	// possibly act.
	//
	// The rules can now say it directly: "these people, in that phase". Write
	// with NewSetActorsEffect, and the kernel enforces it at SubmitSkillUse
	// rather than leaving the rules to filter afterwards.
	//
	// It is stored per phase rather than for "the current phase" only,
	// because the set is often computed in an **earlier phase** -- the
	// missions package picks its team during nomination and uses it in the
	// mission phase. A phase's entry is consumed once that phase resolves.
	Actors map[PhaseType][]string

	// The round's temporary context, rebuilt each round.
	RoundCtx *RoundContext
}

// newState builds a game state.
func newState() *gameState {
	return &gameState{
		Phase:    PhaseStart,
		Round:    0,
		players:  make(map[string]*playerState),
		RoundCtx: newRoundContext(),
	}
}

// addPlayer adds a player.
//
// The kernel records only the ID, the role and aliveness; camp and category
// are the rules' way of dividing things up, handed out as initial state by
// RoleSetup at seating time (see seatPlayer).
//
// Errors: an empty ID; an ID already taken; a role that cannot be assigned to
// a player (the system role).
func (s *gameState) addPlayer(id string, role RoleType) error {
	if id == "" {
		return ErrInvalidPlayerID
	}
	// The system role is a marker, not a player identity.
	if role == RoleUnspecified || role == RoleSystem {
		return WrapError(CodeInvalidRole,
			"role %v cannot be assigned to a player", role)
	}

	if _, exists := s.players[id]; exists {
		return WrapError(CodePlayerExists, "player %q already exists", id)
	}

	player := &playerState{
		ID:    id,
		Role:  role,
		Alive: true,
	}

	s.players[id] = player
	return nil
}

// setPlayerVars writes a batch of custom state onto one player, for handing
// out the initial state at seating time.
//
// An empty value means deletion, matching the SET_VAR write point -- without
// that, an empty string the rules wrote would sit in the snapshot.
func (s *gameState) setPlayerVars(id string, vars map[string]string) {
	if len(vars) == 0 {
		return
	}
	p, ok := s.players[id]
	if !ok {
		return
	}
	for k, v := range vars {
		if k == "" {
			continue
		}
		if v == "" {
			delete(p.Vars, k)
			continue
		}
		if p.Vars == nil {
			p.Vars = make(map[string]string, len(vars))
		}
		p.Vars[k] = v
	}
}

// currentPhase is the current phase. Package-internal.
func (s *gameState) currentPhase() PhaseType {
	return s.Phase
}

// currentRound is the current round. Package-internal.
func (s *gameState) currentRound() int {
	return s.Round
}

// getPlayer returns a player. Package-internal.
// It returns the internal pointer, for use by package code only; callers
// outside should use PlayerInfo(id) for a read-only copy.
func (s *gameState) getPlayer(id string) (*playerState, bool) {
	p, ok := s.players[id]
	return p, ok
}

// PlayerInfo is a read-only view of a player, from the god's point of view.
//
// It contains information only the god should know, and must not be forwarded
// to players wholesale -- for what to send a player, use Engine.PlayerView.
type PlayerInfo struct {
	ID    string   `json:"id"`
	Role  RoleType `json:"role"`
	Alive bool     `json:"alive"`

	// RoundVars are this player's markers for the current round, cleared
	// every round.
	//
	// This used to be a bool called Protected -- "was this player guarded
	// tonight" is a werewolf concept and the kernel has no business knowing
	// it. It is now just a key the rules define, alongside every other
	// marker.
	RoundVars map[string]string `json:"round_vars,omitempty"`

	// Vars is the role's private state, under keys the rules choose.
	//
	// It deliberately appears only here, in the god's view, and not on the
	// player-facing SelfInfo: what goes into it is up to the role, and
	// handing it to the player by default would make every role work out for
	// itself whether each entry may be shown -- exactly the class of
	// judgement this library sets out to take off a caller's hands. What a
	// player should see is projected explicitly by the role through a
	// RoleInfoProvider.
	Vars map[string]string `json:"vars,omitempty"`
}

// PlayerInfo returns a read-only copy of a player's information.
func (s *gameState) PlayerInfo(id string) (PlayerInfo, bool) {
	p, ok := s.players[id]
	if !ok {
		return PlayerInfo{}, false
	}

	return PlayerInfo{
		ID:        p.ID,
		Role:      p.Role,
		Alive:     p.Alive,
		RoundVars: copyVars(p.RoundVars),
		Vars:      copyVars(p.Vars),
	}, true
}

// getAlivePlayerIDsByRole returns the IDs of living players with the given
// role. Package-internal.
func (s *gameState) getAlivePlayerIDsByRole(role RoleType) []string {
	result := make([]string, 0)
	for id, p := range s.players {
		if p.Alive && p.Role == role {
			result = append(result, id)
		}
	}
	return result
}

// allPlayerIDs returns every player ID, sorted lexicographically.
// Package-internal. The sort keeps player-facing views stable regardless of
// map iteration order.
func (s *gameState) allPlayerIDs() []string {
	result := make([]string, 0, len(s.players))
	for id := range s.players {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

// getAlivePlayerIDs returns the IDs of every living player, sorted
// lexicographically. Package-internal.
//
// The sort is not optional: this list flows into the effects the rules
// produce (speech audiences, resolution order), and a map's iteration order
// differs every time -- unsorted, resolving the same board twice would
// produce different effect logs, and replay and byte-for-byte comparison
// would lose their determinism.
func (s *gameState) getAlivePlayerIDs() []string {
	result := make([]string, 0, len(s.players))
	for id, p := range s.players {
		if p.Alive {
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result
}

// clone copies the state, for Engine.View.
//
// A view has to be detached from the engine: play continues after a view is
// taken and that copy must not follow along -- otherwise "the board at this
// moment" means nothing.
func (s *gameState) clone() *gameState {
	out := newState()
	out.Phase = s.Phase
	out.Round = s.Round
	out.Vars = copyVars(s.Vars)
	out.Actors = copyActors(s.Actors)
	out.RoundCtx = &RoundContext{
		Detours: append([]Detour(nil), s.RoundCtx.Detours...),
		Vars:    copyVars(s.RoundCtx.Vars),
	}
	for id, p := range s.players {
		out.players[id] = &playerState{
			ID: p.ID, Role: p.Role, Alive: p.Alive,
			Vars: copyVars(p.Vars), RoundVars: copyVars(p.RoundVars),
		}
	}
	return out
}

// applyEffect applies one effect. This is the single write point for state.
//
// An unrecognised effect type is silently ignored -- a third-party Resolver
// emitting a type the engine does not know raises no error and changes no
// state. When extending, reuse an existing type, or have the Resolver break
// its own semantics down into effects the engine does recognise.
func (s *gameState) applyEffect(effect *Effect) {
	// A third-party Resolver's slice may contain a nil; not worth bringing
	// the game down for.
	if effect == nil {
		return
	}

	// A vetoed effect changes no state but still appears in EndPhase's return
	// value, so the caller knows that someone tried and failed, and why.
	if effect.Canceled {
		return
	}

	// Make sure RoundCtx exists.
	if s.RoundCtx == nil {
		s.RoundCtx = newRoundContext()
	}

	switch effect.Type {
	// -- what follows is the kernel's complete set of state primitives --
	//
	// The engine used to recognise KILL / POISON / ELIMINATE / SHOOT (ways to
	// die), PROTECT / SAVE (tonight's markers), SET_NIGHT_KILL /
	// CLEAR_NIGHT_KILL, SET_LAST_PROTECTED, USE_ANTIDOTE / USE_POISON -- a
	// dozen branches, every one of them a werewolf rule. Change the ruleset
	// and not one is any use, while a new rule wanting to express its own
	// state change had no option but to come and edit this switch.
	//
	// The rules now name what happened themselves (KILL, SHOOT, heartbreak, a
	// duel) and emit one of the primitives below to actually change the
	// state. Two effects, two things: the first for the audience and the
	// effect log, the second for the state machine.
	case EventSetAlive:
		if alive, ok := aliveOf(effect); ok {
			if target, found := s.players[effect.TargetID]; found {
				target.Alive = alive
			}
		}

	case EventSetVar:
		// One piece of custom state, its scope carried in the effect. An
		// empty value deletes, so that empty strings do not pile up in the
		// snapshot.
		if scope, key, value := varOf(effect); key != "" {
			s.setVar(scope, key, value)
		}

	case EventSetActors:
		// The rules name a phase's actors. Players who do not exist are
		// ignored.
		if phase, ids, ok := actorsOf(effect); ok {
			kept := make([]string, 0, len(ids))
			for _, id := range ids {
				if _, exists := s.players[id]; exists {
					kept = append(kept, id)
				}
			}
			s.setActors(phase, kept)
		}

	case EventDetour:
		// Enqueue a detour, to be settled when play reaches its phase.
		if phase, ok := effect.detourPhase(); ok && effect.SourceID != "" {
			s.RoundCtx.Detours = append(s.RoundCtx.Detours,
				Detour{PlayerID: effect.SourceID, Phase: phase})
		}
	}
}

// resetRoundState resets the round state, called at the start of each round.
func (s *gameState) resetRoundState() {
	s.resetRoundStateUnlocked()
}

// resetRoundStateUnlocked is the internal form, taking no lock.
func (s *gameState) resetRoundStateUnlocked() {
	s.RoundCtx = newRoundContext()
	// The round markers on players belong to this round too, so clear them
	// along with it -- miss this and last night's "guarded" and "poisoned"
	// keep piling up, the same class of mistake as hard-coding the round
	// boundary to NIGHT_GUARD.
	for _, p := range s.players {
		p.RoundVars = nil
	}
}

// startAt puts the state at the start of play: the given phase, round one,
// and a clean round context.
func (s *gameState) startAt(phase PhaseType) {
	s.Phase = phase
	s.Round = 1
	s.resetRoundStateUnlocked()
}

// leavePhase consumes the one-shot things on leaving the current phase.
//
// Two of them: this phase's actor list, and the detour at the head of the
// queue pointing at this phase. Both are spent on use -- without clearing
// them, the next visit to the same phase would inherit the previous round's
// list, or the same person would be dragged back to use a skill again and
// again.
//
// **It was gathered into one function because it used to be spread across two
// paths, and those two paths drifted three times.** Normal progression
// (endPhaseInternal) and effect-log replay (replayEffect) have to do exactly
// the same thing, or the replayed engine diverges from the original on the
// very next step. All three divergences were caught by the random-game
// invariants, and each time the replay path had done one thing less:
//
//	the actor list not consumed        -- replay carried the previous phase's list
//	the list not consumed on ending    -- the GAME_ENDED branch skipped consumeActors
//	the detour not consumed on ending  -- likewise, it skipped consumeDetourFor
//
// Rather than apply a third patch, it was gathered into one place.
func (s *gameState) leavePhase() {
	s.consumeActors(s.Phase)
	s.consumeDetourFor(s.Phase)
}

// nextPhase moves to the next phase.
//
// endsRound is declared by the phase **just resolved**
// (PhaseConfig.EndsRound): true means the round ends here, so the round
// number goes up and all round-scoped state is cleared.
//
// The kernel used to guess this -- "looping back to the start phase counts as
// a new round". That guess holds for werewolf (night -> day -> night) and not
// for other rulesets: the mission-based games go round the loop once per
// nomination, so "round" became a nomination counter. What one round of a
// game is, only the rules know, and the kernel no longer decides it for
// them.
func (s *gameState) nextPhase(phase PhaseType, endsRound, clearVars bool) {
	s.Phase = phase
	if endsRound {
		s.Round++
	}
	if clearVars {
		s.resetRoundStateUnlocked()
	}
	s.nameDetourActor()
}

// nameDetourActor writes the owed player as this phase's actor list, when the
// phase just entered is the one the head of the queue is owed in.
//
// This is the seam between the detour queue and the rules naming actors. They
// used to be two parallel mechanisms: actorsForStep and validateSkillUse each
// had a three-layer decision, asking the detour queue first, the named actors
// second, and computing by role third. Both paths answered the same question
// with nearly word-for-word identical implementations (triggerActorFor and
// namedActorsFor were both "of the players named, who carries this role's
// step") -- one concept, two implementations, both to be kept in step.
//
// The queue no longer answers "who may act": it only **produces** a list, and
// everything after that follows the naming path. Three layers became two, and
// triggerActorFor and isTriggerActor were deleted together.
//
// It lives here rather than at the DETOUR write point: the queue may hold
// several detours pointing at the same phase (two hunters eliminated on one
// night), and writing at the enqueue point would have them overwrite each
// other, leaving only the last one able to shoot. Taking the **head** on
// entering the phase is what a queue means in the first place.
//
// Normal progression and effect-log replay share this path (both go through
// nextPhase), so they cannot diverge.
func (s *gameState) nameDetourActor() {
	t, ok := s.peekDetour()
	if !ok || t.Phase != s.Phase {
		return
	}
	s.setActors(s.Phase, []string{t.PlayerID})
}

// varsFor locates the map a scope corresponds to, and whether it exists.
//
// The four scopes converge here: the two unowned cells hang off gameState and
// RoundContext, the two owned ones off playerState. When it cannot be reached
// (no such player, no round context) it returns nil and an unusable writer.
func (s *gameState) varsFor(scope VarScope) (read map[string]string, write func(map[string]string)) {
	if scope.owner == "" {
		if scope.perRound {
			if s.RoundCtx == nil {
				return nil, nil
			}
			return s.RoundCtx.Vars, func(m map[string]string) { s.RoundCtx.Vars = m }
		}
		return s.Vars, func(m map[string]string) { s.Vars = m }
	}

	p, ok := s.players[scope.owner]
	if !ok {
		return nil, nil
	}
	if scope.perRound {
		return p.RoundVars, func(m map[string]string) { p.RoundVars = m }
	}
	return p.Vars, func(m map[string]string) { p.Vars = m }
}

// varOf reads one piece of custom state in a scope, or the empty string.
func (s *gameState) varOf(scope VarScope, key string) string {
	vars, _ := s.varsFor(scope)
	return vars[key]
}

// setVar writes one piece of custom state in a scope. An empty string means
// deletion, identically in all four scopes.
func (s *gameState) setVar(scope VarScope, key, value string) {
	vars, write := s.varsFor(scope)
	if write == nil {
		return
	}
	if value == "" {
		delete(vars, key)
		return
	}
	if vars == nil {
		vars = make(map[string]string, 1)
		write(vars)
	}
	vars[key] = value
}

// peekDetour looks at the pending detour at the head of the queue.
func (s *gameState) peekDetour() (Detour, bool) {
	if s.RoundCtx == nil || len(s.RoundCtx.Detours) == 0 {
		return Detour{}, false
	}
	return s.RoundCtx.Detours[0], true
}

// popDetour removes the pending detour at the head of the queue.
func (s *gameState) popDetour() {
	if s.RoundCtx == nil || len(s.RoundCtx.Detours) == 0 {
		return
	}
	s.RoundCtx.Detours = s.RoundCtx.Detours[1:]
}

// consumeDetourFor dequeues the head detour when it is the one for phase.
//
// The pending queue is one-shot: a death resolution enqueues an entry, and it
// must be dequeued once play enters the corresponding phase. Left in place it
// stays non-empty for the whole round, and the same player is dragged back to
// use a skill again and again.
//
// Both normal progression (calculateNextPhase) and effect-log replay
// (replayEffect handling PHASE_CHANGED) do this step, and must do it
// identically, or the replayed engine carries a detour that should have been
// consumed and diverges from the original on the next step.
func (s *gameState) consumeDetourFor(phase PhaseType) {
	if t, ok := s.peekDetour(); ok && t.Phase == phase {
		s.popDetour()
	}
}

// hasPendingDetour reports whether any detour is still unsettled.
func (s *gameState) hasPendingDetour() bool {
	_, ok := s.peekDetour()
	return ok
}

// RoundContext returns a read-only copy of the round context.
func (s *gameState) RoundContext() *RoundContext {
	if s.RoundCtx == nil {
		return nil
	}

	// A copy, so that outside code cannot modify it.
	return &RoundContext{
		Detours: append([]Detour(nil), s.RoundCtx.Detours...),
		Vars:    copyVars(s.RoundCtx.Vars),
	}
}

// copyActors deep-copies the actor table.
func copyActors(in map[PhaseType][]string) map[PhaseType][]string {
	if in == nil {
		return nil
	}
	out := make(map[PhaseType][]string, len(in))
	for k, v := range in {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// actorsFor returns the actors the rules named for a phase, or nil when they
// named none.
//
// nil and an empty slice are different things: nil is "the rules did not say,
// compute by role", an empty slice is "the rules said, and nobody can act in
// this phase".
func (s *gameState) actorsFor(phase PhaseType) ([]string, bool) {
	if s.Actors == nil {
		return nil, false
	}
	v, ok := s.Actors[phase]
	return v, ok
}

// setActors names a phase's actors.
func (s *gameState) setActors(phase PhaseType, ids []string) {
	if s.Actors == nil {
		s.Actors = map[PhaseType][]string{}
	}
	s.Actors[phase] = sortedStrings(ids)
}

// consumeActors spends a phase's actor naming once that phase resolves.
//
// Without clearing it, the next visit to the same phase would inherit the
// previous list -- which is almost always wrong: it was computed for the
// previous round.
func (s *gameState) consumeActors(phase PhaseType) {
	delete(s.Actors, phase)
}
