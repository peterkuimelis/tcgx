package game

import (
	"strings"
	"testing"

	"github.com/peterkuimelis/tcgx/internal/log"
)

// TestGreedProtocol: Activate Pot of Greed, draw 2 cards, goes to Scrapheap.
func TestGreedProtocol(t *testing.T) {
	potOfGreed := GreedProtocol()
	agent := vanillaAgent("Warrior", 4, 1500, 1000, AttrEARTH)

	// P1 initial hand: potOfGreed, agent, filler, filler, filler. T1 draw: filler.
	deck0 := makePaddedDeck([]*Card{potOfGreed, agent}, 40)
	deck1 := makePaddedDeck([]*Card{}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Turn 1 (P1): Activate Pot of Greed
	p0.AddAction(ActionActivate, "Greed Protocol")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 3}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify: Pot of Greed activated
	activates := logger.EventsOfType(log.EventActivate)
	found := false
	for _, e := range activates {
		if e.Card == "Greed Protocol" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Pot of Greed activation event")
	}

	// Verify: 2 draws from Pot of Greed (during Main Phase, not Draw Phase)
	draws := logger.EventsOfType(log.EventDraw)
	mainPhaseDraws := 0
	for _, e := range draws {
		if e.Player == 0 && e.Phase == "Main Phase 1" {
			mainPhaseDraws++
		}
	}
	if mainPhaseDraws != 2 {
		t.Errorf("Expected 2 draws from Pot of Greed, got %d", mainPhaseDraws)
	}

	// Verify: Pot of Greed sent to Scrapheap
	scrapheapEvents := logger.EventsOfType(log.EventSendToScrapheap)
	found = false
	for _, e := range scrapheapEvents {
		if e.Card == "Greed Protocol" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Pot of Greed to be sent to Scrapheap after resolving")
	}
}

