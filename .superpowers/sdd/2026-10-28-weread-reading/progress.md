# SDD ledger — plan: docs/superpowers/plans/2026-10-28-weread-reading.md

- PAUSED: user explicitly redirected work to Changpei Literature only. Protocol research and plan exist; no implementation started. Do not resume WeRead or Qidian without new direction.

- Base: ea214a3, branch work, clean isolated cloud checkout. Existing approved standalone spec and implementation retained.
- User confirms no source for the uploaded binary; parent announced continuing the editable independent application. This is an implementation choice, not a claim that user requested changing the uploaded binary's identity.
- Pre-flight: Task 1 produces protocol functions for Task 2; Task 2 produces versioned library models and reader client methods for Task 3. Existing library paragraphs remain the UI contract; media/layout limitations must be explicit.
- Public read-only observation: official reader bootstrap includes `sState.reader.bookInfo` for another edition and the actual `reader.bookInfo`/`chapterInfos`. Use the latter and validate exact book ID. Observed free-boundary metadata has `maxFreeChapterRatio`; do not treat all trial text as complete.
- Execution choice: inline continuation; prior user instructions already authorize implementation. No repeated design approval checkpoint. No device/account or original binary modification claimed.
