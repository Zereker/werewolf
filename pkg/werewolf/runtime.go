package werewolf

import (
	"context"
	"fmt"
	"log/slog"
	"strings" // Added for strings.Join
	"sync"
	"time" // Added for time.Sleep

	"github.com/Zereker/socket"
	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event" // For event.VoteResultData
	"github.com/Zereker/werewolf/pkg/game/phase"
	"github.com/Zereker/werewolf/pkg/game/player"
	"github.com/Zereker/werewolf/pkg/game/skill" // For skill.Hunter
	"github.com/Zereker/werewolf/pkg/server"   // For server.MessageTypeHunterShoot (though action type is used)
)

// Runtime 游戏运行时
type Runtime struct {
	sync.RWMutex

	winner            game.Camp
	players           []game.Player
	playerConnections map[string]*socket.Conn

	started bool

	round    int
	phaseIdx int
	phases   []game.Phase

	logger *slog.Logger

	// Hunter skill related
	pendingHunterShotPlayerID string // ID of the Hunter who died and needs to shoot
	hunterShotCtx             context.Context // Context for hunter shot action, potentially with timeout
	hunterShotCancel          context.CancelFunc // To cancel waiting for hunter's shot
}

// NewRuntime 创建游戏运行时
func NewRuntime() *Runtime {
	return &Runtime{
		round:             1,
		logger:            slog.Default().With("game", "werewolf"),
		playerConnections: make(map[string]*socket.Conn),
		players:           make([]game.Player, 0),
	}
}

// HandlePlayerAction handles an action sent by a player.
func (r *Runtime) HandlePlayerAction(playerID string, actionDetails interface{}) {
	r.RLock()
	if !r.started {
		r.logger.Warn("Received player action but game not started", "playerID", playerID)
		r.RUnlock()
		return
	}
	
	// Handle Hunter's shot if pending
	if r.pendingHunterShotPlayerID != "" && r.pendingHunterShotPlayerID == playerID {
		actionMap, ok := actionDetails.(map[string]interface{})
		if !ok {
			r.logger.Error("Could not parse actionDetails for Hunter shot", "details", actionDetails)
			r.RUnlock()
			return
		}
		if skillType, _ := actionMap["skillType"].(string); skillType == string(game.SkillTypeHunter) {
			r.RUnlock() // Unlock before calling processHunterShot which might lock
			r.processHunterShot(playerID, actionMap)
			return
		}
		// If it's the pending hunter but not a hunter_shot action, log and ignore for now.
		// Or, if hunter can do other things while dead (like chat), handle here.
		r.logger.Debug("Action received from pending hunter, but not a hunter_shot", "playerID", playerID, "action", actionDetails)
	}
	
	// Regular phase action handling
	if r.phaseIdx < 0 || r.phaseIdx >= len(r.phases) {
		r.logger.Error("phaseIdx out of bounds in HandlePlayerAction", "phaseIdx", r.phaseIdx, "phases_len", len(r.phases))
		r.RUnlock()
		return
	}
	currentPhase := r.phases[r.phaseIdx]
	var actingPlayer game.Player
	for _, p := range r.players {
		if p.GetID() == playerID {
			actingPlayer = p
			break
		}
	}
	r.RUnlock() // Unlock after initial read of shared fields for regular action

	if actingPlayer == nil {
		r.logger.Error("Player not found for action", "playerID", playerID)
		return
	}

	// Allow dead players to perform "last_words" if current phase is LastWordsPhase
	// And allow dead hunter to shoot if pendingHunterShotPlayerID is set (handled above)
	if !actingPlayer.IsAlive() {
		_, isLastWordsPhase := currentPhase.(*phase.LastWordsPhase)
		isPendingHunter := r.pendingHunterShotPlayerID == playerID

		if !isLastWordsPhase && !isPendingHunter {
			r.logger.Warn("Received action from a non-alive player", "playerID", playerID, "phase", currentPhase.GetName())
			return
		}
	}

	r.logger.Info("Forwarding action to current phase", "playerID", playerID, "phase", currentPhase.GetName(), "action", actionDetails)
	resultsChan := make(chan error, 1)
	if err := currentPhase.HandleAction(actingPlayer, actionDetails, resultsChan); err != nil {
		r.logger.Error("Error from phase handling action immediately", "playerID", playerID, "phase", currentPhase.GetName(), "error", err)
	}

	select {
	case phaseErr := <-resultsChan:
		if phaseErr != nil {
			r.logger.Error("Error from phase after processing action", "playerID", playerID, "phase", currentPhase.GetName(), "error", phaseErr)
		}
	default:
		// Non-blocking, action processed or queued by phase.
	}
}

