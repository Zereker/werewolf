package hiddenrole

// PlayerView is everything one player is entitled to know at this moment,
// seen from where they sit.
//
// # Why it exists
//
// The one genuinely hard thing about these games is who is allowed to know
// what. The engine used to offer the god's view only -- PlayerInfo could look
// up anybody's role, PhaseInfo handed over the wolf roster and the kill in
// one go -- and pushed the most safety-critical filtering onto the caller.
// One handler that slips and broadcasts a whole PhaseInfo voids the game on
// the spot.
//
// A caller acting as the host does need the god's view; but it should not be
// forced to implement the projection itself. PlayerView pulls that back
// inside the library: give it a player ID and what comes back can be sent
// straight to them.
//
// # What it does not contain
//
// A view is the state right now, not the history. The seer's past checks and
// the public record of deaths are history, and are carried by the effect log
// (Engine.EffectLog).
type PlayerView struct {
	PlayerID string    `json:"player_id"` // whose view this is
	Round    int       `json:"round"`     // the current round
	Phase    PhaseType `json:"phase"`     // the current phase

	// Self is their own information: role, camp, whether they are alive.
	Self SelfInfo `json:"self"`

	// Players is the public information about everyone at the table, sorted
	// by ID. A role is filled in only where it is revealed to this view
	// (themselves, their teammates).
	Players []PublicPlayerInfo `json:"players"`

	// AllowedSkills are the skills they may submit this phase, never nil.
	// It is an empty slice when it is not their turn -- which is also how you
	// answer "is it my turn".
	AllowedSkills []SkillType `json:"allowed_skills"`

	// Teammates are the players this one is told are on their side; their
	// roles are revealed to them.
	//
	// Answered by the TeammateProvider (see WithTeammates); the kernel does
	// not know about camps. Werewolf's default implementation is "the other
	// players in the wolf camp" -- by camp rather than by role, or a custom
	// same-camp role from a rules package would find the teammate list empty.
	Teammates []string `json:"teammates,omitempty"`

	// RoleInfo is role-specific information: what this role additionally
	// lets them see.
	//
	// Answered by the role's own RoleInfoProvider (see WithRoleInfo); the
	// engine recognises no specific role. The built-in witch's kill target
	// and remaining potions live here (under the keys RoleInfoKillTarget /
	// RoleInfoAntidote / RoleInfoPoison) -- they used to be named fields on
	// PlayerView and SelfInfo, which made built-in roles first-class
	// citizens next to third-party ones, and adding a role should not
	// require editing the engine.
	RoleInfo map[string]string `json:"role_info,omitempty"`
}

// SelfInfo is everything a player is entitled to know about themselves.
//
// It deliberately does not reuse the god's-view PlayerInfo: that struct
// carries Protected (whether the guard shielded them tonight), and who the
// guard protected is the guard's exclusive information -- the moment the
// protected player knows, they know they cannot be killed tonight, and the
// guard's possible positions narrow sharply. A visibility difference of one
// field should not depend on the caller remembering to blank it.
type SelfInfo struct {
	ID    string   `json:"id"`
	Role  RoleType `json:"role"`
	Alive bool     `json:"alive"`

	// Camp is which side this player is on.
	//
	// An **opaque** label, taken from the canonical Vars key VarCamp. The
	// kernel only carries it: it does not know what "EVIL" means, nor
	// whether this player should know their own camp -- the rules decide
	// that when they hand out the initial state.
	//
	// Sub-divisions within a camp (werewolf's special roles vs plain
	// villagers) do not live here: that is the rules' own key, read from
	// Vars.
	Camp Camp `json:"camp,omitempty"`
}

// PublicPlayerInfo is the publicly visible information about one player.
//
// It, SelfInfo and PlayerInfo are three faces of the same player, and keeping
// them apart is not a naming coincidence: this type **structurally cannot
// hold** Vars, which turns "should they be shown this" into a question about
// signatures rather than one about runtime. Merging them into one type with
// optional fields would throw that guarantee away.
//
// The rule is enforced by TestPlayerView_CarriesNoFreeFormState: any
// free-form state bag appearing in a player-facing struct turns it red.
type PublicPlayerInfo struct {
	ID    string `json:"id"`
	Alive bool   `json:"alive"`

	// Role is filled in only where this player's role is revealed to this
	// view, and is UNSPECIFIED otherwise. The engine reveals only "yourself"
	// and "your teammates" by default -- whether an eliminated player's card
	// is turned over is a table rule, decided by the caller, and the engine
	// does not decide it for them.
	Role RoleType `json:"role,omitempty"`
}

