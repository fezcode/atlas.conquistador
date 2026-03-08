# atlas.conquistador

![Banner](banner-image.png)

`atlas.conquistador` is a powerful, beautiful, and functional terminal-based file explorer for the Atlas Suite. It provides a modern experience similar to Finder or Windows Explorer, right in your terminal.

## Features

- **Modern Boxed UI**: Centered titles, visible borders, and a clean aesthetic built with Bubble Tea.
- **Advanced Navigation**: 
  - `j/k` or Arrows for movement.
  - `PgUp/PgDown` for fast scrolling.
  - `Home/End` to jump to start or end.
  - "Go Up" highlighting: Automatically selects the directory you just came from.
- **Internal Previewers**:
  - **Text Viewer**: View common files (`.txt`, `.md`, `.go`, etc.) with line numbers and EOF markers.
  - **Hex Viewer**: Inspect binary files (like `.zip`, `.exe`) with a dedicated hex/ASCII display.
- **File Operations**:
  - **Multi-selection**: Select multiple items using `Space`.
  - **Copy/Cut/Paste**: Standard clipboard operations with status indicators.
  - **Delete**: Remove files/folders with safety confirmation.
  - **Create**: Create new files or folders with a quick dialog (`n`).
- **Dynamic Routing**: Navigate to any path (Relative or Absolute) using the `/` shortcut.
- **Cross-Platform**: Open files in your system's default application with `Enter`.

## Installation

```bash
# Using gobake
gobake build
./build/atlas.conquistador
```

## Usage

| Key | Action |
|-----|--------|
| `j`/`k`, `Arrows` | Navigate through files |
| `PgUp`/`PgDown` | Fast navigation / Page scroll |
| `Home`/`End` | Jump to start / end |
| `h`, `Left`, `Backspace` | Go to parent directory |
| `l`, `Right`, `Enter` | Open directory / Open externally |
| `v` | View file internally (plain text) |
| `m` | Hex view file |
| `n` | New file or folder |
| `Space` | Toggle selection |
| `/` | Go to specific path (Rel/Abs) |
| `c` / `x` / `p` | Copy / Cut / Paste |
| `d` | Delete items (with confirmation) |
| `s` | Cycle sort order (Name -> Size -> Time) |
| `?` | Show Help |
| `q`, `Esc` | Quit / Close Help or Viewers |

## License

MIT