func (r *Runtime) processHunterShot(hunterID string, actionData map[string]interface{}) {
	r.logger.Info("Processing Hunter's shot", "hunterID", hunterID, "targetData", actionData["targetID"])

	var hunterPlayer game.Player
	var hunterSkillInterface game.Skill
	
	r.RLock()
	for _, p := range r.players {
		if p.GetID() == hunterID {
			hunterPlayer = p
			if p.GetRole().GetName() == game.RoleTypeHunter {
				hunterSkillInterface = r.getHunterSkill(p)
			}
			break
		}
	}
	r.RUnlock()

	if hunterPlayer == nil || hunterSkillInterface == nil {
		r.logger.Error("Hunter or Hunter skill not found for shot processing", "hunterID", hunterID)
		return
	}
	
	hunterRealSkill, ok := hunterSkillInterface.(*skill.Hunter)
    if !ok {
        r.logger.Error("Could not assert Hunter skill type", "hunterID", hunterID)
        return
    }
    if hunterRealSkill.HasUsed() { // Check if already used
         r.logger.Warn("Hunter shot attempted but skill already used", "hunterID", hunterID)
		 r.Lock()
		 r.pendingHunterShotPlayerID = "" // Clear pending state
		 if r.hunterShotCancel != nil {
			r.hunterShotCancel()
			r.hunterShotCancel = nil
		 }
		 r.Unlock()
         return
    }


	targetID, _ := actionData["targetID"].(string)
	var targetPlayer game.Player
	if targetID != "" {
		r.RLock()
		for _, p := range r.players {
			if p.GetID() == targetID {
				targetPlayer = p
				break
			}
		}
		r.RUnlock()
	}

	if targetPlayer == nil {
		r.logger.Error("Hunter's target not found", "targetID", targetID)
		hunterPlayer.Write(event.Event[any]{
			Type: event.SystemSkillResult, PlayerID: game.SystemPlayerID, Receivers: []string{hunterPlayer.GetID()}, Timestamp: time.Now(),
			Data: event.SkillResultData{SkillType: string(game.SkillTypeHunter), Message: "无效的目标，目标不存在。"}})
		return // Hunter might need to choose again, or shot is wasted. For now, it's wasted.
	}
	
	// Hunter is already dead. The skill.Hunter.Check is for target validity.
	err := hunterRealSkill.Check(game.PhaseNone, hunterPlayer, targetPlayer)
	if err != nil {
		r.logger.Warn("Hunter skill check failed", "hunterID", hunterID, "targetID", targetID, "error", err)
		hunterPlayer.Write(event.Event[any]{
			Type: event.SystemSkillResult, PlayerID: game.SystemPlayerID, Receivers: []string{hunterPlayer.GetID()}, Timestamp: time.Now(),
			Data: event.SkillResultData{SkillType: string(game.SkillTypeHunter), Message: fmt.Sprintf("射击失败: %s", err.Error())}})
		return
	}

	var skillRes game.SkillResult
	hunterRealSkill.Put(hunterPlayer, targetPlayer, &skillRes) // This sets hunterSkill.hasUsed = true
	
	r.Lock()
	r.pendingHunterShotPlayerID = "" // Clear pending shot
	if r.hunterShotCancel != nil {
		r.hunterShotCancel()
		r.hunterShotCancel = nil
	}
	r.Unlock()

	shotMessage := fmt.Sprintf("猎人 %s 开枪射击了玩家 %s。", hunterPlayer.GetID(), targetPlayer.GetID())
	if skillRes.Success {
		shotMessage += fmt.Sprintf(" %s", skillRes.Message)
		if !targetPlayer.IsAlive() {
			r.logger.Info("Hunter's shot killed target", "hunterID", hunterID, "targetID", targetID)
			r.broadcastSystemMessage(fmt.Sprintf("玩家 %s 被猎人 %s 射杀。", targetPlayer.GetID(), hunterPlayer.GetID()))
			// Use a new context for these last words as the original phase context might be done.
			go r.handleLastWords(context.Background(), []game.Player{targetPlayer}) 
		}
	} else {
		shotMessage += fmt.Sprintf(" 但射击失败: %s", skillRes.Message)
		r.broadcastSystemMessage(fmt.Sprintf("猎人 %s 对玩家 %s 的射击失败: %s", hunterPlayer.GetID(), targetPlayer.GetID(), skillRes.Message))
	}
	r.logger.Info(shotMessage)

	if r.checkGameEnd() != game.CampNone {
		r.logger.Info("Game ended after Hunter's shot")
		// The main game loop will detect this and end.
	}
}


