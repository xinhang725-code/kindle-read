# 长佩实现记录

Base: ea214a3. Inline execution. User authorized implementation and QR login.

Pre-flight: Task 1 -> Task 2 shares Account/Login and Book/Chapter models; preserve existing JSON contracts, add scanned state. Task 2 -> Task 3 shares source=changpei and app ID; use existing native identity, source filter only changes registration selection. No conflicts.

Task 1: in progress. Official public search/catalog and QR token response observed; real account authorization and hardware not available.

Task 1: complete. Missing client/parser tests failed first; protocol, rights, source IDs, corrupted frames, HTTP trailers, cancel/long-poll and session expiry now pass. Live search=10 items, catalog=64 chapters, discover=5 groups, QR creation and waiting observed; no real account authorization.
Task 2: complete. Browser gate/hidden-controls and punctuation tests failed first. Go race/vet + 5 Node tests + 6 browser tests pass. Structured cache preserves note blocks and 18px progress; all legacy routes still pass.
Task 3: packaging complete; single-source installer test failed before implementation, now passes. ARMv7 static ELF and two source-inclusive ZIPs built. Independent final review pending.
Implementation decision: user clarified DeepSeek C/LVGL document is a candidate, not an architecture mandate. Keep local helper/Mesquite, use visual design; alternatives and unmeasured device performance are documented in docs/changpei-architecture.md.

Final: fixed QR refresh serialization — TestRefreshInvalidatesInFlightQRBeforeItsApproval RED→GREEN.
Final: fixed expired session during page turns — browser page-turn authorization RED→GREEN.
Final: fixed cached locked chapter exposure — TestCachedChapterRespectsRefreshedRestriction RED→GREEN.
Final: fixed search viewport pagination — browser search splits RED→GREEN, batch-boundary regression RED→GREEN.
Final suite: sh scripts/check.sh PASS (Go race/vet + 5 Node); npm run test:browser PASS 9/9. No deferred minors.
Final: Ruling: real account confirmation/renewal/purchase validation — remain unverified without credentials; fixture success is not acceptance — risk: real login or reading can fail.
Final: Ruling: KPW5/Vera hardware behavior — remain development-only until device tests — risk: launch/render/sleep incompatibility.
Final: Ruling: cloud shelf/progress, comments, recharge/audio — explicitly deferred, no fake controls — cost: incomplete official-app parity.
Final: Ruling: image/Canvas content — explicit unsupported error — cost: some chapters unavailable.
Final: Ruling: eleven screens and remote /v1 — first local iteration, documented interface mapping — cost: not a complete C frontend backend contract.
Final: Ruling: C/LVGL/remote architecture — user delegated selection; keep local Go/Mesquite pending measurement — cost: possible future frontend migration.
Final: Ruling: paused WeRead/legacy features — preserve user-requested pause and regression coverage only — cost: historical missing features remain.
