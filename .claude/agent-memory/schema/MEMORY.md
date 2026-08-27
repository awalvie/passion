# Memory Index

- [Foreign keys not enforced](foreign-keys-not-enforced.md) — SUPERSEDED 2026-08-27: this described the pragma as OFF; it was turned ON for real in commit 7c95a4c. Read fk-pragma-rollout-verified.md instead for current behavior.
- [Catalog-managed backfill risk](catalog-managed-backfill-risk.md) — one-time "mark all existing rows as X" migrations are unsafe once a UI path for creating that row already exists; diff-vs-clean-import verification can't catch this class of bug
- [FK pragma rollout verified](fk-pragma-rollout-verified.md) — LIVE as of commit 7c95a4c (not just a proposal): only 9 relations have real constraints and are now actually enforced; soft-deletes are inert to FK checks; new GORM associations will get real enforcement — prefer plain undeclared uint/*uint columns for new cross-references unless enforcement is actually wanted