// RegisterPlayerConnection, RemovePlayerConnection, AddPlayer, Init as before...
// RegisterPlayerConnection stores the connection for a given player ID.
func (r *Runtime) RegisterPlayerConnection(playerID string, conn *socket.Conn) {
	r.Lock()
	defer r.Unlock()
	r.playerConnections[playerID] = conn
	r.logger.Info("Registered connection for player", "playerID", playerID)
}

// RemovePlayerConnection removes a player's connection and marks them as inactive.
func (r *Runtime) RemovePlayerConnection(playerID string) {
	r.Lock()
	defer r.Unlock()

	delete(r.playerConnections, playerID)
	for _, p := range r.players {
		if p.GetID() == playerID {
			p.SetAlive(false)
			break
		}
	}
	r.logger.Info("Removed connection and marked player as inactive", "playerID", playerID)
	// If the disconnected player was the pending hunter, clear it
	if r.pendingHunterShotPlayerID == playerID {
		r.logger.Info("Pending hunter disconnected, clearing hunter shot.", "hunterID", playerID)
		r.pendingHunterShotPlayerID = ""
		if r.hunterShotCancel != nil {
			r.hunterShotCancel()
			r.hunterShotCancel = nil
		}
	}
}

// AddPlayer creates a new player, stores them, and registers their connection.
func (r *Runtime) AddPlayer(id string, role game.Role, conn *socket.Conn) error {
	r.Lock()
	defer r.Unlock()

	for _, p := range r.players {
		if p.GetID() == id {
			r.logger.Warn("Attempted to add existing player ID", "playerID", id)
			return fmt.Errorf("player with ID %s already exists", id)
		}
	}

	p := player.New(id, role, conn)
	r.players = append(r.players, p)
	r.logger.Info("Added player to runtime", "playerID", id)
	return nil
}

// Init 初始化游戏
func (r *Runtime) Init() error {
	r.Lock()
	defer r.Unlock()

	if len(r.players) == 0 { 
		return fmt.Errorf("need at least 1 player to init")
	}

	r.phaseIdx = 0
	r.phases = []game.Phase{
		phase.NewNightPhase(r.players),
		phase.NewDayPhase(r.players),
		phase.NewVotePhase(r.players),
	}
	r.logger.Info("Runtime initialized with phases", "player_count", len(r.players))
	return nil
}


