package noteblockplayer

import (
	"reflect"
	"unsafe"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// PacketPlaySound sends a PlaySound packet directly to the player's session connection.
//
// This function takes a player pointer (p), the sound name (name), float32 pitch and volume,
// and a 3D position (pos, mgl64.Vec3). It first converts the position to [3]float32 as required
// by the network packet.
//
// Then, using Go reflection and pointer-unsafe tricks, it accesses the unexported player session
// field "s" and attempts to invoke the "WritePacket" method on it. If not directly available, it
// tries to extract the connection object (field "conn") and invoke "WritePacket" on that.
//
// Ultimately, this method delivers the PlaySound packet to the player, which makes the sound
// play at the specified position with the given pitch and volume from the server side.
// This bypasses higher level APIs and directly calls the underlying session, which is
// useful for custom, low-level sound triggers in plugins or game logic.
func PacketPlaySound(p *player.Player, name string, pitch, volume float32, pos mgl64.Vec3) {
	mgl32Pos := [3]float32{float32(pos[0]), float32(pos[1]), float32(pos[2])}

	val := reflect.ValueOf(p).Elem().FieldByName("s")
	if !val.IsValid() {
		return
	}

	sessionPtr := reflect.NewAt(val.Type(), unsafe.Pointer(val.UnsafeAddr())).Elem()

	method := sessionPtr.MethodByName("WritePacket")
	if method.IsValid() {
		method.Call([]reflect.Value{
			reflect.ValueOf(&packet.PlaySound{
				SoundName: name,
				Volume:    volume, // float32
				Pitch:     pitch,  // float32
				Position:  mgl32Pos,
			}),
		})
		return
	}

	connField := sessionPtr.Elem().FieldByName("conn")
	if connField.IsValid() {
		conn := reflect.NewAt(connField.Type(), unsafe.Pointer(connField.UnsafeAddr())).Elem()
		writeMethod := conn.MethodByName("WritePacket")
		if writeMethod.IsValid() {
			writeMethod.Call([]reflect.Value{
				reflect.ValueOf(&packet.PlaySound{
					SoundName: name,
					Volume:    volume, // float32
					Pitch:     pitch,  // float32
					Position:  mgl32Pos,
				}),
			})
		}
	}
}