// PlayerView returns one player's view.
//
// What it returns can be sent straight to that player with no further
// filtering by the caller. It returns nil when there is no such player.
//
// By contrast PhaseInfo and PlayerInfo are god's-view APIs: a caller acting
// as the host needs them, but their contents must not be forwarded to players
// wholesale.
func (e *Engine) PlayerView(playerID string) *PlayerView {
	e.mu.RLock()
	defer e.mu.RUnlock()

	self, ok := e.state.PlayerInfo(playerID)
	if !ok {
		return nil
	}

	view := &PlayerView{
		PlayerID: playerID,
		Round:    e.state.Round,
		Phase:    e.state.Phase,
		Self: SelfInfo{
			ID:    self.ID,
			Role:  self.Role,
			Camp:  Camp(self.Vars[VarCamp]),
			Alive: self.Alive,
		},
		AllowedSkills: e.allowedSkillsForPlayer(playerID, self),
	}

	// Teammates see each other. Who is on whose side is answered by the
	// rules (see TeammateProvider); the kernel does not know about camps,
	// which is also what makes Blood on the Clocktower's one-way visibility
	// expressible.
	revealed := map[string]bool{playerID: true}
	view.Teammates = e.teammatesOf(playerID)
	for _, id := range view.Teammates {
		revealed[id] = true
	}

	view.Players = e.publicPlayers(revealed)

	// Role-specific information is answered by the role itself.
	view.RoleInfo = e.roleInfoFor(playerID, self.Role)

	return view
}

// publicPlayers assembles the public information about everyone. Players in
// revealed carry their role. The caller must hold e.mu.
func (e *Engine) publicPlayers(revealed map[string]bool) []PublicPlayerInfo {
	ids := e.state.allPlayerIDs()
	out := make([]PublicPlayerInfo, 0, len(ids))
	for _, id := range ids {
		p, ok := e.state.PlayerInfo(id)
		if !ok {
			continue
		}
		info := PublicPlayerInfo{ID: p.ID, Alive: p.Alive}
		if revealed[id] {
			info.Role = p.Role
		}
		out = append(out, info)
	}
	return out
}

// allowedSkillsForPlayer returns the skills this player may submit right now,
// never nil.
//
// "Empty means it is not my turn yet" is semantically the same as nil, but
// they serialise as [] and null respectively -- one field, two shapes, and
// the caller has to handle both. The caller must hold e.mu.
//
// The two layers line up **item for item** with the validation in
// SubmitSkillUse: a different order there would produce the
// self-contradiction of "the kernel accepted his submission while telling him
// he cannot act".
func (e *Engine) allowedSkillsForPlayer(playerID string, info PlayerInfo) []SkillType {
	// When the rules have named this phase's actors, anyone off the list can
	// do nothing; the player a detour is for goes through this branch too --
	// on entering the phase they were already written onto the list (see
	// gameState.nameDetourActor). For those on the list, aliveness is the
	// rules' business and the kernel does not veto a second time.
	if ids, ok := e.state.actorsFor(e.state.Phase); ok {
		if !contains(ids, playerID) {
			return []SkillType{}
		}
		return e.allowedSkillsFor(info.Role)
	}
	if !info.Alive {
		return []SkillType{}
	}
	return e.allowedSkillsFor(info.Role)
}

// contains reports whether the list holds this ID.
func contains(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// ==================== Who receives an effect ====================

// AudienceOf returns which players should be told about something.
//
// This is PlayerView's other half: the view settles what state a player
// should see, and this settles who should be told about what happened. A
// caller routes on it instead of having to remember for itself that "a check
// result goes to the seer only".
//
// The parameter is an outward Event rather than an internal Effect: the
// question is what the outside world should see, and an Event is exactly what
// OnEvent pushes to the caller. When you hold an Effect (EndPhase's return
// value), convert it with Effect.ToEvent().
//
// The kernel's state primitives (SET_ALIVE and friends) always return empty,
// and that part is not configurable -- they are the state machine's
// bookkeeping and have no business in front of any player. Everything else
// goes to the AudienceProvider; werewolf's is wolfAudience, and it can be
// replaced wholesale.
//
// The second result says whether the event type is recognised. A third-party
// Resolver may emit events of its own types, whose visibility the rules
// cannot judge, and (nil, false) is the answer then: the caller has to route
// it themselves, and "I don't know" must not be mistaken for "show it to
// nobody".
func (e *Engine) AudienceOf(event *Event) ([]string, bool) {
	if event == nil {
		return nil, false
	}
	if isInternalEvent(event.Type) {
		// A kernel state primitive: shown to nobody. This is a definite
		// verdict, not an "I don't know".
		return nil, true
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.audience == nil {
		return nil, false
	}
	return e.audience.Audience(event, newStateView(e.state))
}
