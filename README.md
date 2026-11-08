# file-tui

Convert a dropped or pasted file to another format from the terminal.

A minimalist Bubble Tea TUI with a Claude Code-inspired aesthetic. Detects file
type by magic bytes, offers only logically valid targets, and never overwrites
an existing file (collisions get `-2`, `-3`, …).

## Build

```bash
go build -o file-tui .
```

## Run

```bash
./file-tui              # empty UI, paste a path
./file-tui <file>       # open with file loaded
./file-tui -- <file>    # end-of-options marker
./file-tui --version    # print version
```

## Platforms

- **Linux (amd64, arm64):** Compiled and runtime-tested.
- **macOS (amd64, arm64):** Compiled successfully. Runtime not yet verified.
- **Windows (amd64):** Supported via cross-compilation. Process cancellation uses
  `taskkill` to terminate the converter process tree. Some optional conversion
  utilities must be installed separately.

## Keys

| State | Keys |
|---|---|
| empty | paste path · `enter` load · `ctrl+c` quit |
| ready | `↑↓` select · `enter` convert · `tab` show locked · `esc` new file · `q` quit |
| running | `esc` cancel · `ctrl+c` cancel |
| done | `o` open folder · `esc` new file · `q` quit |
| error | `r` retry · `esc` back · `q` quit |

## Output

Written beside the source as `file-tui-<name>.<ext>`. Multi-page PDF output is
`file-tui-<name>-1.png`, `-2.png`, …

## Dependencies

Detected at runtime. The more you have installed, the more targets unlock:
ImageMagick, ffmpeg, poppler, pandoc, LibreOffice, calibre, tesseract, 7-Zip.

## Archive extraction limits

ZIP, TAR, and compressed single-file archives (gz, bz2, xz, zst) are checked
during extraction: total bytes (default 2 GiB) and entry count (default 200k).
Override with `FILE_CLI_MAX_EXTRACT_MB` and `FILE_CLI_MAX_EXTRACT_ENTRIES`.

7z and RAR extraction is delegated to the external 7z/unar tool, which has no
built-in extraction budget. After extraction completes, the extracted directory
is verified against the same byte and entry limits. This is a **post-extraction**
check: resource consumption during extraction is not prevented. Only convert 7z
and RAR archives from trusted sources.

## Arch Linux (AUR)

A PKGBUILD is provided under `packaging/aur/`. To build locally:

```bash
cd packaging/aur
makepkg -si
```

To submit to the AUR, create an AUR package repository and push the PKGBUILD
following the AUR submission guidelines. No SSH credentials are stored in this
repository.

## Limitations

- 7z/RAR extraction relies on external tools (7z or unar). Post-extraction
  verification does not prevent resource consumption during extraction.
- `~user` paths are not expanded (only `~` and `~/`).
- Multiline paste selects the first existing path.
- Minimum terminal width: 20 columns.
- Requires Go 1.24 to build.
- Runtime dependencies (ImageMagick, ffmpeg, etc.) must be installed
  separately. Missing tools produce clear error messages.
## Windows

Windows is supported via cross-compilation. Process cancellation uses `taskkill`
to terminate the converter's process tree. Some optional conversion utilities
may need to be installed separately.
