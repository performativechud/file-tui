# Graph Report - file-cli  (2026-11-19)

## Corpus Check
- 36 files · ~29,418 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 3 file(s) not represented in the graph (top: (none) 3)

## Summary
- 435 nodes · 1073 edges · 22 communities (18 shown, 4 thin omitted)
- Extraction: 77% EXTRACTED · 23% INFERRED · 0% AMBIGUOUS · INFERRED: 252 edges (avg confidence: 0.88)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `7ab3a14a`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- convert.go
- model
- testing.T
- go_pkg_os
- convert
- Functional Areas
- steps
- security_test.go
- file-tui
- Refactoring with GitNexus
- file-tui Architecture
- GitNexus CLI Commands
- GitNexus Guide
- Impact Analysis with GitNexus
- Debugging with GitNexus
- Exploring Codebases with GitNexus
- GitNexus — Code Intelligence
- GitNexus — Code Intelligence
- file-tui
- CHANGELOG.md
- known-issues.md

## God Nodes (most connected - your core abstractions)
1. `Functional Areas` - 49 edges
2. `model` - 38 edges
3. `fakeTools()` - 37 edges
4. `newModel()` - 35 edges
5. `convert()` - 31 edges
6. `autocomplete()` - 24 edges
7. `targetsFor()` - 23 edges
8. `kindOf()` - 20 edges
9. `Target` - 20 edges
10. `planSingle()` - 16 edges

## Surprising Connections (you probably didn't know these)
- `4. Main → SrcBase (startup with argument, output base path)` --references--> `srcBase()`  [INFERRED]
  ARCHITECTURE.md → convert.go
- `Functional Areas` --references--> `newExtractBudget()`  [INFERRED]
  ARCHITECTURE.md → archive.go
- `Execution Flow Summary` --references--> `convertArchive()`  [INFERRED]
  ARCHITECTURE.md → archive.go
- `Functional Areas` --references--> `extractArchive()`  [INFERRED]
  ARCHITECTURE.md → archive.go
- `Functional Areas` --references--> `openTarStream()`  [INFERRED]
  ARCHITECTURE.md → archive.go

## Import Cycles
- None detected.

## Communities (22 total, 4 thin omitted)

### Community 0 - "convert.go"
Cohesion: 0.07
Nodes (34): addZipFile(), convertArchive(), execStream(), extractArchive(), openCompressed(), openTarStream(), packArchive(), packTarFile() (+26 more)

### Community 1 - "model"
Cohesion: 0.08
Nodes (20): Execution Flow Summary, fetchSuggestions(), doneMsg, hit, infoMsg, model, progressMsg, Result (+12 more)

### Community 2 - "testing.T"
Cohesion: 0.13
Nodes (47): autocomplete(), TestAutocompleteUserExpansion(), TestAutocompleteUserFallback(), fakeTools(), TestDoneStateViewRendersAtNarrowWidth(), TestEmptyStateViewRendersAtNarrowWidth(), TestErrorStateViewRendersAtNarrowWidth(), newModel() (+39 more)

### Community 3 - "go_pkg_os"
Cohesion: 0.07
Nodes (23): expandUserDir(), joinPath(), splitPath(), TestAutocompleteSplitPathUser(), TestExpandUserDir(), TestResolvePathEdgeCases(), TestResolvePathForms(), TestExtractBudgetCapsBytes() (+15 more)

### Community 4 - "convert"
Cohesion: 0.14
Nodes (27): convert(), magickArgs(), planSingle(), makeDocx(), mustCmd(), TestArchive7zToTarGz(), TestArchiveRepackZipToTarGz(), TestConvertAudioIntegration() (+19 more)

### Community 5 - "Functional Areas"
Cohesion: 0.17
Nodes (29): Functional Areas, planFor(), removeOutputs(), kindOf(), Target, Tools, archiveTargets(), audioTargets() (+21 more)

### Community 6 - "steps"
Cohesion: 0.09
Nodes (29): runs-on, steps, strategy, GOARCH, GOOS, jobs, build-matrix, test (+21 more)

### Community 7 - "security_test.go"
Cohesion: 0.17
Nodes (17): newExtractBudget(), safeJoin(), verifyExtractBudget(), writeStream(), TestSafeJoinBlocksTraversal(), extractBudget, TestExtractBudgetCapsBytesAtDefault(), TestExtractBudgetCapsEntriesAtDefault() (+9 more)