// TestVoidPurge: Both sides have agents, all destroyed.
func TestVoidPurge(t *testing.T) {
	darkHole := VoidPurge()
	warrior := vanillaAgent("Warrior", 4, 1500, 1000, AttrEARTH)
	knight := vanillaAgent("Knight", 4, 1600, 1200, AttrLIGHT)

	// P1 initial hand: warrior + 4 filler. T1 draw: filler. T3 draw: Dark Hole.
	f := vanillaAgent("Filler X", 1, 0, 0, AttrLIGHT)
	deck0 := makePaddedDeck([]*Card{warrior, f, f, f, f, f, darkHole}, 40)
	deck1 := makePaddedDeck([]*Card{knight}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Turn 1 (P1): Summon Warrior
	p0.AddAction(ActionNormalSummon, "Warrior")

	// Turn 2 (P2): Summon Knight
	p1.AddAction(ActionNormalSummon, "Knight")

	// Turn 3 (P1): Draws Dark Hole, activates it — both agents on field
	p0.AddAction(ActionActivate, "Void Purge")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify: both agents destroyed
	destroys := logger.EventsOfType(log.EventDestroy)
	names := make(map[string]bool)
	for _, e := range destroys {
		names[e.Card] = true
	}
	if !names["Warrior"] {
		t.Error("Expected Warrior to be destroyed by Dark Hole")
	}
	if !names["Knight"] {
		t.Error("Expected Knight to be destroyed by Dark Hole")
	}
}

// TestEMPCascadeMSTChain: P1 activates Heavy Storm [CL1], P2 chains MST [CL2], LIFO resolution.
func TestEMPCascadeMSTChain(t *testing.T) {
	heavyStorm := EMPCascade()
	mst := ICEBreaker()

	// P1 has set Filler Trap, P2 has set MST.
	// P1 activates Heavy Storm → P2 chains MST targeting Filler Trap.
	// LIFO: MST resolves (destroys Filler Trap), then Heavy Storm resolves (destroys remaining tech).
	fillerTrap := &Card{Name: "Filler Trap", CardType: CardTypeTrap, TrapSub: TrapNormal}
	fl := vanillaAgent("Filler Y", 1, 0, 0, AttrLIGHT)

	// P1: Filler Trap in initial hand, Heavy Storm drawn Turn 3 (7th card).
	// P2: MST in initial hand.
	deck0 := makePaddedDeck([]*Card{fillerTrap, fl, fl, fl, fl, fl, heavyStorm}, 40)
	deck1 := makePaddedDeck([]*Card{mst}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Turn 1 (P1): Set Filler Trap
	p0.AddAction(ActionSetTech, "Filler Trap")

	// Turn 2 (P2): Set MST
	p1.AddAction(ActionSetTech, "ICE Breaker")

	// Turn 3 (P1): Draws Heavy Storm. Activate Heavy Storm → P2 chains MST targeting Filler Trap
	p0.AddAction(ActionActivate, "EMP Cascade")
	// In the response window, P2 should activate MST
	p1.AddAction(ActionActivate, "ICE Breaker")
	// MST target: choose Filler Trap (P1's set card)
	p1.AddCardChoice("Filler Trap")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify chain links: CL1 = Heavy Storm, CL2 = MST
	chainLinks := logger.EventsOfType(log.EventChainLink)
	if len(chainLinks) < 2 {
		t.Fatalf("Expected at least 2 chain links, got %d", len(chainLinks))
	}
	if chainLinks[0].Card != "EMP Cascade" {
		t.Errorf("CL1 should be Heavy Storm, got %s", chainLinks[0].Card)
	}
	if chainLinks[1].Card != "ICE Breaker" {
		t.Errorf("CL2 should be Mystical Space Typhoon, got %s", chainLinks[1].Card)
	}

	// Verify LIFO resolution: CL2 (MST) resolves before CL1 (Heavy Storm)
	chainResolves := logger.EventsOfType(log.EventChainResolve)
	if len(chainResolves) < 2 {
		t.Fatalf("Expected at least 2 chain resolves, got %d", len(chainResolves))
	}
	if chainResolves[0].Card != "ICE Breaker" {
		t.Errorf("First resolve should be MST, got %s", chainResolves[0].Card)
	}
	if chainResolves[1].Card != "EMP Cascade" {
		t.Errorf("Second resolve should be Heavy Storm, got %s", chainResolves[1].Card)
	}
}

// TestReflectorArray: P1 attacks, P2 Mirror Force, all P1 ATK agents destroyed.
func TestReflectorArray(t *testing.T) {
	mirrorForce := ReflectorArray()
	warrior := vanillaAgent("Warrior", 4, 1500, 1000, AttrEARTH)
	knight := vanillaAgent("Knight", 4, 1600, 1200, AttrLIGHT)

	deck0 := makePaddedDeck([]*Card{warrior, knight}, 40)
	deck1 := makePaddedDeck([]*Card{mirrorForce}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Turn 1 (P1): Summon Warrior
	p0.AddAction(ActionNormalSummon, "Warrior")

	// Turn 2 (P2): Set Mirror Force
	p1.AddAction(ActionSetTech, "Reflector Array")

	// Turn 3 (P1): Summon Knight, enter battle, attack with Warrior
	p0.AddAction(ActionNormalSummon, "Knight")
	p0.AddAction(ActionEnterBattlePhase, "")
	// P1 attacks directly with Warrior (P2 has no agents)
	p0.AddDirectAttack("Warrior")
	// P2 activates Mirror Force in response window
	p1.AddAction(ActionActivate, "Reflector Array")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify: Mirror Force activated
	activates := logger.EventsOfType(log.EventActivate)
	found := false
	for _, e := range activates {
		if e.Card == "Reflector Array" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Mirror Force activation")
	}

	// Verify: Both Warrior and Knight destroyed
	destroys := logger.EventsOfType(log.EventDestroy)
	names := make(map[string]bool)
	for _, e := range destroys {
		names[e.Card] = true
	}
	if !names["Warrior"] {
		t.Error("Expected Warrior to be destroyed by Mirror Force")
	}
	if !names["Knight"] {
		t.Error("Expected Knight to be destroyed by Mirror Force")
	}
}

// TestReactivePlating: P1 attacks, P2 Sakuretsu, attacker destroyed.
func TestReactivePlating(t *testing.T) {
	sakuretsu := ReactivePlating()
	warrior := vanillaAgent("Warrior", 4, 1500, 1000, AttrEARTH)

	deck0 := makePaddedDeck([]*Card{warrior}, 40)
	deck1 := makePaddedDeck([]*Card{sakuretsu}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Turn 1 (P1): Summon Warrior
	p0.AddAction(ActionNormalSummon, "Warrior")

	// Turn 2 (P2): Set Sakuretsu Armor
	p1.AddAction(ActionSetTech, "Reactive Plating")

	// Turn 3 (P1): Enter battle, attack directly
	p0.AddAction(ActionEnterBattlePhase, "")
	p0.AddDirectAttack("Warrior")
	// P2 activates Sakuretsu Armor in response
	p1.AddAction(ActionActivate, "Reactive Plating")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify: Warrior destroyed by Sakuretsu
	destroys := logger.EventsOfType(log.EventDestroy)
	found := false
	for _, e := range destroys {
		if e.Card == "Warrior" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Warrior to be destroyed by Sakuretsu Armor")
	}
}

// TestCascadeFailure: P1 summons, P2 Torrential, all agents destroyed.
func TestCascadeFailure(t *testing.T) {
	torrential := CascadeFailure()
	warrior := vanillaAgent("Warrior", 4, 1500, 1000, AttrEARTH)
	knight := vanillaAgent("Knight", 4, 1600, 1200, AttrLIGHT)

	deck0 := makePaddedDeck([]*Card{warrior, knight}, 40)
	deck1 := makePaddedDeck([]*Card{torrential}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Turn 1 (P1): Summon Warrior
	p0.AddAction(ActionNormalSummon, "Warrior")

	// Turn 2 (P2): Set Cascade Failure
	p1.AddAction(ActionSetTech, "Cascade Failure")

	// Turn 3 (P1): Summon Knight → Torrential triggers (P2 says yes to optional)
	p0.AddAction(ActionNormalSummon, "Knight")
	p1.AddYesNo(true) // Yes, activate Torrential

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify: All agents destroyed
	destroys := logger.EventsOfType(log.EventDestroy)
	names := make(map[string]bool)
	for _, e := range destroys {
		names[e.Card] = true
	}
	if !names["Warrior"] {
		t.Error("Expected Warrior to be destroyed by Cascade Failure")
	}
	if !names["Knight"] {
		t.Error("Expected Knight to be destroyed by Cascade Failure")
	}
}

// TestSelfDestructCircuit: Target agent destroyed, both players take damage.
func TestSelfDestructCircuit(t *testing.T) {
	ring := SelfDestructCircuit()
	warrior := vanillaAgent("Warrior", 4, 1500, 1000, AttrEARTH)

	deck0 := makePaddedDeck([]*Card{warrior}, 40)
	deck1 := makePaddedDeck([]*Card{ring}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Turn 1 (P1): Summon Warrior
	p0.AddAction(ActionNormalSummon, "Warrior")

	// Turn 2 (P2): Set Ring of Destruction
	p1.AddAction(ActionSetTech, "Self-Destruct Circuit")

	// Turn 3 (P1): Enter battle, attack directly, P2 activates Ring in response
	p0.AddAction(ActionEnterBattlePhase, "")
	p0.AddDirectAttack("Warrior")
	p1.AddAction(ActionActivate, "Self-Destruct Circuit")
	p1.AddCardChoice("Warrior")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify: Warrior destroyed
	destroys := logger.EventsOfType(log.EventDestroy)
	found := false
	for _, e := range destroys {
		if e.Card == "Warrior" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Warrior to be destroyed by Ring of Destruction")
	}

	// Verify: Both players took 1500 damage (Warrior's ATK)
	lpChanges := logger.EventsOfType(log.EventHPChange)
	p1Damage := false
	p2Damage := false
	for _, e := range lpChanges {
		if e.Player == 0 && strings.Contains(e.Details, "Self-Destruct Circuit") {
			p1Damage = true
		}
		if e.Player == 1 && strings.Contains(e.Details, "Self-Destruct Circuit") {
			p2Damage = true
		}
	}
	if !p1Damage {
		t.Error("Expected P1 to take 1500 damage from Ring of Destruction")
	}
	if !p2Damage {
		t.Error("Expected P2 to take 1500 damage from Ring of Destruction")
	}
}

// TestRootOverride: Negate a program activation, HP halved.
func TestRootOverride(t *testing.T) {
	solemn := RootOverride()
	potOfGreed := GreedProtocol()
	fl := vanillaAgent("Filler Z", 1, 0, 0, AttrLIGHT)

	// Pot of Greed drawn on Turn 3 (7th from top).
	deck0 := makePaddedDeck([]*Card{fl, fl, fl, fl, fl, fl, potOfGreed}, 40)
	deck1 := makePaddedDeck([]*Card{solemn}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Turn 2 (P2): Set Solemn Judgment
	p1.AddAction(ActionSetTech, "Root Override")

	// Turn 3 (P1): Draws Pot of Greed. Activate it → P2 chains Solemn Judgment
	p0.AddAction(ActionActivate, "Greed Protocol")
	p1.AddAction(ActionActivate, "Root Override")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify: Solemn Judgment activated
	activates := logger.EventsOfType(log.EventActivate)
	found := false
	for _, e := range activates {
		if e.Card == "Root Override" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Solemn Judgment activation")
	}

	// Verify: P2's HP was halved (8192 → 4096)
	lpChanges := logger.EventsOfType(log.EventHPChange)
	costFound := false
	for _, e := range lpChanges {
		if e.Player == 1 {
			costFound = true
			break
		}
	}
	if !costFound {
		t.Error("Expected P2 to pay HP cost for Solemn Judgment")
	}

	// Verify: Pot of Greed was destroyed/negated
	destroys := logger.EventsOfType(log.EventDestroy)
	potDestroyed := false
	for _, e := range destroys {
		if e.Card == "Greed Protocol" {
			potDestroyed = true
			break
		}
	}
	if !potDestroyed {
		t.Error("Expected Pot of Greed to be destroyed by Solemn Judgment")
	}

	// Verify: Pot of Greed's draw effect was actually negated (P1 drew 0 cards in Main Phase)
	draws := logger.EventsOfType(log.EventDraw)
	mainPhaseDraws := 0
	for _, e := range draws {
		if e.Player == 0 && e.Phase == "Main Phase 1" {
			mainPhaseDraws++
		}
	}
	if mainPhaseDraws != 0 {
		t.Errorf("Expected Pot of Greed to be negated (0 main phase draws), got %d", mainPhaseDraws)
	}
}

// TestBlackoutPatch: Flip a agent face-down.
func TestBlackoutPatch(t *testing.T) {
	bookOfMoon := BlackoutPatch()
	warrior := vanillaAgent("Warrior", 4, 1500, 1000, AttrEARTH)

	deck0 := makePaddedDeck([]*Card{bookOfMoon, warrior}, 40)
	deck1 := makePaddedDeck([]*Card{}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Turn 1 (P1): Summon Warrior
	p0.AddAction(ActionNormalSummon, "Warrior")

	// Turn 3 (P1): Activate Book of Moon targeting Warrior
	p0.AddAction(ActionActivate, "Blackout Patch")
	p0.AddCardChoice("Warrior")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify: Book of Moon activated
	activates := logger.EventsOfType(log.EventActivate)
	found := false
	for _, e := range activates {
		if e.Card == "Blackout Patch" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Book of Moon activation")
	}

	// Verify: Warrior flipped face-down
	flipDowns := logger.EventsOfType(log.EventFlipFaceDown)
	found = false
	for _, e := range flipDowns {
		if e.Card == "Warrior" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Warrior to be flipped face-down by Book of Moon")
	}
}

// TestExecSpeedValidation: Verify ES2 can't chain to ES3.
func TestExecSpeedValidation(t *testing.T) {
	// ES2 cannot chain to ES3, only ES3 can
	if canChainWith(ExecSpeed3, ExecSpeed2) {
		t.Error("ES2 should not be able to chain to ES3")
	}
	if !canChainWith(ExecSpeed3, ExecSpeed3) {
		t.Error("ES3 should be able to chain to ES3")
	}
	if !canChainWith(ExecSpeed1, ExecSpeed2) {
		t.Error("ES2 should be able to chain to ES1")
	}
	if !canChainWith(ExecSpeed2, ExecSpeed2) {
		t.Error("ES2 should be able to chain to ES2")
	}
	if canChainWith(ExecSpeed2, ExecSpeed1) {
		t.Error("ES1 should not be able to chain to ES2")
	}
}

// TestBreakerProgramCounter: Summoning Breaker triggers a mandatory effect that adds a program counter (+300 ATK).
func TestBreakerProgramCounter(t *testing.T) {
	breaker := BreakerTheChromeWarrior()
	filler := vanillaAgent("Filler", 4, 0, 0, AttrEARTH)

	deck0 := makePaddedDeck([]*Card{breaker}, 40)
	deck1 := makePaddedDeck([]*Card{filler}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// T1: P1 summons Breaker — program counter trigger fires and resolves
	p0.AddAction(ActionNormalSummon, "Breaker the Chrome Warrior")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	summons := logger.EventsOfType(log.EventNormalSummon)
	found := false
	for _, e := range summons {
		if e.Card == "Breaker the Chrome Warrior" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Breaker the Magical Warrior normal summon event")
	}

	// Verify: Breaker's program counter trigger resolved
	resolves := logger.EventsOfType(log.EventChainResolve)
	breakerResolved := false
	for _, e := range resolves {
		if e.Card == "Breaker the Chrome Warrior" {
			breakerResolved = true
			break
		}
	}
	if !breakerResolved {
		t.Error("Expected chain resolve for Breaker's program counter trigger")
	}
}

// TestEquipDestroyedWithAgent: An equip card goes to Scrapheap when its equipped agent is destroyed.
// P1 revives Warrior with Premature Burial, then P2 destroys it with Dark Hole — both Warrior
// and Premature Burial should end up in the Scrapheap.
func TestEquipDestroyedWithAgent(t *testing.T) {
	premature := EmergencyReboot()
	warrior := vanillaAgent("Warrior", 4, 1500, 1000, AttrEARTH)
	darkHole1 := VoidPurge()
	darkHole2 := VoidPurge()

	// P1 hand: warrior, darkHole1, premature (all in initial hand)
	// P2 hand: darkHole2 (in initial hand, used T4 to destroy equipped agent)
	deck0 := makePaddedDeck([]*Card{warrior, darkHole1, premature}, 40)
	deck1 := makePaddedDeck([]*Card{darkHole2}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// T1: P1 summons Warrior, then Dark Holes it to Scrapheap
	p0.AddAction(ActionNormalSummon, "Warrior")
	p0.AddAction(ActionActivate, "Void Purge")
	// T1: P1 activates Premature Burial to revive Warrior (now warrior + equip on field)
	p0.AddAction(ActionActivate, "Emergency Reboot")
	p0.AddCardChoice("Warrior")
	// T2: P2 activates Dark Hole — destroys Warrior, equip lifecycle destroys Premature Burial
	p1.AddAction(ActionActivate, "Void Purge")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify warrior was special summoned by Premature Burial
	specials := logger.EventsOfType(log.EventSpecialSummon)
	found := false
	for _, e := range specials {
		if e.Card == "Warrior" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Warrior to be special summoned by Premature Burial")
	}

	// Verify P2's Dark Hole destroyed the equipped Warrior
	destroys := logger.EventsOfType(log.EventDestroy)
	warriorDestroyedByVoidPurge := false
	for _, e := range destroys {
		if e.Card == "Warrior" && strings.Contains(e.Details, "Void Purge") && e.Turn >= 2 {
			warriorDestroyedByVoidPurge = true
			break
		}
	}
	if !warriorDestroyedByVoidPurge {
		t.Error("Expected equipped Warrior to be destroyed by P2's Dark Hole")
	}

	// Verify Premature Burial was destroyed as a result of the equipped agent leaving the field
	prematureDestroyed := false
	for _, e := range destroys {
		if e.Card == "Emergency Reboot" && strings.Contains(e.Details, "equipped agent left field") {
			prematureDestroyed = true
			break
		}
	}
	if !prematureDestroyed {
		t.Error("Expected Premature Burial to be destroyed when equipped agent left the field")
	}
}

// TestFlipEffect: Magician of Faith flip effect recovers a program from Scrapheap.
func TestFlipEffect(t *testing.T) {
	mof := Datamancer()
	potOfGreed := GreedProtocol()

	// P1: MoF and Pot of Greed in initial hand
	deck0 := makePaddedDeck([]*Card{mof, potOfGreed}, 40)
	deck1 := makePaddedDeck([]*Card{}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// T1: Set MoF face-down, activate Pot of Greed (sends PoG to Scrapheap after resolution)
	p0.AddAction(ActionNormalSet, "Datamancer")
	p0.AddAction(ActionActivate, "Greed Protocol")
	// T3: Flip summon MoF → triggers flip effect, recover Pot of Greed from Scrapheap
	p0.AddAction(ActionFlipSummon, "Datamancer")
	p0.AddYesNo(true)                  // yes, activate MoF effect
	p0.AddCardChoice("Greed Protocol") // choose Pot of Greed from Scrapheap

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 6}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify Pot of Greed was added to hand
	addEvents := logger.EventsOfType(log.EventAddToHand)
	found := false
	for _, e := range addEvents {
		if e.Card == "Greed Protocol" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Pot of Greed to be added to hand by Magician of Faith")
	}
}

// TestIgnitionEffect: Breaker removes program counter to destroy a set tech.
func TestIgnitionEffect(t *testing.T) {
	breaker := BreakerTheChromeWarrior()
	mst := ICEBreaker()

	// P1: Breaker in initial hand. P2: MST in initial hand (will be set).
	deck0 := makePaddedDeck([]*Card{breaker}, 40)
	deck1 := makePaddedDeck([]*Card{mst}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// T1: P1 summons Breaker (gains program counter via trigger)
	p0.AddAction(ActionNormalSummon, "Breaker the Chrome Warrior")
	// T2: P2 sets MST
	p1.AddAction(ActionSetTech, "ICE Breaker")
	// T3: P1 activates Breaker's ignition effect to destroy MST
	p0.AddAction(ActionActivate, "Breaker the Chrome Warrior")
	p0.AddCardChoice("ICE Breaker")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 6}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	destroyEvents := logger.EventsOfType(log.EventDestroy)
	found := false
	for _, e := range destroyEvents {
		if e.Card == "ICE Breaker" && strings.Contains(e.Details, "Breaker") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected MST to be destroyed by Breaker's effect")
	}
}

// TestContinuousTrapReviveAndLinkedDestruction: Call of the Haunted revives a agent.
// Then MST destroys CotH in a separate chain, which triggers linked destruction of the revived agent.
func TestContinuousTrapReviveAndLinkedDestruction(t *testing.T) {
	coth := ResurrectionProtocol()
	warrior := vanillaAgent("Warrior", 4, 1500, 1000, AttrEARTH)
	darkHole := VoidPurge()
	mst := ICEBreaker()
	filler := vanillaAgent("Filler", 1, 0, 0, AttrLIGHT)

	// P1 initial hand: warrior, coth, darkHole, filler, filler
	// T1 draw: filler. T3 draw: filler. T5 draw: mst.
	// MST is drawn on T5 so it can't accidentally chain to CotH's activation on T3.
	deck0 := makePaddedDeck([]*Card{warrior, coth, darkHole, filler, filler, filler, filler, mst}, 40)
	deck1 := makePaddedDeck([]*Card{}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// T1: Summon warrior, Dark Hole it (to Scrapheap), set CotH
	p0.AddAction(ActionNormalSummon, "Warrior")
	p0.AddAction(ActionActivate, "Void Purge")
	p0.AddAction(ActionSetTech, "Resurrection Protocol")
	// T3: Activate CotH (trap, set since T1) to revive warrior — equip link established
	p0.AddAction(ActionActivate, "Resurrection Protocol")
	p0.AddCardChoice("Warrior")
	// T5: Draw MST, activate it targeting CotH — CotH destroyed → warrior also destroyed (linked)
	p0.AddAction(ActionActivate, "ICE Breaker")
	p0.AddCardChoice("Resurrection Protocol")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 8}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify warrior was special summoned by CotH
	specials := logger.EventsOfType(log.EventSpecialSummon)
	found := false
	for _, e := range specials {
		if e.Card == "Warrior" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Warrior to be special summoned by Call of the Haunted")
	}

	// Verify CotH was destroyed by MST
	destroyEvents := logger.EventsOfType(log.EventDestroy)
	cothDestroyed := false
	for _, e := range destroyEvents {
		if e.Card == "Resurrection Protocol" {
			cothDestroyed = true
			break
		}
	}
	if !cothDestroyed {
		t.Error("Expected Call of the Haunted to be destroyed by MST")
	}

	// Verify warrior was destroyed as a result of CotH leaving the field (linked destruction)
	warriorDestroyCount := 0
	for _, e := range destroyEvents {
		if e.Card == "Warrior" {
			warriorDestroyCount++
		}
	}
	if warriorDestroyCount < 2 {
		t.Errorf("Expected Warrior destroyed at least twice (Dark Hole + CotH linked), got %d", warriorDestroyCount)
	}
}

// TestBLSSpecialSummon: Chrome Paladin purgees 1 LIGHT + 1 DARK from Scrapheap to special summon.
func TestBLSSpecialSummon(t *testing.T) {
	bls := ChromePaladinEnvoy()
	lightAgent := vanillaAgent("Angel", 4, 1500, 1000, AttrLIGHT)
	darkAgent := vanillaAgent("Fiend", 4, 1400, 1200, AttrDARK)
	darkHole := VoidPurge()

	// P1 hand: Angel, Fiend, Void Purge, Chrome Paladin (all in initial hand)
	// T1: Summon Angel. T3: Summon Fiend. T5: Void Purge (both to Scrapheap). T5: Special summon Chrome Paladin.
	deck0 := makePaddedDeck([]*Card{lightAgent, darkAgent, darkHole, bls}, 40)
	deck1 := makePaddedDeck([]*Card{}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	p0.AddAction(ActionNormalSummon, "Angel")
	p0.AddAction(ActionNormalSummon, "Fiend")
	p0.AddAction(ActionActivate, "Void Purge")
	// Special summon Chrome Paladin (LIGHT + DARK now in Scrapheap)
	p0.AddAction(ActionActivate, "Chrome Paladin - Envoy of Genesis")
	p0.AddCardChoice("Angel") // purge LIGHT
	p0.AddCardChoice("Fiend") // purge DARK

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 8}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	specials := logger.EventsOfType(log.EventSpecialSummon)
	found := false
	for _, e := range specials {
		if strings.Contains(e.Card, "Chrome Paladin") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Chrome Paladin to be special summoned")
	}

	purgeEvents := logger.EventsOfType(log.EventPurge)
	if len(purgeEvents) < 2 {
		t.Errorf("Expected at least 2 purge events (LIGHT + DARK), got %d", len(purgeEvents))
	}
}

// TestBattleReplay: Attack declared, defender removed by trap response, attacker gets replay.
func TestBattleReplay(t *testing.T) {
	warrior := vanillaAgent("Warrior", 4, 1500, 1000, AttrEARTH)
	knight := vanillaAgent("Knight", 4, 1200, 1000, AttrLIGHT)
	goblin := vanillaAgent("Goblin", 4, 1000, 800, AttrDARK)
	filler := vanillaAgent("Filler", 1, 0, 0, AttrLIGHT)

	// Custom trap: destroys the current defender when opponent attacks
	defenderDestruct := &CardEffect{
		Name:      "Defender Destruction",
		ExecSpeed: ExecSpeed2,
		CanActivate: func(d *Duel, card *CardInstance, player int) bool {
			gs := d.State
			return gs.CurrentAttacker != nil && gs.CurrentTarget != nil &&
				gs.CurrentAttacker.Controller != player
		},
		Resolve: func(d *Duel, card *CardInstance, player int, targets []*CardInstance) error {
			gs := d.State
			if gs.CurrentTarget != nil && d.isOnField(gs.CurrentTarget) {
				d.destroyByEffect(gs.CurrentTarget, "Defender Destruction")
			}
			return nil
		},
	}
	trap := normalTrap("Defender Trap", defenderDestruct)

	// P1 draws Warrior on T5 (7 fillers + warrior at end = warrior drawn on P1's 3rd draw phase)
	deck0 := makePaddedDeck([]*Card{filler, filler, filler, filler, filler, filler, filler, warrior}, 40)
	// P2: Knight, trap, Goblin in initial hand
	deck1 := makePaddedDeck([]*Card{knight, trap, goblin}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// T2: P2 summon Knight, set trap
	p1.AddAction(ActionNormalSummon, "Knight")
	p1.AddAction(ActionSetTech, "Defender Trap")
	// T4: P2 summon Goblin
	p1.AddAction(ActionNormalSummon, "Goblin")
	// T5: P1 draws Warrior, summons it, attacks Knight
	p0.AddAction(ActionNormalSummon, "Warrior")
	p0.AddAction(ActionEnterBattlePhase, "")
	p0.AddAttack("Warrior", "Knight")
	// P2 activates trap in response (destroys Knight, triggering replay)
	p1.AddAction(ActionActivate, "Defender Trap")
	// Replay: P1 re-attacks, choosing Goblin
	p0.AddAttack("Warrior", "Goblin")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 8}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	replayEvents := logger.EventsOfType(log.EventReplay)
	if len(replayEvents) == 0 {
		t.Error("Expected battle replay event")
	}

	battleDestroys := logger.EventsOfType(log.EventBattleDestroy)
	found := false
	for _, e := range battleDestroys {
		if e.Card == "Goblin" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Goblin to be destroyed by battle after replay")
	}
}

// TestPiercingDamage: Aero-Knight Parshath attacks DEF agent, excess damage is dealt (piercing).
func TestPiercingDamage(t *testing.T) {
	airknight := AeroKnightParshath()
	wall := vanillaAgent("Wall", 4, 100, 500, AttrEARTH)
	sacrifice := vanillaAgent("Sacrifice", 4, 1000, 1000, AttrEARTH)

	// P1: sacrifice fodder + Aero-Knight. P2: Wall (set face-down).
	deck0 := makePaddedDeck([]*Card{sacrifice, airknight}, 40)
	deck1 := makePaddedDeck([]*Card{wall}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// T1: P1 summons sacrifice fodder
	p0.AddAction(ActionNormalSummon, "Sacrifice")
	// T2: P2 sets Wall face-down
	p1.AddAction(ActionNormalSet, "Wall")
	// T3: P1 sacrifice summons Aero-Knight (sacrifice the fodder)
	p0.AddAction(ActionSacrificeSummon, "Aero-Knight Parshath")
	p0.AddCardChoice("Sacrifice")
	// T5: P1 attacks face-down Wall — Aero-Knight (1900) vs Wall (500 DEF) = 1400 piercing damage
	p0.AddAction(ActionEnterBattlePhase, "")
	p0.AddAttack("Aero-Knight Parshath", "Wall")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 8}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	lpEvents := logger.EventsOfType(log.EventHPChange)
	piercingDone := false
	for _, e := range lpEvents {
		if strings.Contains(e.Details, "piercing") {
			piercingDone = true
			break
		}
	}
	if !piercingDone {
		t.Error("Expected piercing damage to be dealt")
	}

	// Verify Aero-Knight drew a card after dealing battle damage
	draws := logger.EventsOfType(log.EventDraw)
	battleDraws := 0
	for _, e := range draws {
		if e.Phase == "Battle Phase" {
			battleDraws++
		}
	}
	if battleDraws == 0 {
		t.Error("Expected Aero-Knight Parshath to draw a card after dealing battle damage")
	}
}

// TestNeuralSiphon: Draw 3 cards, then discard 2.
func TestNeuralSiphon(t *testing.T) {
	charity := NeuralSiphon()

	deck0 := makePaddedDeck([]*Card{charity}, 40)
	deck1 := makePaddedDeck([]*Card{}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// T1: Activate Graceful Charity — draw 3, discard 2
	p0.AddAction(ActionActivate, "Neural Siphon")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	activates := logger.EventsOfType(log.EventActivate)
	found := false
	for _, e := range activates {
		if e.Card == "Neural Siphon" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Graceful Charity activation event")
	}

	// Verify: 3 draws from Graceful Charity (during Main Phase, not Draw Phase)
	draws := logger.EventsOfType(log.EventDraw)
	mainPhaseDraws := 0
	for _, e := range draws {
		if e.Player == 0 && e.Phase == "Main Phase 1" {
			mainPhaseDraws++
		}
	}
	if mainPhaseDraws != 3 {
		t.Errorf("Expected 3 draws from Graceful Charity, got %d", mainPhaseDraws)
	}

	// Verify: exactly 2 discards during Main Phase (from Graceful Charity, not hand size)
	discards := logger.EventsOfType(log.EventDiscard)
	charityDiscards := 0
	for _, e := range discards {
		if e.Player == 0 && e.Phase == "Main Phase 1" {
			charityDiscards++
		}
	}
	if charityDiscards != 2 {
		t.Errorf("Expected exactly 2 discards from Graceful Charity, got %d", charityDiscards)
	}
}

// TestTraceAndTerminate: Destroy and purge a face-down agent.
func TestTraceAndTerminate(t *testing.T) {
	nobleman := TraceAndTerminate()
	warrior := vanillaAgent("Warrior", 4, 1500, 1000, AttrEARTH)

	// P1: nobleman in initial hand. P2: warrior in initial hand (set face-down).
	deck0 := makePaddedDeck([]*Card{nobleman}, 40)
	deck1 := makePaddedDeck([]*Card{warrior}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// T2: P2 sets Warrior face-down
	p1.AddAction(ActionNormalSet, "Warrior")
	// T3: P1 activates Nobleman of Crossout targeting face-down Warrior
	p0.AddAction(ActionActivate, "Trace and Terminate")
	p0.AddCardChoice("Warrior")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 6}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	purgeEvents := logger.EventsOfType(log.EventPurge)
	found := false
	for _, e := range purgeEvents {
		if e.Card == "Warrior" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Warrior to be purgeed by Nobleman of Crossout")
	}
}

// TestMobiusTorrentialEffectSerialization: P1 sacrifice summons Mobius, triggering both Mobius's effect
// (destroy up to 2 tech) and opponent's Cascade Failure in effect serialization.
// Chain: CL1=Mobius (TP optional, targets Sakuretsu), CL2=Torrential (NTP optional).
// LIFO: Torrential destroys all agents (Mobius dies), then Mobius effect still resolves
// and destroys the targeted Sakuretsu Armor.
func TestMobiusTorrentialEffectSerialization(t *testing.T) {
	mobius := MobiusTheCryoSovereign()
	fodder := vanillaAgent("Fodder", 4, 1000, 1000, AttrWATER)
	torrential := CascadeFailure()
	sakuretsu := ReactivePlating()

	// P1: fodder in initial hand, Mobius drawn later
	deck0 := makePaddedDeck([]*Card{fodder, mobius}, 40)
	// P2: Torrential + Sakuretsu in initial hand
	deck1 := makePaddedDeck([]*Card{torrential, sakuretsu}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// T1 (P1): Summon sacrifice fodder
	p0.AddAction(ActionNormalSummon, "Fodder")

	// T2 (P2): Set both traps
	p1.AddAction(ActionSetTech, "Cascade Failure")
	p1.AddAction(ActionSetTech, "Reactive Plating")

	// T3 (P1): Sacrifice summon Mobius → effect serialization fires
	p0.AddAction(ActionSacrificeSummon, "Mobius the Cryo Sovereign")
	p0.AddCardChoice("Fodder")           // sacrifice target
	p0.AddYesNo(true)                    // yes, activate Mobius trigger
	p1.AddYesNo(true)                    // yes, activate Torrential
	p0.AddCardChoice("Reactive Plating") // Mobius targets the other trap (not Torrential)

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify: Mobius destroyed by Torrential
	destroys := logger.EventsOfType(log.EventDestroy)
	destroyNames := make(map[string]bool)
	for _, e := range destroys {
		destroyNames[e.Card] = true
	}
	if !destroyNames["Mobius the Cryo Sovereign"] {
		t.Error("Expected Mobius to be destroyed by Cascade Failure")
	}

	// Verify: Sakuretsu Armor destroyed by Mobius effect (resolves even though Mobius is gone)
	if !destroyNames["Reactive Plating"] {
		t.Error("Expected Sakuretsu Armor to be destroyed by Mobius effect (CL1 still resolves after CL2)")
	}
}
