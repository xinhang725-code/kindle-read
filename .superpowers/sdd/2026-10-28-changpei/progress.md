# 长佩实现记录

Base: ea214a3. Inline execution. User authorized implementation and QR login.

Pre-flight: Task 1 -> Task 2 shares Account/Login and Book/Chapter models; preserve existing JSON contracts, add scanned state. Task 2 -> Task 3 shares source=changpei and app ID; use existing native identity, source filter only changes registration selection. No conflicts.

Task 1: in progress. Official public search/catalog and QR token response observed; real account authorization and hardware not available.

Task 1: complete. Missing client/parser tests failed first; protocol, rights, source IDs, corrupted frames, HTTP trailers, cancel/long-poll and session expiry now pass. Live search=10 items, catalog=64 chapters, discover=5 groups, QR creation and waiting observed; no real account authorization.
Task 2: complete. Browser gate/hidden-controls and punctuation tests failed first. Go race/vet + 5 Node tests + 6 browser tests pass. Structured cache preserves note blocks and 18px progress; all legacy routes still pass.
Task 3: packaging complete; single-source installer test failed before implementation, now passes. ARMv7 static ELF and two source-inclusive ZIPs built. Independent final review pending.
Implementation decision: user clarified DeepSeek C/LVGL document is a candidate, not an architecture mandate. Keep local helper/Mesquite, use visual design; alternatives and unmeasured device performance are documented in docs/changpei-architecture.md.