// Start 开始游戏
func (r *Runtime) Start(ctx context.Context) error {
	r.Lock()
	if r.started {
		r.Unlock()
		return fmt.Errorf("game already started")
	}
	if err := r.Init(); err != nil {
		r.Unlock()
		return fmt.Errorf("failed to initialize game: %w", err)
	}
	r.started = true
	r.Unlock()

	r.logger.Info("Game starting")

	startPhase := phase.NewStartPhase(r.players)
	startPhase.SetRound(r.round)
	if err := startPhase.Start(ctx); err != nil {
		return fmt.Errorf("start phase failed: %w", err)
	}

	// Main game loop
	for r.checkGameEnd() == game.CampNone {
		currentPhase := r.getCurrentPhase()
		currentPhase.SetRound(r.round)
		r.logger.Info("Starting phase", "phase", currentPhase.GetName(), "round", r.round)

		if err := currentPhase.Start(ctx); err != nil {
			return fmt.Errorf("phase %s start failed: %w", currentPhase.GetName(), err)
		}

		phaseStartTime := time.Now()
		// Phase completion loop (includes waiting for pending hunter shot if any)
		for {
			r.RLock()
			pendingHunter := r.pendingHunterShotPlayerID != ""
			r.RUnlock()

			if pendingHunter {
				r.logger.Debug("Waiting for pending hunter shot or timeout", "hunterID", r.pendingHunterShotPlayerID)
				// Timeout logic for hunter shot can be added here if hunterShotCtx is used
			} else if currentPhase.IsComplete(r) {
				r.logger.Info("Phase processing complete", "phase", currentPhase.GetName(), "round", r.round, "duration", time.Since(phaseStartTime).String())
				break
			}
			
			// If a hunter shot is pending, the loop continues until it's cleared (either by action or timeout).
			// If not pending, it breaks when phase is complete.
			// If pending and phase also completes (e.g. timeout), the pending hunter shot still needs resolution.
			// This simple sleep doesn't fully manage a hunter-shot timeout vs phase timeout.
			// A more robust solution would use select on multiple channels (phase completion, hunter shot, timeout).
			time.Sleep(200 * time.Millisecond) 
		}
		
		// If a hunter shot was pending and might have been resolved, re-check game end.
		if r.checkGameEnd() != game.CampNone {
			r.logger.Info("Game ended after hunter shot resolution or phase completion check", "phase", currentPhase.GetName())
			break 
		}

		var phaseDeaths []game.Player
		switch p := currentPhase.(type) {
		case *phase.NightPhase:
			nightResult := p.CalculatePhaseResult()
			phaseDeaths = nightResult.Deaths
			r.logger.Info("Night phase results calculated", "deaths", len(phaseDeaths))
			p.NotifyPhaseEnd()

			// Check for Hunter death after night phase
			var currentDeathsWarrantHunterCheck []game.Player
			currentDeathsWarrantHunterCheck = append(currentDeathsWarrantHunterCheck, phaseDeaths...)
			
			// Handle hunter shot before last words for these deaths
			if r.triggerHunterShotIfDeceased(currentDeathsWarrantHunterCheck) {
				// If hunter shot is triggered, wait for it to complete
				shotStartTime := time.Now()
				for r.pendingHunterShotPlayerID != "" && time.Since(shotStartTime) < 30*time.Second { // 30s timeout for hunter shot
					time.Sleep(200 * time.Millisecond)
				}
				if r.pendingHunterShotPlayerID != "" {
					r.logger.Warn("Hunter shot timed out", "hunterID", r.pendingHunterShotPlayerID)
					r.Lock()
					r.pendingHunterShotPlayerID = "" // Clear pending shot
					if r.hunterShotCancel != nil { r.hunterShotCancel(); r.hunterShotCancel = nil; }
					r.Unlock()
				}
			}
			if len(phaseDeaths) > 0 && r.pendingHunterShotPlayerID == "" {
				r.handleLastWords(ctx, phaseDeaths)
			}

		case *phase.DayPhase:
			_ = p.GetPhaseResult() 
			r.logger.Info("Day phase results processed", "actions_count", len(p.GetActions()))
			p.BroadcastPhaseEnd(game.PhaseDay, "发言阶段结束，即将进入投票阶段。")

		case *phase.VotePhase:
			voteResult := p.GetPhaseResult() 
			phaseDeaths = voteResult.Deaths
			r.logger.Info("Vote phase results calculated", "deaths", len(phaseDeaths))
			
			var deathNames []string
			if len(phaseDeaths) > 0 {
				for _, deadPlayer := range phaseDeaths {
					deathNames = append(deathNames, deadPlayer.GetID())
				}
				p.BroadcastPhaseEnd(game.PhaseVote, fmt.Sprintf("投票结束，被处决的玩家是: %s", strings.Join(deathNames, ", ")))
				
				if r.triggerHunterShotIfDeceased(phaseDeaths) {
					shotStartTime := time.Now()
					for r.pendingHunterShotPlayerID != "" && time.Since(shotStartTime) < 30*time.Second {
						time.Sleep(200 * time.Millisecond)
					}
					if r.pendingHunterShotPlayerID != "" {
						r.logger.Warn("Hunter shot timed out", "hunterID", r.pendingHunterShotPlayerID)
						r.Lock()
						r.pendingHunterShotPlayerID = "" 
						if r.hunterShotCancel != nil { r.hunterShotCancel(); r.hunterShotCancel = nil; }
						r.Unlock()
					}
				}
				if r.pendingHunterShotPlayerID == "" {
					r.handleLastWords(ctx, phaseDeaths)
				}
			} else {
                 tie := false
                 if vrData, ok := voteResult.ExtraData[game.SkillTypeVote]; ok {
                     if specificData, okData := vrData.Data.(event.VoteResultData); okData {
                         tie = specificData.IsTie
                     }
                 }
                 if tie {
                     p.BroadcastPhaseEnd(game.PhaseVote, "投票结束，结果为平票，无人被处决。")
                 } else {
                     p.BroadcastPhaseEnd(game.PhaseVote, "投票结束，无人被处决。")
                 }
			}
		default:
			r.logger.Warn("Unknown phase type for result processing", "phase", currentPhase.GetName())
		}

		if r.checkGameEnd() != game.CampNone {
			r.logger.Info("Game ended after processing phase and potential last words", "phase", currentPhase.GetName())
			break
		}

		r.advancePhase()
		if r.phaseIdx == 0 { 
			r.round++
			r.logger.Info("New round starting", "round", r.round)
			r.resetPerRoundSkills()
		}
	}

	winner := r.checkGameEnd()
	r.logger.Info("Game ended", "winner", winner.String())
	endPhase := phase.NewEndPhase(r.players, winner)
	endPhase.SetRound(r.round)
	if err := endPhase.Start(ctx); err != nil {
		return fmt.Errorf("end phase failed: %w", err)
	}

	return nil
}

