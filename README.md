# df-noteblockplayer

A simple Dragonfly-MC plugin that lets players load and play Note Block Studio (.nbs) songs in-game.

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

Currently, there is no `StopNoteblock()` function. I wasn't able to test it due to a Minecraft DNS issue (from 2025-10-29 to 2025-10-30) that prevented me from joining a server. I will add this function once the issue is resolved.

## Known Issues and Limitations

- Dragonfly-MC's API does not have volume control yet, so you will hear sounds at their default volume from the resource pack.
- Pitch is limited: because dragonfly-mc uses an int for pitch control (instead of float), notes below F#3 (key 0) cannot be played and will instead be shifted up to F#4 (key 12). However, all notes above F#5 (key 24) will still play correctly.
