package game

import (
	"context"
	"testing"

	"github.com/peterkuimelis/tcgx/internal/log"
)

// TestBasicSummonAndAttack: P1 summons a agent, P2 summons a agent, they fight.
func TestBasicSummonAndAttack(t *testing.T) {
	geminiElf := vanillaAgent("Gemini Elf", 4, 1900, 900, AttrEARTH)
	laJinn := vanillaAgent("La Jinn", 4, 1800, 1000, AttrDARK)

	// P1 draws Gemini Elf first, P2 draws La Jinn first
	deck0 := makePaddedDeck([]*Card{geminiElf}, 40)
	deck1 := makePaddedDeck([]*Card{laJinn}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Turn 1 (P1): Summon Gemini Elf, end turn (can't attack turn 1)
	p0.AddAction(ActionNormalSummon, "Gemini Elf")

	// Turn 2 (P2): Summon La Jinn, attack Gemini Elf
	p1.AddAction(ActionNormalSummon, "La Jinn")
	p1.AddAction(ActionEnterBattlePhase, "")
	p1.AddAttack("La Jinn", "Gemini Elf")
	// La Jinn (1800) vs Gemini Elf (1900) → La Jinn destroyed, P2 takes 100 damage

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 3}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify: La Jinn should have been destroyed by battle
	battleDestroys := logger.EventsOfType(log.EventBattleDestroy)
	if len(battleDestroys) == 0 {
		t.Fatal("Expected a battle destruction event")
	}
	if battleDestroys[0].Card != "La Jinn" {
		t.Errorf("Expected La Jinn to be destroyed, got %s", battleDestroys[0].Card)
	}

	// Verify: P2 should have taken 100 damage
	lpChanges := logger.EventsOfType(log.EventHPChange)
	found := false
	for _, e := range lpChanges {
		if e.Player == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected P2 to take HP damage")
	}
}

// TestDirectAttackWin: P1 summons a strong agent, attacks directly over two turns to win.
func TestDirectAttackWin(t *testing.T) {
	blueEyes := vanillaAgent("Blue-Eyes White Dragon", 8, 3000, 2500, AttrLIGHT)
	fodder1 := vanillaAgent("Fodder A", 1, 100, 100, AttrLIGHT)
	fodder2 := vanillaAgent("Fodder B", 1, 100, 100, AttrLIGHT)

	// P1 needs 2 sacrifice fodder + Blue-Eyes in hand after initial draw
	deck0 := makePaddedDeck([]*Card{fodder1, fodder2, blueEyes}, 40)
	deck1 := makePaddedDeck([]*Card{}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Turn 1 (P1): Summon Fodder A (can't attack)
	p0.AddAction(ActionNormalSummon, "Fodder A")

	// Turn 2 (P2): End turn
	// (default behavior)

	// Turn 3 (P1): Summon Fodder B
	p0.AddAction(ActionNormalSummon, "Fodder B")

	// Turn 4 (P2): End turn

	// Turn 5 (P1): Sacrifice summon Blue-Eyes (sacrifice Fodder A + Fodder B), attack directly
	p0.AddAction(ActionSacrificeSummon, "Blue-Eyes White Dragon")
	p0.AddCardChoice("Fodder A", "Fodder B")
	p0.AddAction(ActionEnterBattlePhase, "")
	p0.AddDirectAttack("Blue-Eyes White Dragon")

	// Turn 6 (P2): End turn

	// Turn 7 (P1): Attack directly again (P2: 8192 → 5192 → 2192)
	p0.AddAction(ActionEnterBattlePhase, "")
	p0.AddDirectAttack("Blue-Eyes White Dragon")

	// Turn 8 (P2): End turn

	// Turn 9 (P1): Attack directly → 2000 - 3000 → 0 HP → P1 wins
	p0.AddAction(ActionEnterBattlePhase, "")
	p0.AddDirectAttack("Blue-Eyes White Dragon")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify P1 won
	wins := logger.EventsOfType(log.EventWin)
	if len(wins) == 0 {
		t.Fatal("Expected a win event")
	}
	if wins[0].Player != 0 {
		t.Errorf("Expected P1 to win, got P%d", wins[0].Player+1)
	}
}

// TestATKvsDEF: Attack into a DEF position agent.
func TestATKvsDEF(t *testing.T) {
	attacker := vanillaAgent("Luster Dragon", 4, 1900, 1600, AttrWIND)
	defender := vanillaAgent("Giant Soldier of Stone", 3, 1300, 2000, AttrEARTH)

	deck0 := makePaddedDeck([]*Card{attacker}, 40)
	deck1 := makePaddedDeck([]*Card{defender}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Turn 1 (P1): Summon Luster Dragon
	p0.AddAction(ActionNormalSummon, "Luster Dragon")

	// Turn 2 (P2): Set Giant Soldier of Stone
	p1.AddAction(ActionNormalSet, "Giant Soldier of Stone")

	// Turn 3 (P1): Attack the face-down agent
	// Luster Dragon (1900 ATK) vs Giant Soldier (2000 DEF) → P1 takes 100 damage, nothing destroyed
	p0.AddAction(ActionEnterBattlePhase, "")
	p0.AddAttack("Luster Dragon", "Giant Soldier of Stone")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify: no agents destroyed
	battleDestroys := logger.EventsOfType(log.EventBattleDestroy)
	if len(battleDestroys) != 0 {
		t.Errorf("Expected no battle destruction, got %d", len(battleDestroys))
	}

	// Verify: P1 took 100 damage (1900 - 2000 = -100)
	lpChanges := logger.EventsOfType(log.EventHPChange)
	found := false
	for _, e := range lpChanges {
		if e.Player == 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected P1 to take recoil damage from attacking into higher DEF")
	}

	// Verify: Giant Soldier was flipped face-up
	flips := logger.EventsOfType(log.EventFlipNoSummon)
	if len(flips) == 0 {
		t.Error("Expected face-down agent to be flipped face-up when attacked")
	}
}

// TestFlipSummon: Set a agent, then flip summon it next turn.
func TestFlipSummon(t *testing.T) {
	agent := vanillaAgent("Mystical Elf", 4, 800, 2000, AttrLIGHT)

	deck0 := makePaddedDeck([]*Card{agent}, 40)
	deck1 := makePaddedDeck([]*Card{}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Turn 1 (P1): Set Mystical Elf
	p0.AddAction(ActionNormalSet, "Mystical Elf")

	// Turn 2 (P2): End turn

	// Turn 3 (P1): Flip summon Mystical Elf
	p0.AddAction(ActionFlipSummon, "Mystical Elf")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify flip summon event
	flips := logger.EventsOfType(log.EventFlipSummon)
	if len(flips) == 0 {
		t.Fatal("Expected a flip summon event")
	}
	if flips[0].Card != "Mystical Elf" {
		t.Errorf("Expected Mystical Elf to be flip summoned, got %s", flips[0].Card)
	}
}

// TestChangePosition: Summon in ATK, change to DEF next turn.
func TestChangePosition(t *testing.T) {
	agent := vanillaAgent("Gemini Elf", 4, 1900, 900, AttrEARTH)

	deck0 := makePaddedDeck([]*Card{agent}, 40)
	deck1 := makePaddedDeck([]*Card{}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Turn 1 (P1): Summon Gemini Elf (can't change position same turn)
	p0.AddAction(ActionNormalSummon, "Gemini Elf")

	// Turn 2 (P2): End turn

	// Turn 3 (P1): Change Gemini Elf to DEF
	p0.AddAction(ActionChangePosition, "Gemini Elf")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify position change event
	changes := logger.EventsOfType(log.EventChangePosition)
	if len(changes) == 0 {
		t.Fatal("Expected a change position event")
	}
}

// TestDeckOut: Player with tiny deck loses by drawing.
func TestDeckOut(t *testing.T) {
	// P2 has only 6 cards (5 for initial hand + 1 draw on turn 2)
	filler := vanillaAgent("Filler", 1, 0, 0, AttrLIGHT)
	var smallDeck []*Card
	for i := 0; i < 6; i++ {
		smallDeck = append(smallDeck, filler)
	}

	deck0 := makePaddedDeck([]*Card{}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	cfg := DuelConfig{Deck0: deck0, Deck1: smallDeck}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// P2 draws 5 initial + 1 on turn 2 → empty. On turn 4, P2 can't draw → loses.
	wins := logger.EventsOfType(log.EventWin)
	if len(wins) == 0 {
		t.Fatal("Expected a win event from deck out")
	}
	// P1 should win
	if wins[0].Player != 0 {
		t.Errorf("Expected P1 to win by deck out, got P%d", wins[0].Player+1)
	}
}

// TestSacrificeSummon: Sacrifice one agent to summon a Level 5-6 agent.
func TestSacrificeSummon(t *testing.T) {
	fodder := vanillaAgent("Fodder", 1, 100, 100, AttrLIGHT)
	jinzo := vanillaAgent("Jinzo", 6, 2400, 1500, AttrDARK)

	deck0 := makePaddedDeck([]*Card{fodder, jinzo}, 40)
	deck1 := makePaddedDeck([]*Card{}, 40)

	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Turn 1: Summon Fodder
	p0.AddAction(ActionNormalSummon, "Fodder")

	// Turn 2: P2 end

	// Turn 3: Sacrifice Summon Jinzo (sacrifice Fodder)
	p0.AddAction(ActionSacrificeSummon, "Jinzo")
	p0.AddCardChoice("Fodder")

	cfg := DuelConfig{Deck0: deck0, Deck1: deck1, MaxTurns: 4}
	logger := runDuelToCompletion(t, cfg, p0, p1)

	// Verify sacrifice summon
	tribSummons := logger.EventsOfType(log.EventSacrificeSummon)
	if len(tribSummons) == 0 {
		t.Fatal("Expected a sacrifice summon event")
	}
	if tribSummons[0].Card != "Jinzo" {
		t.Errorf("Expected Jinzo to be sacrifice summoned, got %s", tribSummons[0].Card)
	}

	// Verify fodder was sent to Scrapheap
	scrapheapEvents := logger.EventsOfType(log.EventSendToScrapheap)
	found := false
	for _, e := range scrapheapEvents {
		if e.Card == "Fodder" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Fodder to be sent to Scrapheap as sacrifice")
	}
}

// TestCannotAttackTurn1: First player cannot enter battle phase on turn 1.
func TestCannotAttackTurn1(t *testing.T) {
	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	// Set up state manually to check actions
	gs := NewGameState()
	gs.Turn = 1
	gs.TurnPlayer = 0
	gs.Phase = PhaseMain1
	gs.Players[0].HP = StartingHP
	gs.Players[1].HP = StartingHP

	memLog := log.NewMemoryLogger()
	testDuel := &Duel{
		State:       gs,
		Controllers: [2]PlayerController{p0, p1},
		Logger:      memLog,
		ctx:         context.Background(),
	}

	actions := testDuel.computeMainPhaseActions(0)
	for _, a := range actions {
		if a.Type == ActionEnterBattlePhase {
			t.Error("First player should not be able to enter Battle Phase on turn 1")
		}
	}
}

// TestSecondPlayerCanAttackTurn2: Second player (P2) CAN attack on their first turn (turn 2).
func TestSecondPlayerCanAttackTurn2(t *testing.T) {
	gs := NewGameState()
	gs.Turn = 2
	gs.TurnPlayer = 1
	gs.Phase = PhaseMain1
	gs.Players[0].HP = StartingHP
	gs.Players[1].HP = StartingHP

	memLog := log.NewMemoryLogger()
	p0 := NewScriptedController(t, "P1")
	p1 := NewScriptedController(t, "P2")

	testDuel := &Duel{
		State:       gs,
		Controllers: [2]PlayerController{p0, p1},
		Logger:      memLog,
		ctx:         context.Background(),
	}

	actions := testDuel.computeMainPhaseActions(1)
	found := false
	for _, a := range actions {
		if a.Type == ActionEnterBattlePhase {
			found = true
			break
		}
	}
	if !found {
		t.Error("Second player should be able to enter Battle Phase on turn 2")
	}
}
