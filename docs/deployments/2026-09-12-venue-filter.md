# Venue filter deployment — September 12, 2026

Deployed v0.3.11 (app commit 82b3d67) at 20:59 UTC after full Go tests, go vet, CI 34718267913, and Release 34718269530 passed. The intermediate v0.3.10 release build was canceled before deployment to include review corrections.

- Image digest: sha256:92b022c8ef4dc78ae999bc99bb229613e88766caf2acc69038db046a3c7935b4.
- Backup: /mnt/user/appdata/opportunity-hunter-backups/20260912T205852Z.
- Stopped rollback container: opportunity-hunter-rollback-v039.
- Isolated database-copy web smoke passed before replacement.
- Live Comedy Works Downtown selection returns 14 cards; score dropdown correctly says All (14). Aliased Comedy Works listings are included, South excluded.
- All hunt pages and System Status returned HTTP 200. SQLite quick_check OK; all 91 review backfill audit rows retained; zero undelivered notifications and zero container restarts.
- Comedy Discord remains off. No test feedback or notifications were sent.
- Startup comedy completed OK (0 new / 0 evaluated / 0 notified), performing arts OK (1 / 0 / 0), movies OK (2 / 1 / 0). Powder also completed OK (0 / 0 / 0). All four startup runs succeeded, with no startup WARN/ERROR logs.
- Initial diagnostic mistakenly used MAX(id) to identify latest runs; IDs are random strings, so this returned an old April comedy error. Rechecking by started_at confirmed the actual new comedy run succeeded. This was an operator query error, not an application regression.

Browser screenshots and independent-review dispositions are in ../reviews/2026-09-12-venue-filter/.
