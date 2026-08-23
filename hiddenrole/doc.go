// Package hiddenrole is a kernel for social deduction games.
//
// It does not know what Werewolf is. What it knows is: there are players,
// there is a cycle of phases, and at the end of each phase you ask that
// phase's resolver "what happened", then fold the answer into state.
// Plus the hard part — who is entitled to know what.
//
// A concrete rule set (roles, skills, ways to die, victory, information
// boundaries) comes from a rules package, installed entirely through public
// constructor options. github.com/Zereker/werewolf was the first such package,
// and that it uses no back door is checkable: in this package's non-test
// sources RoleType has exactly two values (RoleUnspecified, RoleSystem),
// PhaseType three and SkillType three, all in types.go. Not one "Witch",
// not one "Werewolf".
//
// # Everything the state machine knows
//
//	SubmitSkillUse  ->  Resolver.Resolve  ->  []*Effect  ->  applyEffect
//	 collect skills      adjudicate            described             the one
//	                     (pure function)       state changes         write point
//
// A Resolver receives a read-only GameView and can express state changes only
// by returning Effects. That constraint is enforced by the signature, not by
// convention — every change to state flows through one write point, which is
// what makes snapshots, replay and auditing possible at all.
//
// The state machine knows two primitives:
//
//	NewSetAliveEffect              flip a player's alive bit
//	NewSetVarEffect(scope, k, v)   write one piece of custom state
//
// Scope is a 2x2 table (lifetime x ownership); the four cells come from
// ScopeGame / ScopeRound crossed with .Of(playerID). See VarScope.
//
// Plus NewDetourEffect, which files an IOU: take a trip through some phase
// for the sake of some player.
//
// "Wolf kill", "lynch", "shoot" are names the rules give to what happened.
// The state machine does not know them — a KILL effect on its own kills
// nobody. To make someone die, the rules emit a SET_ALIVE next to it.
// Two effects, two jobs: the first is for the audience and the effect log,
// the second is for the state machine.
//
// # Who may know what
//
//	Engine.PlayerView(id)     everything this player is entitled to know,
//	                          safe to send to them verbatim
//	Engine.AudienceOf(event)  which players an event should go to
//
// The rules draw the actual lines: AudienceProvider (who hears about an
// event), TeammateProvider (who is on whose side — asymmetry allowed),
// SpeechProvider (who can hear a message).
//
// The kernel holds exactly one floor here, and it is not configurable:
// its own state primitives are never sent out. They are the state machine's
// bookkeeping; pushing them to players hands out the god view directly.
//
// # Writing a rules package
//
//	cfg := &hiddenrole.Config{StartPhase: myFirstPhase, Phases: ...}
//	e, err := hiddenrole.NewEngine(cfg,
//		hiddenrole.WithResolver(myPhase, myResolver),   // how this phase resolves
//		hiddenrole.WithRoleSetup(myRole, mySetup),      // what this role sits down with
//		hiddenrole.WithVictoryChecker(myChecker),       // what counts as winning
//		hiddenrole.WithAudience(myAudience),            // who hears about an event
//		hiddenrole.WithTeammates(myTeammates),          // who is on whose side
//		hiddenrole.WithSpeech(mySpeech))                // who can hear a message
//
// Without these you still get an engine that advances phases — it just never
// decides a winner and does not recognise a single role. That is precisely
// what "the kernel knows nothing" means.
//
// To unit-test your own resolver, use Board: lay out a position by hand, turn
// it into a GameView, feed it to the resolver, then fold the resulting effects
// back with Board.Apply and assert on what the position became.
//
// # Two decisions the kernel refuses to make for the rules
//
// Two questions about how a game advances have answers only the rules know,
// so the kernel does not guess:
//
//	which phase comes next    PhaseConfig.NextPhase is the default exit;
//	                          rules may rewrite it during resolution with
//	                          NewGotoPhaseEffect
//	is this a new round       declared by PhaseConfig.EndsRound
//
// Both used to be the kernel's own calls: the exit came from a static graph,
// and the round boundary was guessed as "we looped back to the start phase".
// In Werewolf both guesses happen to hold (night -> day -> night); change the
// rules and they stop holding. In the mission-based pack every proposal goes
// once round the loop, so "round" degenerated into a proposal counter — and
// a branch like "go to the mission if the vote passed, back to nomination
// otherwise" cannot be expressed by a static graph at all.
//
// The test is one sentence: can the kernel decide whether this is right
// without knowing what game it is? "Did state change" it can decide, so that
// belongs to the kernel. "Is this a new round" it cannot, so that belongs to
// the rules.
//
// Exit priority: pending detour queue > GOTO_PHASE > NextPhase. Detours come
// first because the queue must drain — both victory checking and the round
// boundary are waiting on it.
//
// Giving up the decision bought back checkability: while the kernel was
// guessing the round boundary it could not check whether the guess was right;
// once the rules declare it, Config.Validate can. A looping config that
// declares no EndsRound is now rejected at construction, whereas its
// consequence — round state that never resets — used to surface only
// mid-game.
//
// # Extension points must not call back into the engine
//
// The eight extension points — Resolver, VictoryChecker, AudienceProvider,
// TeammateProvider, SpeechProvider, RoleInfoProvider, RoleSetup, GameSetup —
// are all invoked synchronously while the engine holds its lock. Calling any
// Engine method from inside one hangs the game; it does not return an error.
// Go's RWMutex is not reentrant, and that game stops responding for good.
//
// They do not need to call back: everything they could want is in the
// parameters. The GameView handed to a Resolver or a provider is the complete
// position at that instant; RoleSetup does not even need a GameView, because
// seating happens before the game starts. The signatures are deliberately
// narrow — an extension point cannot reach an *Engine, and routing around
// that means storing the engine in your own struct, which is a deliberate act.
//
// To ask the engine something from a callback, use an OnEvent / OnMessage
// handler: events and messages are published outside the lock, so calling
// AudienceOf, PlayerView or Snapshot from a handler is the supported usage.
// Wiring the engine into a server is exactly that — receive an event, ask
// "who should get this", write to those connections; see example/netserver
// in the werewolf repository. TestCallbacks_MayCallBackIntoTheEngine watches
// this property and carries a timeout: if dispatch is ever moved inside the
// lock that test goes red instead of hanging the whole suite.
//
// # Boundaries: what the kernel does not do
//
//   - No timers. PhaseConfig.Timeout is advisory only.
//   - No networking, no rooms, no matchmaking.
//   - No storage. Snapshot exports a position and RestoreEngine rebuilds one;
//     where it is stored is the caller's business.
//   - No knowledge of any game's rules. That is the rules package's job.
package hiddenrole
