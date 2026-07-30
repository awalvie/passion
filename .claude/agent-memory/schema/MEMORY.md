# Memory Index

- [Foreign keys not enforced](foreign-keys-not-enforced.md) — PRAGMA foreign_keys is OFF on the app's connection; all OnDelete:CASCADE tags are decorative, never credit them as real cascade safety
- [Catalog-managed backfill risk](catalog-managed-backfill-risk.md) — one-time "mark all existing rows as X" migrations are unsafe once a UI path for creating that row already exists; diff-vs-clean-import verification can't catch this class of bug
- [FK pragma rollout verified](fk-pragma-rollout-verified.md) — empirical results of enabling `_foreign_keys=on`: only 9 relations have real constraints, soft-deletes are inert to FK checks, flipping the pragma is safe on an already-inconsistent DB
