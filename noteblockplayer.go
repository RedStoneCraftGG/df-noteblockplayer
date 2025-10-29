package noteblockplayer

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server/cmd"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/sound"
)

// Note represents a single note in a noteblock song.
// It includes properties like tick (time), layer, instrument, key (pitch), velocity, panning, and pitch bend.
type Note struct {
	Tick       int `json:"tick"`
	Layer      int `json:"layer"`
	Instrument int `json:"instrument"`
	Key        int `json:"key"`
	Velocity   int `json:"velocity,omitempty"`
	Panning    int `json:"panning,omitempty"`
	Pitch      int `json:"pitch,omitempty"`
}

// Song represents the parsed noteblock song file, including meta info and all notes.
type Song struct {
	Tempo    float64 `json:"tempo"`              // Song tempo (ticks per second)
	Length   int     `json:"length"`             // Song length in ticks
	Notes    []Note  `json:"notes"`              // Notes
	Title    string  `json:"title,omitempty"`    // Optional song title
	Author   string  `json:"author,omitempty"`   // Optional song author
	Duration float64 `json:"duration,omitempty"` // Calculated song duration (seconds)
}

// instrumentSounds maps instrument indices to dragonfly sound.Instrument types.
var instrumentSounds = []sound.Instrument{
	sound.Piano(),           // 0
	sound.BassDrum(),        // 1
	sound.Snare(),           // 2
	sound.ClicksAndSticks(), // 3
	sound.Bass(),            // 4
	sound.Flute(),           // 5
	sound.Bell(),            // 6
	sound.Guitar(),          // 7
	sound.Chimes(),          // 8
	sound.Xylophone(),       // 9
	sound.IronXylophone(),   // 10
	sound.CowBell(),         // 11
	sound.Didgeridoo(),      // 12
	sound.Bit(),             // 13
	sound.Banjo(),           // 14
	sound.Pling(),           // 15
}

// stopPlayer holds channels for song-control per player for async song stopping.
// stopPlayerMtx protects access to stopPlayer.
var (
	stopPlayer    = make(map[*world.EntityHandle]chan struct{})
	stopPlayerMtx sync.Mutex
)

// ---------- Command Structs & Registration ----------

// PlayNoteBlockCmd is the command to play a noteblock song (NBS or JSON-based).
type PlayNoteBlockCmd struct {
	Filename string `cmd:"filename"`
}

// AllowConsole allows this command from the server console.
func (PlayNoteBlockCmd) AllowConsole() bool { return true }

// Run executes the playnoteblock command: loads the song, and, if a player, plays it to them only.
func (c PlayNoteBlockCmd) Run(src cmd.Source, output *cmd.Output, w *world.Tx) {
	// If extension is ".nbs" load as NBS, else ".json" or no extension loads as JSON.
	song, err := flexSongLoader(c.Filename)
	if err != nil {
		output.Errorf("Failed to load file: %v", err)
		return
	}
	p, ok := src.(*player.Player)
	if ok {
		output.Printf("Playing %s", c.Filename)
		go playSong(p.H(), song)
		return
	}
	output.Printf("Song %s loaded, but playback is only supported for players", c.Filename)
}

// StopNoteBlockCmd is the command to stop any currently playing noteblock song for the player.
type StopNoteBlockCmd struct{}

// AllowConsole allows this command from the server console.
func (StopNoteBlockCmd) AllowConsole() bool { return true }

// Run executes the stopnoteblock command; only works for players.
func (c StopNoteBlockCmd) Run(src cmd.Source, output *cmd.Output, w *world.Tx) {
	p, ok := src.(*player.Player)
	if !ok {
		output.Print("The stopnoteblock command is only valid for players")
		return
	}
	if stopSong(p.H()) {
		output.Print("Song playback stopped")
	} else {
		output.Print("No song is currently playing")
	}
}

// ----------- Song Data Conversion & Control Utilities -----------

// nbsConverter converts NBSData to Song struct for unified usage.
func nbsConverter(nd *NBSData) *Song {
	notes := make([]Note, len(nd.Notess))
	for i, n := range nd.Notess {
		notes[i] = Note{
			Tick:       n.Tick,
			Layer:      n.Layer,
			Instrument: int(n.Instrument),
			Key:        int(n.Key),
			Velocity:   int(n.Velocity),
			Panning:    int(n.Panning),
			Pitch:      int(n.Pitch),
		}
	}
	return &Song{
		Tempo:    float64(nd.Tempo),
		Length:   int(nd.Length),
		Notes:    notes,
		Duration: float64(nd.Duration),
	}
}