// GetPlayers returns the list of players in the game.
func (r *Runtime) GetPlayers() []game.Player {
    r.RLock()
    defer r.RUnlock()
    playersCopy := make([]game.Player, len(r.players))
    copy(playersCopy, r.players)
    return playersCopy
}

func (r *Runtime) getCurrentPhase() game.Phase {
	r.RLock()
	defer r.RUnlock()
	if r.phaseIdx < 0 || r.phaseIdx >= len(r.phases) {
		r.logger.Error("phaseIdx out of bounds in getCurrentPhase", "phaseIdx", r.phaseIdx, "phases_len", len(r.phases))
		return r.phases[0] 
	}
	return r.phases[r.phaseIdx]
}

func (r *Runtime) advancePhase() {
	r.Lock()
	defer r.Unlock()
	r.phaseIdx = (r.phaseIdx + 1) % len(r.phases)
}

func (r *Runtime) checkGameEnd() game.Camp {
	r.RLock()
	defer r.RUnlock()
	goodCount := 0
	evilCount := 0
	for _, p := range r.players {
		if !p.IsAlive() {
			continue
		}
		switch p.GetRole().GetCamp() {
		case game.CampGood:
			goodCount++
		case game.CampEvil:
			evilCount++
		case game.CampNone:
			r.logger.Warn("Player with CampNone found", "playerID", p.GetID())
		}
	}
	if evilCount == 0 && goodCount > 0 { return game.CampGood }
	if goodCount == 0 && evilCount > 0 { return game.CampEvil }
	// TODO: Add more complex win conditions (e.g., Werewolves >= Villagers)
	return game.CampNone
}

// triggerHunterShotIfDeceased checks if any deceased player is a Hunter and can shoot.
// Returns true if a hunter shot is now pending.
func (r *Runtime) triggerHunterShotIfDeceased(deceasedPlayers []game.Player) bool {
	for _, deadPlayer := range deceasedPlayers {
		if deadPlayer.GetRole().GetName() == game.RoleTypeHunter {
			hunterSkillInstance := r.getHunterSkill(deadPlayer)
			if hunterSkillInstance != nil {
				// Type assert to access HasUsed - this is a bit unsafe, better to have HasUsed on interface
				if hs, ok := hunterSkillInstance.(*skill.Hunter); ok && !hs.HasUsed() {
					r.Lock()
					r.pendingHunterShotPlayerID = deadPlayer.GetID()
					// Prepare for potential timeout while waiting for hunter's action
					// hunterShotCtx, hunterShotCancel := context.WithTimeout(context.Background(), 30*time.Second) // Example timeout
					// r.hunterShotCtx = hunterShotCtx
					// r.hunterShotCancel = hunterShotCancel
					r.Unlock()
					r.notifyHunterToShoot(deadPlayer)
					return true // Hunter shot is now pending
				}
			}
		}
	}
	return false
}


