package game

import (
	"os"
	"testing"

	"github.com/peterkuimelis/tcgx/internal/log"
)

// TestTranscriptFuryVsBlaze replays a real game fragment between
// Fury from the Deep (P1, WATER) and Blaze of Destruction (P2, FIRE).
// Covers 4 turns with: program activations, Quick-Play from hand (MST),
// battle with destruction, special summon (ThermalSpike), continuous ATK modifiers
// (Micro Chimera), and direct attacks.
//
// No assertions — this test writes the full event log for human analysis.
func TestTranscriptFuryVsBlaze(t *testing.T) {
	// ===== P1 Deck: Fury from the Deep =====
	// Draw order (index 0 drawn first):
	//   0-4: initial hand
	//   5:   T1 draw
	//   6:   T3 draw
	//   7-8: Pot of Greed draws during T3
	p1Deck := makePaddedDeck([]*Card{
		AbyssalNetrunner(),  // initial hand
		VoidDrifter(),       // initial hand
		DeadlockSeal(),      // initial hand
		IdentityHijack(),    // initial hand
		HeadshotRoutine(),   // initial hand
		LeviaMechDaedalus(), // T1 draw
		GreedProtocol(),     // T3 draw
		CoreDump(),          // PoG draw 1
		SignalAmplifier(),   // PoG draw 2
	}, 40)

	// ===== P2 Deck: Blaze of Destruction =====
	// Draw order:
	//   0-4: initial hand
	//   5:   T2 draw
	//   6-7: Pot of Greed draws during T2
	//   8:   T4 draw
	p2Deck := makePaddedDeck([]*Card{
		GreedProtocol(),            // initial hand
		SectorLockdownZoneB(),      // initial hand
		ThermalSpike(),             // initial hand
		GaiaCoreTheVolatileSwarm(), // initial hand
		ChromeAngus(),              // initial hand
		OrbitalPayload(),           // T2 draw
		ICEBreaker(),               // PoG draw 1
		MicroChimera(),             // PoG draw 2
		DroneCarrier(),             // T4 draw
	}, 40)

	// ===== Script P1 (Fury from the Deep) =====
	p0 := NewScriptedController(t, "P1")

	// Turn 1: Set trap, summon beater
	p0.AddAction(ActionSetTech, "Deadlock Seal")
	p0.AddAction(ActionNormalSummon, "Void Drifter")

	// Turn 3: Pot of Greed, Hammer Shot (destroy Great Angus), summon + direct attack
	p0.AddAction(ActionActivate, "Greed Protocol")
	p0.AddAction(ActionActivate, "Headshot Routine")
	p0.AddAction(ActionNormalSummon, "Abyssal Netrunner")
	p0.AddAction(ActionEnterBattlePhase, "")
	p0.AddDirectAttack("Abyssal Netrunner")

	// ===== Script P2 (Blaze of Destruction) =====
	p1 := NewScriptedController(t, "P2")

	// Turn 2: Burn program, draw program, MST the set trap, summon + attack
	p1.AddAction(ActionActivate, "Orbital Payload")
	p1.AddAction(ActionActivate, "Greed Protocol")
	p1.AddAction(ActionActivate, "ICE Breaker")
	p1.AddCardChoice("Deadlock Seal") // MST target
	p1.AddAction(ActionNormalSummon, "Chrome Angus")
	p1.AddAction(ActionEnterBattlePhase, "")
	p1.AddAttack("Chrome Angus", "Void Drifter")

	// Turn 4: Special summon ThermalSpike (purge from Scrapheap), summon Micro Chimera, attack with modifier
	p1.AddAction(ActionActivate, "Thermal Spike") // special summon via ActionActivate
	p1.AddCardChoice("Chrome Angus")              // purge FIRE from Scrapheap as ThermalSpike cost
	p1.AddAction(ActionNormalSummon, "Micro Chimera")
	p1.AddAction(ActionEnterBattlePhase, "")
	p1.AddAttack("Thermal Spike", "Abyssal Netrunner")

	// ===== Run =====
	cfg := DuelConfig{
		Deck0:    p1Deck,
		Deck1:    p2Deck,
		MaxTurns: 5,
	}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Write event log to project root for analysis
	eventLog := log.FormatAll(logger.Events())
	err := os.WriteFile("../../transcript_events.log", []byte(eventLog), 0644)
	if err != nil {
		t.Fatalf("Failed to write event log: %v", err)
	}
	t.Logf("Event log written to transcript_events.log (%d events)", len(logger.Events()))
}

// TestGravityClampStopsAttack verifies that when Gravity Clamp is activated
// during the response window after a Level 4+ agent declares a direct attack,
// the attack is stopped and no damage is dealt.
//
// T1 (P1): Set Gravity Clamp face-down
// T2 (P2): Summon Chrome Angus (Level 4, ATK 1800), declare direct attack
//
//	→ P1 activates Gravity Clamp in response → attack stopped, 0 damage
func TestGravityClampStopsAttack(t *testing.T) {
	// P1: Gravity Clamp in hand, rest filler
	p1Deck := makePaddedDeck([]*Card{
		GravityClamp(), // initial hand [0]
	}, 40)

	// P2: Chrome Angus in hand, rest filler
	p2Deck := makePaddedDeck([]*Card{
		ChromeAngus(), // initial hand [0]
	}, 40)

	// === P1 script ===
	p0 := NewScriptedController(t, "P1")
	// T1: Set Gravity Clamp face-down
	p0.AddAction(ActionSetTech, "Gravity Clamp")
	// T2 response window: activate Gravity Clamp when Chrome Angus declares attack
	p0.AddAction(ActionActivate, "Gravity Clamp")

	// === P2 script ===
	p1 := NewScriptedController(t, "P2")
	// T2: Summon Chrome Angus, enter battle, direct attack
	p1.AddAction(ActionNormalSummon, "Chrome Angus")
	p1.AddAction(ActionEnterBattlePhase, "")
	p1.AddDirectAttack("Chrome Angus")

	cfg := DuelConfig{
		Deck0:    p1Deck,
		Deck1:    p2Deck,
		MaxTurns: 3,
	}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Write event log
	eventLog := log.FormatAll(logger.Events())
	err := os.WriteFile("../../transcript_gravity_clamp.log", []byte(eventLog), 0644)
	if err != nil {
		t.Fatalf("Failed to write event log: %v", err)
	}
	t.Logf("Event log written to transcript_gravity_clamp.log (%d events)", len(logger.Events()))

	// Assert: AttackStopped event must exist
	found := false
	for _, ev := range logger.Events() {
		if ev.Type == log.EventAttackStopped && ev.Card == "Chrome Angus" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected EventAttackStopped for Chrome Angus, but not found in log")
	}

	// Assert: P1 HP unchanged (no damage dealt)
	for _, ev := range logger.Events() {
		if ev.Type == log.EventHPChange && ev.Player == 0 {
			t.Errorf("P1 HP should not change, but got: %s", ev.Details)
		}
	}
}
