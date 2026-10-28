# Standalone WeRead reading implementation plan

**Status: paused before implementation.** User subsequently asked to put WeRead aside and work only on the Changpei Literature app. No production code or tests for this plan have been added.

> **For agentic workers:** Use superpowers:executing-plans to implement this continuation of the approved standalone design. Steps use checkbox syntax for tracking.

**Goal:** Give the existing independent application a real WeRead catalog and verified text chapter path, with versioned local caching and explicit unsupported states.

**Architecture:** Keep the Go helper and Mesquite UI. Port the fixed MiuRead Web reader URL, signed requests and shard decoder; read the official reader bootstrap as data, never execute its scripts. The uploaded 0.3.6-rc.4 package remains a comparison artifact, not our build source.

**Tech Stack:** Go 1.25.3, existing x/net HTML parser, ES5, existing Go/Node/Chromium tests. No KOReader or new device runtime.

**Spec:** ../specs/2026-10-27-kindle-qidian-design.md, ../../miuread-porting.md, ../../weread-0.3.6-rc.4-audit.md.

## Global constraints and implementation decisions

- User has no source for 0.3.6-rc.4. Continue the previously authorized independent implementation, without claiming to patch that binary.
- Fixed open-source baseline: MiuRead `581f9b660097aa42fe33170098346862cf0a9bbf`, AGPL-3.0-only. Retain attribution and corresponding source.
- Read authenticated account books only; no purchase, entitlement activation, automatic cloud progress writes, or account secrets in frontend/logs.
- First text pipeline accepts clearly free full chapters and explicitly purchased content. Unknown/member-only/partial-trial rights fail visibly until those contracts are validated. Never infer purchase from a successful transport or an available preview.
- Book identity, format and positive version must match the requested book. Changed versions require refreshing the directory; stale cache and anchors cannot silently apply to new content.
- Body decoding verifies every shard checksum, bounded size and UTF-8. EPUB text extraction preserves all body fragments in source order, ignores active/head content, and reports omitted non-text resources. Images and publisher layout are not claimed supported by this text milestone.
- Keep 100 MiB/account cache, 2 MiB text chapter limit, source/account isolation, existing native entries and local import behavior.
- Network failure, missing shard, expired auth, denied access or cancellation leaves committed chapters and bookmarks intact; no automatic retry storm.
- Live authenticated reading and KPW5/Vera tests require user-side verification. Contract tests and public-page observations cannot replace them.

## Review focus

1. Real reader bootstrap has another book under `sState`; use the requested `reader.bookInfo`, reject mismatch rather than mixing metadata/token contexts.
2. A last free chapter can be only a partial trial; reject its partial ratio even if a shard decodes.
3. Shelf metadata refresh must retain an already downloaded directory, cache and position.
4. A chapter/book revision change must isolate old cache and clear incompatible active anchors without destroying recovery data.
5. Multiple XHTML documents and media-only chapters must not become silently truncated or successful empty chapters.

### Task 1: Protocol and decoder

**Files:** new `internal/miuread/protocol.go`, `codec.go`, matching tests.

**Interfaces:** `readerURL(bookID, chapterID string) string`; `contentFields(bookID, chapterID, psvts string, now int64, nonce uint32) map[string]any`; `decodeShards(parts [][]byte) ([]byte, error)`.

- [ ] Write fixed URL/signature and checksum/shuffle/Unicode vectors; malformed, empty and oversized shard tests.
- [ ] Run `sh scripts/go.sh test ./internal/miuread`; observe missing implementation failure.
- [ ] Port only the pure protocol and decoder algorithms; no host libraries.
- [ ] Run tests; expected PASS. Commit tested task.

### Task 2: Catalog, readable text and versioned cache

**Files:** new `internal/miuread/reading.go`, `reading_test.go`; modify `internal/library/store.go`, `store_test.go` and `internal/miuread/auth.go`.

**Interfaces:** `Client.Catalog(ctx, bookID) (library.Book,error)` and `Client.Chapter(ctx, book, chapterID) ([]string,error)`; `Book.Revision` and `Chapter.Revision` define cache identity; `Store.MergeShelf([]Book)` atomically updates metadata while preserving existing catalogs. Any display limitation belongs to explicit catalog metadata.

- [ ] Write failing tests for exact reader context, whole-chapter permission, version mismatch, account change, multi-body text, media-only content, shelf refresh and cache revision changes.
- [ ] Run relevant Go tests and observe expected failures.
- [ ] Implement bounded bootstrap extraction, same-session content requests and parsing; retain original cache when a fetch fails.
- [ ] Run Go suite; expected PASS. Commit tested task.

### Task 3: Connect UI and validate deliverable

**Files:** `internal/web/server.go`, `server_test.go`, `assets/app.js`, browser tests, version/attribution/status docs and packaging metadata.

**Interfaces:** `POST api/catalog {book_id}` returns the stored book; `GET api/chapter` uses the account reader for uncached WeRead text. UI fetches a missing catalog on book selection and offers explicit directory refresh.

- [ ] Add failing HTTP integration and browser tests covering catalog→chapter→saved position→offline reopening; failure must retain usable cached directory.
- [ ] Wire the existing reader UI with clear text-mode/permission/verification limits; shelf refresh uses `MergeShelf`.
- [ ] Run `sh scripts/check.sh`, `npm run test:browser`, `sh scripts/build.sh`; inspect output and corresponding-source package.
- [ ] Update implementation status and notices, record public-page observations separately from authenticated/device verification, and request one fresh review of the change.

Later work remains the full MiuRead feature target: validated member rights, images/layout, precise cloud progress, notes, translation, statistics and device lifecycle. This plan does not re-label the original package or declare feature parity.