### Community 8 - "file-tui"
Cohesion: 0.14
Nodes (13): Archive extraction limits, Dependencies, Features, file-tui, Install, Keys, License, Limitations (+5 more)

### Community 9 - "Refactoring with GitNexus"
Cohesion: 0.17
Nodes (11): Bind the repository first, Checklists, Example: Rename `validateUser` to `authenticateUser`, Extract Module, Refactoring with GitNexus, Rename Symbol, Risk Rules, Split Function/Service (+3 more)

### Community 10 - "file-tui Architecture"
Cohesion: 0.15
Nodes (12): 1. Main → Used (application startup, file loaded via argument), 2. Update → Used (user changes file via keyboard in ready state), 3. Update → SrcBase (user changes file, output base path recomputed), 4. Main → SrcBase (startup with argument, output base path), 5. Update → Kind (file type detection during load), Decision (2026-11-19), Decision (2026-11-19), file-tui Architecture (+4 more)

### Community 11 - "GitNexus CLI Commands"
Cohesion: 0.18
Nodes (10): After Indexing, analyze — Build or refresh the index, clean — Delete the index, Commands, GitNexus CLI Commands, Index storage and retention, list — Show all indexed repos, status — Check index freshness (+2 more)

### Community 12 - "GitNexus Guide"
Cohesion: 0.18
Nodes (10): Always Start Here, Control & data dependence (`pdg_query`), GitNexus Guide, Graph Schema, Paginating `list_repos`, Resources Reference, Shortest path between two symbols (`trace`), Skills (+2 more)

### Community 13 - "Impact Analysis with GitNexus"
Cohesion: 0.20
Nodes (9): Bind the repository first, Checklist, Example: "What breaks if I change validateUser?", Impact Analysis with GitNexus, Risk Assessment, Tools, Understanding Output, When to Use (+1 more)

### Community 14 - "Debugging with GitNexus"
Cohesion: 0.22
Nodes (8): Bind the repository first, Checklist, Debugging Patterns, Debugging with GitNexus, Example: "Payment endpoint returns 500 intermittently", Tools, When to Use, Workflow

### Community 15 - "Exploring Codebases with GitNexus"
Cohesion: 0.22
Nodes (8): Bind the repository first, Checklist, Example: "How does payment processing work?", Exploring Codebases with GitNexus, Resources, Tools, When to Use, Workflow

### Community 16 - "GitNexus — Code Intelligence"
Cohesion: 0.33
Nodes (5): Always Do, CLI, GitNexus — Code Intelligence, Never Do, Resources

### Community 17 - "GitNexus — Code Intelligence"
Cohesion: 0.33
Nodes (5): Always Do, CLI, GitNexus — Code Intelligence, Never Do, Resources

## Knowledge Gaps
- **1 isolated node(s):** `file-tui`
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 124 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Functional Areas` connect `Functional Areas` to `convert.go`, `model`, `testing.T`, `go_pkg_os`, `convert`, `security_test.go`, `file-tui Architecture`?**
  _High betweenness centrality (0.096) - this node is a cross-community bridge._
- **Are the 48 inferred relationships involving `Functional Areas` (e.g. with `execStream()` and `extractArchive()`) actually correct?**
  _`Functional Areas` has 48 INFERRED edges - model-reasoned connections that need verification._
- **What connects `file-tui` to the rest of the system?**
  _1 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `convert.go` be split into smaller, more focused modules?**
  _Cohesion score 0.0734006734006734 - nodes in this community are weakly interconnected._
- **Why does `model` connect `model` to `convert.go`, `testing.T`, `convert`, `Functional Areas`?**
  _High betweenness centrality (0.064) - this node is a cross-community bridge._
- **Are the 26 inferred relationships involving `fakeTools()` (e.g. with `TestDoneStateViewRendersAtNarrowWidth()` and `TestEmptyStateViewRendersAtNarrowWidth()`) actually correct?**
  _`fakeTools()` has 26 INFERRED edges - model-reasoned connections that need verification._
- **Should `model` be split into smaller, more focused modules?**
  _Cohesion score 0.08350168350168351 - nodes in this community are weakly interconnected._