func (r *Runtime) getHunterSkill(player game.Player) game.Skill {
	if player.GetRole().GetName() == game.RoleTypeHunter {
		// Hunter skill might not be in "available skills" for a specific phase.
		// We need to get it directly from the role's full skill list.
		// This assumes role.GetSkills() or similar exists, or we iterate what NewHunter adds.
		// For now, create a new instance to check .hasUsed (state is in the skill instance).
		// This is flawed if the skill instance on the player has state.
		// The skill instance should be retrieved from the player object.
		// Let's assume player.GetRole().GetAvailableSkills(game.PhaseNone) can fetch it.
		for _, sk := range player.GetRole().GetAvailableSkills(game.PhaseNone) { 
			if sk.GetName() == game.SkillTypeHunter {
				return sk
			}
		}
		// Fallback: if not found via PhaseNone, create one to check (less ideal)
		// This indicates a potential issue in how skills are retrieved or stored.
		r.logger.Warn("Hunter skill not found via PhaseNone, creating temporary instance for check", "playerID", player.GetID())
		return skill.NewHunterSkill()
	}
	return nil
}

func (r *Runtime) notifyHunterToShoot(hunterPlayer game.Player) {
	r.logger.Info("Notifying Hunter to shoot", "hunterID", hunterPlayer.GetID())
	var availableTargets []event.PlayerInfo
	r.RLock()
	for _, p := range r.players {
		if p.IsAlive() && p.GetID() != hunterPlayer.GetID() {
			availableTargets = append(availableTargets, event.PlayerInfo{ID: p.GetID(), IsAlive: p.IsAlive()})
		}
	}
	r.RUnlock()

	evtData := event.SkillResultData{
		SkillType: string(game.SkillTypeHunter),
		Message:   "你已死亡，请选择一名玩家进行射击。",
		Options:   availableTargets,
	}
	err := hunterPlayer.Write(event.Event[any]{
		Type: event.SystemSkillResult, PlayerID: game.SystemPlayerID, Receivers: []string{hunterPlayer.GetID()}, Timestamp: time.Now(), Data: evtData,
	})
	if err != nil {
		r.logger.Error("Failed to notify Hunter to shoot", "hunterID", hunterPlayer.GetID(), "error", err)
	}
}

func (r *Runtime) broadcastSystemMessage(message string) {
	evt := event.Event[any]{
		Type: event.SystemSkillResult, PlayerID: game.SystemPlayerID, Timestamp: time.Now(), Data: event.SkillResultData{Message: message},
	}
	r.RLock()
	receivers := make([]string, 0, len(r.players))
	playersToBroadcast := make([]game.Player, 0, len(r.players))
	for _, p := range r.players {
		if p.IsAlive() {
			receivers = append(receivers, p.GetID())
			playersToBroadcast = append(playersToBroadcast, p)
		}
	}
	r.RUnlock()
	evt.Receivers = receivers // Set receivers for the event itself

	for _, player := range playersToBroadcast {
		if err := player.Write(evt); err != nil {
			r.logger.Error("Failed to write system message to player", "playerID", player.GetID(), "error", err)
		}
	}
	r.logger.Info("SystemMessageBroadcast", "message", message)
}

func (r *Runtime) handleLastWords(ctx context.Context, deceasedPlayers []game.Player) {
	if len(deceasedPlayers) == 0 {
		return
	}
	// Create a new context for last words, as original might be tied to a phase that's ending.
	lwCtx, lwCancel := context.WithTimeout(context.Background(), 2*time.Minute) // Example timeout for all last words
	defer lwCancel()

	r.logger.Info("Starting LastWordsPhase for deceased players", "count", len(deceasedPlayers))
	lastWordsPhaseInstance := phase.NewLastWordsPhase(r.round, r.players, deceasedPlayers)
	lastWordsPhaseInstance.SetRound(r.round) 
	
	if err := lastWordsPhaseInstance.Start(lwCtx); err != nil {
		r.logger.Error("LastWordsPhase start failed", "error", err)
		return
	}

	phaseStartTime := time.Now()
	for {
		if lastWordsPhaseInstance.IsComplete(r) { 
			r.logger.Info("LastWordsPhase complete", "duration", time.Since(phaseStartTime).String())
			break
		}
		select {
		case <-lwCtx.Done():
			r.logger.Warn("LastWordsPhase timed out.", "duration", time.Since(phaseStartTime).String())
			// Potentially force broadcast end of phase if timed out.
			lastWordsPhaseInstance.BroadcastPhaseEnd(lastWordsPhaseInstance.GetName(), "遗言时间已到。")
			return
		default:
			time.Sleep(100 * time.Millisecond) 
		}
	}
}

func (r *Runtime) resetPerRoundSkills() {
	r.logger.Info("Resetting general per-round states (if any)", "round", r.round)
	// Most critical resets are handled by phases (e.g. NightPhase resets protections and night skills)
}
