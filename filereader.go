package noteblockplayer

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Notes represents a single note event read from the NBS file.
type Notes struct {
	Tick       int   `json:"tick"`
	Layer      int   `json:"layer"`
	Instrument uint8 `json:"instrument"`
	Key        uint8 `json:"key"`
	Velocity   uint8 `json:"velocity"`
	Panning    uint8 `json:"panning"`
	Pitch      int16 `json:"pitch"`
}

// NBSData holds global information as well as all Notes parsed from a NBS file.
type NBSData struct {
	Version  uint8   `json:"version"`
	Length   uint16  `json:"length"`
	Layers   uint16  `json:"layers"`
	Tempo    float32 `json:"tempo"`
	Duration float32 `json:"duration"`
	Notess   []Notes `json:"Notess"`
}

// ==================== File Utility Functions ====================

// fileExists checks if the path exists and is a regular file.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ==================== Binary Reader Helper Functions ====================

// readUint8 reads a uint8 from io.Reader (little endian).
func readUint8(r io.Reader) (uint8, error) {
	var b [1]byte
	_, err := io.ReadFull(r, b[:])
	return b[0], err
}

// readUint16 reads a uint16 from io.Reader (little endian).
func readUint16(r io.Reader) (uint16, error) {
	var b [2]byte
	_, err := io.ReadFull(r, b[:])
	return binary.LittleEndian.Uint16(b[:]), err
}

// readUint32 reads a uint32 from io.Reader (little endian).
func readUint32(r io.Reader) (uint32, error) {
	var b [4]byte
	_, err := io.ReadFull(r, b[:])
	return binary.LittleEndian.Uint32(b[:]), err
}

// readInt16 reads an int16 from io.Reader (little endian).
func readInt16(r io.Reader) (int16, error) {
	var b [2]byte
	_, err := io.ReadFull(r, b[:])
	return int16(binary.LittleEndian.Uint16(b[:])), err
}

// readString reads a string prefixed with uint32 length and cleans it.
func readString(r io.Reader) (string, error) {
	length, err := readUint32(r)
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}
	strBytes := make([]byte, length)
	_, err = io.ReadFull(r, strBytes)
	if err != nil {
		return "", err
	}
	return cleanString(string(strBytes)), nil
}

// cleanString trims whitespace and removes NUL characters.
func cleanString(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(strings.ReplaceAll(s, "\x00", "")), "\x00", "")
}

// ==================== NBS File Parsing & Loading Functions ====================

// ParseNBS parses an NBS file (*.nbs) and returns an NBSData structure
// containing the parsed notes and metadata.
func ParseNBS(filename string) (*NBSData, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data NBSData

	// Parse header and meta fields
	data.Length, err = readUint16(file)
	if err != nil {
		return nil, err
	}

	data.Version, err = readUint8(file)
	if err != nil {
		return nil, err
	}

	// Skip vanilla instrument count
	if _, err := readUint8(file); err != nil {
		return nil, err
	}

	data.Layers, err = readUint16(file)
	if err != nil {
		return nil, err
	}

	// Skip custom instrument count
	if _, err := readUint16(file); err != nil {
		return nil, err
	}

	// Skip title, author, original_author, description
	for i := 0; i < 4; i++ {
		if _, err := readString(file); err != nil {
			return nil, err
		}
	}

	// Tempo (as centi-tempo)
	tempoRaw, err := readUint16(file)
	if err != nil {
		return nil, err
	}
	data.Tempo = float32(tempoRaw) / 100.0

	// Skip: auto_save, auto_save_duration, time_signature
	for i := 0; i < 3; i++ {
		if _, err := readUint8(file); err != nil {
			return nil, err
		}
	}
	// Skip: minutes_spent, left_clicks, right_clicks, blocks_added, blocks_removed
	for i := 0; i < 5; i++ {
		if _, err := readUint32(file); err != nil {
			return nil, err
		}
	}
	// Skip import_name
	if _, err := readString(file); err != nil {
		return nil, err
	}

	// Skip loop, max_loop_count, loop_start_tick
	for i := 0; i < 2; i++ {
		if _, err := readUint8(file); err != nil {
			return nil, err
		}
	}
	if _, err := readUint16(file); err != nil {
		return nil, err
	}

	// Begin parsing note blocks
	tick := -1
	var allNotess []Notes
	for {
		jumpTicks, err := readUint16(file)
		if err != nil {
			return nil, err
		}
		if jumpTicks == 0 {
			break
		}
		tick += int(jumpTicks)

		layer := -1
		for {
			jumpLayers, err := readUint16(file)
			if err != nil {
				return nil, err
			}
			if jumpLayers == 0 {
				break
			}
			layer += int(jumpLayers)

			instrument, err := readUint8(file)
			if err != nil {
				return nil, err
			}
			key, err := readUint8(file)
			if err != nil {
				return nil, err
			}
			velocity := uint8(100)
			panning := uint8(100)
			pitch := int16(0)
			// Version >= 4 files have additional velocity, panning, pitch fields
			if data.Version >= 4 {
				if velocity, err = readUint8(file); err != nil {
					return nil, err
				}
				if panning, err = readUint8(file); err != nil {
					return nil, err
				}
				if pitch, err = readInt16(file); err != nil {
					return nil, err
				}
			}
			// Ignore placeholder note (key==0)
			if key == 0 {
				continue
			}
			n := Notes{
				Tick:       tick,
				Layer:      layer,
				Instrument: instrument,
				Key:        key,
				Velocity:   velocity,
				Panning:    panning,
				Pitch:      pitch,
			}
			allNotess = append(allNotess, n)
		}
	}

	// In some rare NBS files, length field is zero but notes exist.
	if data.Length == 0 && len(allNotess) > 0 {
		maxTick := allNotess[0].Tick
		for _, n := range allNotess {
			if n.Tick > maxTick {
				maxTick = n.Tick
			}
		}
		data.Length = uint16(maxTick)
	}

	// Calculate song duration (in seconds)
	if data.Tempo > 0.0 {
		data.Duration = float32(data.Length) / data.Tempo
	}
	data.Notess = allNotess
	return &data, nil
}

// ReadNBS reads and parses an NBS file from disk and returns NBSData.
func ReadNBS(path string) (*NBSData, error) {
	return ParseNBS(path)
}

// loadJSON loads a Song struct from a .json file on disk.
func loadJSON(path string) (*Song, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var song Song
	if err := json.Unmarshal(data, &song); err != nil {
		return nil, err
	}
	return &song, nil
}

// flexSongLoader tries to load a song from ./noteblock/ by name, choosing between NBS or JSON format automatically.
// NBS files are parsed with ReadNBS, JSON files are decoded into Song.
func flexSongLoader(name string) (*Song, error) {
	name = strings.TrimSuffix(name, ".json")
	name = strings.TrimSuffix(name, ".nbs")
	jsonPath := filepath.Join("noteblock", name+".json")
	nbsPath := filepath.Join("noteblock", name+".nbs")

	if fileExists(nbsPath) {
		data, err := ReadNBS(nbsPath)
		if err != nil {
			return nil, err
		}
		return nbsConverter(data), nil
	} else if fileExists(jsonPath) {
		return loadJSON(jsonPath)
	}
	return nil, fmt.Errorf("file not found")
}
