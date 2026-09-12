# Saved downvote placement deployment — September 12, 2026

Deployed v0.3.12 (app commit 57a9fee) at 22:30 UTC after full Go tests, go vet, CI 34722433446, and Release 34722434456 passed.

- Image digest: sha256:f117a23bce0be3b6a074be4ef650489787a473291fa6c7fe498ddc748ee2a740.
- Backup: /mnt/user/appdata/opportunity-hunter-backups/20260912T223001Z.
- Stopped rollback container: opportunity-hunter-rollback-v0311.
- Isolated database-copy web smoke passed before replacement.
- Live comedy page placed the existing saved downvote (Jerry Seinfeld, card 314) inside the collapsed Not for me section, outside the 59 main cards. No duplicate IDs. Expanding the section showed the saved downvote and restoration hint. No production feedback was modified.
- Local browser tests on a database copy verified Cancel, save down, save up, retained venue/score/sort, and mobile layout. Integration tests cover latest feedback, placement, restoration, all-downvoted results, and unchanged AI scores.
- All four hunt pages and System Status returned HTTP 200. All four startup scans completed OK, each with zero new/evaluated/notified items.
- SQLite quick_check OK; all 91 review backfill audit rows and existing feedback retained. Zero undelivered notifications, container restarts, or startup WARN/ERROR logs.
- Comedy Discord remains off. No test notifications were sent.

Screenshots and review dispositions: ../reviews/2026-09-12-downvotes/.
