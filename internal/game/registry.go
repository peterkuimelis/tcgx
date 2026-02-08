package game

import "fmt"

// CardRegistry maps card names to their constructor functions.
var CardRegistry = map[string]func() *Card{
	"Greed Protocol":                    GreedProtocol,
	"Void Purge":                        VoidPurge,
	"EMP Cascade":                       EMPCascade,
	"ICE Breaker":                       ICEBreaker,
	"Blackout Patch":                    BlackoutPatch,
	"Reactive Plating":                  ReactivePlating,
	"Reflector Array":                   ReflectorArray,
	"Cascade Failure":                   CascadeFailure,
	"Self-Destruct Circuit":             SelfDestructCircuit,
	"Root Override":                     RootOverride,
	"Breaker the Chrome Warrior":        BreakerTheChromeWarrior,
	"Polymorphic Virus":                 PolymorphicVirus,
	"Recursive Worm":                    RecursiveWorm,
	"Datamancer":                        Datamancer,
	"Morph Canister":                    MorphCanister,
	"Aero-Knight Parshath":              AeroKnightParshath,
	"Chrome Paladin - Envoy of Genesis": ChromePaladinEnvoy,
	"Hostile Takeover":                  HostileTakeover,
	"Emergency Reboot":                  EmergencyReboot,
	"Neural Siphon":                     NeuralSiphon,
	"Memory Corruption":                 MemoryCorruption,
	"Trace and Terminate":               TraceAndTerminate,
	"Resurrection Protocol":             ResurrectionProtocol,
	"Static Discharge":                  StaticDischarge,
	"Decoy Holograms":                   DecoyHolograms,
	"Prismatic Datafish":                PrismaticDatafish,
	"Blazing Automaton":                 BlazingAutomaton,
	"Chrome Angus":                      ChromeAngus,
	"Abyssal Netrunner":                 AbyssalNetrunner,
	"Void Drifter":                      VoidDrifter,
	"Headshot Routine":                  HeadshotRoutine,
	"Orbital Payload":                   OrbitalPayload,
	"Flatline Command":                  FlatlineCommand,
	"Scrapheap Recovery":                ScrapheapRecovery,
	"Core Dump":                         CoreDump,
	"Surge Override":                    SurgeOverride,
	"Identity Hijack":                   IdentityHijack,
	"Cache Siphon":                      CacheSiphon,
	"Reactor Meltdown":                  ReactorMeltdown,
	"The Undercity Grid":                TheUndercityGrid,
	"Torture Subnet":                    TortureSubnet,
	"Sector Lockdown - Zone B":          SectorLockdownZoneB,
	"Neural Shackle":                    NeuralShackle,
	"Firewall Type-8":                   FirewallType8,
	"Counter-Hack":                      CounterHack,
	"Gravity Clamp":                     GravityClamp,
	"Surge Barrier":                     SurgeBarrier,
	"Deadlock Seal":                     DeadlockSeal,
	"Signal Amplifier":                  SignalAmplifier,
	"Micro Chimera":                     MicroChimera,
	"Den Mother Unit":                   DenMotherUnit,
	"Drone Carrier":                     DroneCarrier,
	"Mobius the Cryo Sovereign":         MobiusTheCryoSovereign,
	"Thestalos the Plasma Sovereign":    ThestalosThePlasmaSovereign,
	"Thermal Spike":                     ThermalSpike,
	"Fenrir Mk.II":                      FenrirMkII,
	"Amphibious Mech MK-3":              AmphibiousMechMK3,
	"Siren Enforcer":                    SirenEnforcer,
	"Levia-Mech - Daedalus":             LeviaMechDaedalus,
	"Neon Hydra Lord - Neo-Daedalus":    NeonHydraLordNeoDaedalus,
	"Stealth Glider":                    StealthGlider,
	"Raging Plasma Sprite":              RagingPlasmaSprite,
	"Solar Flare Serpent":               SolarFlareSerpent,
	"Ghost Process":                     GhostProcess,
	"Gaia Core the Volatile Swarm":      GaiaCoreTheVolatileSwarm,
	"Molten Cyborg":                     MoltenCyborg,
	"Ultimate Street Punk":              UltimateStreetPunk,
	"Junkyard Lurker":                   JunkyardLurker,
	"Infernal Plasma Emperor":           InfernalPlasmaEmperor,
}

// LookupCard looks up a card by name and returns a new instance.
// Panics if the card is not found.
func LookupCard(name string) *Card {
	ctor, ok := CardRegistry[name]
	if !ok {
		panic(fmt.Sprintf("card not found in registry: %q", name))
	}
	return ctor()
}
