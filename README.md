# file-cli

Convert a dropped or pasted file to another format from the terminal.

A minimalist Bubble Tea TUI with a Claude Code-inspired aesthetic. Detects file
type by magic bytes, offers only logically valid targets, and never overwrites
an existing file (collisions get `-2`, `-3`, …).

## Build

```bash
go build -o file-cli .
```

## Run

```bash
./file-cli              # empty UI, paste a path
./file-cli <file>       # open with file loaded
./file-cli -- <file>    # end-of-options marker
./file-cli --version    # print version
```

## Platforms

- **Linux (amd64, arm64):** Compiled and runtime-tested.
- **macOS (amd64, arm64):** Compiled successfully. Runtime not yet verified.
- **Windows:** Not supported. The conversion engine uses Unix process-group
  signaling (`syscall.Setpgid`, `syscall.Kill`) which is unavailable on Windows.

## Keys

| State | Keys |
|---|---|
| empty | paste path · `enter` load · `ctrl+c` quit |
| ready | `↑↓` select · `enter` convert · `tab` show locked · `esc` new file · `q` quit |
| running | `esc` cancel · `ctrl+c` cancel |
| done | `o` open folder · `esc` new file · `q` quit |
| error | `r` retry · `esc` back · `q` quit |

## Output

Written beside the source as `file-cli-<name>.<ext>`. Multi-page PDF output is
`file-cli-<name>-1.png`, `-2.png`, …

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

## Limitations

- 7z/RAR extraction relies on external tools (7z or unar). Post-extraction
  verification does not prevent resource consumption during extraction.
- `~user` paths are not expanded (only `~` and `~/`).
- Multiline paste selects the first existing path.
- Minimum terminal width: 20 columns.
- Requires Go 1.24 to build.
- Runtime dependencies (ImageMagick, ffmpeg, etc.) must be installed
  separately. Missing tools produce clear error messages.