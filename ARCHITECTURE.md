# file-tui Architecture

Generated from GitNexus knowledge graph analysis of `/home/arham/file-tui`.

## Overview

`file-tui` is a Go (1.24) terminal user interface built with [bubbletea](https://github.com/charmbracelet/bubbletea) v1.3.10 and [lipgloss](https://github.com/charmbracelet/lipgloss) v1.1.0. It converts a dropped, pasted, or argument-passed file to a chosen format through a menu-driven TUI. The application detects file type via magic bytes, probes installed conversion tools at startup, and offers only logically valid targets.

**Stats (from GitNexus index):**
- Files: 16 Go source files
- Symbols: 328
- Processes: 28 execution flows
- Clusters: 16 functional areas

## Functional Areas

The knowledge graph partitions the codebase into 16 clusters (Leiden communities). The six largest, highest-cohesion clusters map to these functional areas:

| Cluster | Symbols | Cohesion | Files | Responsibility |
|---|---|---|---|---|
| Cluster_14 | 22 | 83% | `ui.go` | **View layer** — all rendering: `View`, `header`, `cards`, `targetsPanel`, `renderRow`, `resultPanel`, `emptyZone`, `inputLine`, `footer`, `panelBox`, `place`, `innerWidth`, `humanSize`, `trunc`, `kv`, `chip`, `familyIcon`, `stateLabel`, `outputLine` |
| Cluster_4 | 22 | 83% | `convert.go`, `convert_test.go` | **Conversion engine** — `convert`, `planSingle`, `planMultiBase`, `magickArgs`, `ffmpegArgs`, `ffmpegProgress`, `globSize`, `removeOutputs` + 12 integration tests |
| Cluster_7 | 19 | 86% | `convert.go`, `info.go`, `ui.go`, `formats.go` | **Core update loop + model** — `Init`, `Update`, `key`, `mouse`, `start`, `loadResolved`, `rebuild`, `refreshOut`, `reset`, `move`, `tickCmd`, `toast`, `planFor`, `toolName`, `probeInfo`, `ffprobeInfo`, `pdfInfo` |
| Cluster_10 | 12 | 70% | `detect.go`, `formats.go`, `formats_test.go` | **Format availability** — `kindOf`, `Available`, `has`, `reasonFor` + 7 tests |
| Cluster_11 | 8 | 64% | `formats.go`, `formats_test.go` | **Conversion matrix** — `targetsFor`, `imageTargets`, `videoTargets`, `audioTargets`, `archiveTargets`, `subtitleTargets`, `dataTargets` + test |
| Cluster_9 | 8 | 88% | `detect.go`, `detect_test.go` | **Magic-byte detection** — `detect`, `kindOf`, `binaryControl`, `has`, `has0`, `splitExt` + 3 tests |

Remaining clusters (smaller, single-responsibility):

| Cluster | Symbols | Files | Responsibility |
|---|---|---|---|
| Cluster_0 | 6 | `archive.go` | Archive packing (`packArchive`, `packTar`, `addZip`, `collectFiles`, `step`) |
| Cluster_1 | 7 | `archive.go` | Archive extraction (`extractArchive`, `openTarStream`, `openCompressed`, `execStream`, `safeJoin`, `entry`) |
| Cluster_2 | 6 | `convert.go` | Command execution (`runCmd`, `convertLibreOffice`, `convertPandocPDF`, `lastLine`, `String`, `wait`) |
| Cluster_3 | 5 | `archive.go`, `limits_test.go` | Archive extraction budget (`newExtractBudget`, `add`, `writeStream`) |
| Cluster_12 | 6 | `path.go`, `formats_test.go` | Path resolution (`resolvePath`, `normalizePath`, `inputCandidates`, `unescapeShell`, `isAlnumByte`) |
| Cluster_13 | 8 | `main.go`, `probe.go`, `ui.go`, `ui_test.go` | Entry point + probe (`main`, `probeTools`, `newModel`) + UI tests |

## Key Execution Flows

The five most important cross-community processes (all 8 steps, traced via GitNexus):

### 1. Main → Used (application startup, file loaded via argument)
```
main (main.go)
  → newModel (ui.go)
    → loadResolved (ui.go)
      → rebuild (ui.go)
        → refreshOut (ui.go)
          → planFor (convert.go)
            → planMultiBase (convert.go)
              → used (convert.go)
```
Entry point: `main.go` parses args, constructs the Bubble Tea model, loads the source file, detects its kind, builds the target list, and plans the default output path.

### 2. Update → Used (user changes file via keyboard in ready state)
```
Update (ui.go)
  → key (ui.go)
    → loadResolved (ui.go)
      → rebuild (ui.go)
        → refreshOut (ui.go)
          → planFor (convert.go)
            → planMultiBase (convert.go)
              → used (convert.go)
```

### 3. Update → SrcBase (user changes file, output base path recomputed)
```
Update (ui.go)
  → key (ui.go)
    → loadResolved (ui.go)
      → rebuild (ui.go)
        → refreshOut (ui.go)
          → planFor (convert.go)
            → planMultiBase (convert.go)
              → srcBase (convert.go)
```

### 4. Main → SrcBase (startup with argument, output base path)
Same as flow 1 but terminates at `srcBase` instead of `used`.

### 5. Update → Kind (file type detection during load)
```
Update (ui.go)
  → key (ui.go)
    → loadResolved (ui.go)
      → detectFile (detect.go)
        → detect (detect.go)
          → kindOf (detect.go)
            → Kind (detect.go)
```

Other notable flows:
- **Update → Has0 / Main → Has0** (6 steps): feature availability check during load/update
- **ExtractArchive → ExtractBudget** (3 steps): archive extraction with bomb-protection budget (`archive.go`)
- **Key → FfprobeInfo / Init → FfprobeInfo / Init → PdfInfo** (4 steps): metadata probing for video/audio/PDF files

## Mermaid Architecture Diagram

```mermaid
graph TB
    %% Entry point
    MAIN[main.go<br/>main()] -->|creates model| NEWMODEL[ui.go<br/>newModel()]

    %% Model lifecycle
    NEWMODEL -->|loads file| LOAD[ui.go<br/>loadResolved()]
    LOAD -->|detects type| DETECT[detect.go<br/>detectFile()/detect()]
    LOAD -->|builds menu| REBUILD[ui.go<br/>rebuild()]
    REBUILD -->|plans output| REFRESH[ui.go<br/>refreshOut()]
    REFRESH -->|target paths| PLAN[convert.go<br/>planFor()]

    %% Format detection
    DETECT -->|magic bytes| KINDOF[detect.go<br/>kindOf()]
    KINDOF -->|extension tiebreak| KIND[detect.go<br/>Kind]

    %% Conversion matrix
    PLAN -->|target selection| TARGETS[formats.go<br/>targetsFor()]
    TARGETS -->|availability| AVAIL[formats.go<br/>Available()/has()]
    AVAIL -->|tool check| PROBE[probe.go<br/>probeTools()]

    %% Conversion execution
    PLAN -->|engine dispatch| CONVERT[convert.go<br/>convert()]
    CONVERT -->|ImageMagick| MAGICK[convert.go<br/>magickArgs()]
    CONVERT -->|ffmpeg| FFMPEG[convert.go<br/>ffmpegArgs()/ffmpegProgress()]
    CONVERT -->|pdftoppm| PDFP[convert.go<br/>pdfimg path]
    CONVERT -->|pandoc| PANDOC[convert.go<br/>convertPandocPDF()]
    CONVERT -->|LibreOffice| LO[convert.go<br/>convertLibreOffice()]
    CONVERT -->|calibre| CAL[convert.go<br/>ebook-convert]
    CONVERT -->|OCR| OCR[convert.go<br/>tesseract]
    CONVERT -->|data| DATA[convert.go<br/>convertData()]
    CONVERT -->|archive| ARC[archive.go<br/>convertArchive()]

    %% Archive sub-flow
    ARC -->|extract| EXTRACT[archive.go<br/>extractArchive()]
    ARC -->|repack| PACK[archive.go<br/>packArchive()/packTar()]
    EXTRACT -->|budget| BUDGET[archive.go<br/>newExtractBudget()/writeStream()]
    EXTRACT -->|stream| STREAM[archive.go<br/>openTarStream()/openCompressed()/execStream()]

    %% UI update loop
    NEWMODEL -->|Bubble Tea| INIT[ui.go<br/>Init()]
    INIT -->|tick| TICK[ui.go<br/>tickCmd()]
    INIT -->|info| INFO[ui.go<br/>infoCmd()]

    %% User interaction
    KEY[ui.go<br/>key()] -->|Enter| START[ui.go<br/>start()]
    KEY -->|Tab| TOGGLE[ui.go<br/>showLocked]
    KEY -->|↑↓| MOVE[ui.go<br/>move()]
    KEY -->|Esc| RESET[ui.go<br/>reset()]
    MOUSE[ui.go<br/>mouse()] -->|click| SELECT[ui.go<br/>hitbox match]

    %% Rendering
    START -->|running state| VIEW[ui.go<br/>View()]
    TOGGLE --> VIEW
    MOVE --> VIEW
    RESET --> VIEW
    SELECT --> VIEW
    VIEW -->|renders| HEADER[ui.go<br/>header()]
    VIEW -->|renders| CARDS[ui.go<br/>cards()]
    VIEW -->|renders| PANEL[ui.go<br/>targetsPanel()]
    VIEW -->|renders| PROGRESS[ui.go<br/>progressLine()]
    VIEW -->|renders| FOOTER[ui.go<br/>footer()]

    %% Styling
    VIEW -->|lipgloss styles| STYLE[ui.go<br/>colFg/colBg/colAccent/...]

    classDef entry fill:#f9f,stroke:#333,stroke-width:2px;
    classDef convert fill:#bbf,stroke:#333,stroke-width:1px;
    classDef detect fill:#bfb,stroke:#333,stroke-width:1px;
    classDef archive fill:#fbb,stroke:#333,stroke-width:1px;
    classDef ui fill:#fff,stroke:#333,stroke-width:1px;

    class MAIN entry;
    class CONVERT,MAGICK,FFMPEG,PDFP,PANDOC,LO,CAL,OCR,DATA,ARC,PACK convert;
    class DETECT,KINDOF,KIND detect;
    class EXTRACT,BUDGET,STREAM archive;
    class NEWMODEL,LOAD,REBUILD,REFRESH,PLAN,INIT,TICK,INFO,KEY,MOUSE,START,TOGGLE,MOVE,RESET,SELECT,VIEW,HEADER,CARDS,PANEL,PROGRESS,FOOTER,STYLE ui;
```

## Execution Flow Summary

1. **Startup**: `main()` → `newModel()` → `loadResolved()` → detect file type → `rebuild()` → `refreshOut()` → plan output path → Bubble Tea `Init()`
2. **User input**: `Update()` → `key()`/`mouse()` → state transition (ready → running → done/error)
3. **Conversion**: `start()` → `convert()` → engine-specific command → `doneMsg` → state update
4. **Rendering**: `View()` assembles header, source/details cards, target panel, progress/output line, footer — all styled with lipgloss
5. **Archive**: `convertArchive()` → extract to temp (with bomb budget) → repack to target format

## Files

| File | Purpose |
|---|---|
| `main.go` | Entry point, flag parsing, Bubble Tea program setup, `internal/boot` import |
| `ui.go` | Bubble Tea model, `Update`, `View`, all rendering, hitbox computation, keyboard/mouse handling |
| `convert.go` | `convert()` dispatcher, engine args, progress reporting, output naming, staging/cleanup |
| `detect.go` | Magic-byte file type detection, extension tiebreaking |
| `formats.go` | Declarative conversion matrix, `Targets`, `Tools.has()` availability |
| `probe.go` | Binary probing (magick formats, ffmpeg encoders, tesseract langs, PDF engines) |
| `archive.go` | Archive extraction/repacking, bomb budget, path traversal guards |
| `info.go` | Metadata probes (magick identify, ffprobe, pdfinfo) |
| `path.go` | Path resolution (quotes, escapes, `file://`, `~`, multi-line) |
| `internal/boot/boot.go` | Pre-bubbletea background pin to prevent OSC 11 startup hang |
