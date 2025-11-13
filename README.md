# df-noteblockplayer

A simple Dragonfly-MC plugin that lets players load and play Note Block Studio (.nbs) songs in-game.

# What's New?

- The limitation for note keys below F#3 has been resolved. You can now play low-pitched notes without any problem.
- You can now control the note volume using the velocity property (see JSON examples).
- The `PlaySound` method has been updated to use direct packet session writing, allowing packets to be sent directly to the player.

## Installation

1. Import the package, and make sure there is a `noteblock` folder in your project directory:

```go
package main

import (
  _ "github.com/redstonecraftgg/df-noteblockplayer"
  // other imports
)
```

2. Put your `.nbs` files or JSON files (you can create these with [NoteblockParser](https://github.com/RedStoneCraftGG/NoteblockParser)) inside the `noteblock` folder.

## Usage

You can play songs in two ways:

### Using Commands

- To play a song, use `/playnoteblock <your file name>`. You can also use `/playnb` or `/pnb` as shortcuts.
- To stop the song, use `/stopnoteblock`. Shortcuts are `/stopnb` and `/snb`.

### Using Functions

You can also play a song from your code with the `PlayNoteblock()` function:

```go
err := PlayNoteblock(p.H(), "my_song.nbs")
if err != nil {
    // handle error
}
```

To stop a song, you can use the `StopNoteblock()` function. You can also use the lower-level `stopSong(eh *world.EntityHandle)` function if needed.

```go
success := StopNoteblock(p.H())
if success {
    // The song was successfully stopped
} else {
    // No song was playing
}
```

## Known Issues and Limitations

- Playing custom noteblock instruments from resource packs is not yet supported (this feature may be added in a future version).