// stopSong signals the running goroutine (if exists) to stop playing the song for a given player.
// Returns true if a song was stopped, false if not.
func stopSong(eh *world.EntityHandle) bool {
	stopPlayerMtx.Lock()
	defer stopPlayerMtx.Unlock()
	ch, ok := stopPlayer[eh]
	if ok {
		select {
		case ch <- struct{}{}:
		default:
		}
		delete(stopPlayer, eh)
		return true
	}
	return false
}

// ------------ Song Playback Utilities ------------

// playSong plays the given Song asynchronously for the provided EntityHandle (player).
// Allows controlled stopping, handles tick timing, and message.
func playSong(eh *world.EntityHandle, song *Song) {
	stopPlayerMtx.Lock()
	if ch, ok := stopPlayer[eh]; ok {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	stopChan := make(chan struct{}, 1)
	stopPlayer[eh] = stopChan
	stopPlayerMtx.Unlock()

	tickDuration := time.Second / 20 // Default: 20 ticks per second
	if song.Tempo > 0 {
		tickDuration = time.Duration(float64(time.Second) / song.Tempo)
	}

	currentTick := 0
	notesPerTick := make(map[int][]Note)
	for _, note := range song.Notes {
		notesPerTick[note.Tick] = append(notesPerTick[note.Tick], note)
	}

	defer func() {
		_ = eh.ExecWorld(func(_ *world.Tx, ent world.Entity) {
			pp, ok := ent.(*player.Player)
			if ok {
				pp.Message("Song playback finished.")
			}
		})
		stopPlayerMtx.Lock()
		delete(stopPlayer, eh)
		stopPlayerMtx.Unlock()
	}()

	for tick := 0; tick <= song.Length; tick++ {
		select {
		case <-stopChan:
			return
		default:
		}

		if tick > currentTick {
			time.Sleep(time.Duration(tick-currentTick) * tickDuration)
			currentTick = tick
		}
		if notes, found := notesPerTick[tick]; found {
			for _, note := range notes {
				inst := sound.Piano()
				if note.Instrument >= 0 && note.Instrument < len(instrumentSounds) {
					inst = instrumentSounds[note.Instrument]
				}
				pitch := pitchKey(note.Key)
				// For further enhancement: use velocity, custom pitch, and panning as needed.
				fmt.Printf(
					"Tick=%d Layer=%d Instr=%d Key=%d Pitch=%d Vel=%d Pan=%d\n",
					note.Tick, note.Layer, note.Instrument, note.Key, pitch, note.Velocity, note.Panning,
				)
				playSoundSelf(eh, sound.Note{
					Instrument: inst,
					Pitch:      pitch,
				})
			}
		}
	}
}

// playSoundSelf plays a sound only for the provided player entity (self).
func playSoundSelf(eh *world.EntityHandle, snd world.Sound) {
	_ = eh.ExecWorld(func(tx *world.Tx, ent world.Entity) {
		pp, ok := ent.(*player.Player)
		if !ok {
			return
		}
		pos := pp.Position()
		tx.PlaySound(pos, snd)
	})
}

// pitchKey calculates the Bedrock note pitch index based on the NBS note key.
// Bedrock's base is 33 (F#3).
func pitchKey(key int) int {
	base := 33 // F#3 is key 33 in Bedrock
	return key - base
}

// Pow is a helper to call math.Pow.
func Pow(base, exp float64) float64 {
	return math.Pow(base, exp)
}

// --------------- Function Call Helper ----------------------

// PlayNoteblock is a helper function to programmatically play a song file for a player.
//
// Accepts player handle (EntityHandle) and file name (string, path relative to "noteblock" folder or base folder).
// Supported formats: ".nbs" (Noteblock Studio), ".json" (custom Song struct).
//
// Returns error if loading or playback fails.
// Example usage (from any Go function with *player.Player object `p`):
//
//	err := PlayNoteblock(p.H(), "my_song.nbs")
//	if err != nil {
//	    // handle error
//	}
//
// Note: This helper does not send a chat message to the player! (Unlike the command.)
func PlayNoteblock(eh *world.EntityHandle, filename string) error {
	song, err := flexSongLoader(filename)
	if err != nil {
		return err
	}
	go playSong(eh, song)
	return nil
}

// --------------- Command Registration ---------------

// init registers all noteblock-related player commands.
func init() {
	cmd.Register(cmd.New(
		"playnoteblock",
		"Play a noteblock song file (json/nbs)",
		[]string{"playnb", "pnb"},
		PlayNoteBlockCmd{},
	))
	cmd.Register(cmd.New(
		"stopnoteblock",
		"Stop the currently playing noteblock file",
		[]string{"stopnb", "snb"},
		StopNoteBlockCmd{},
	))
}
