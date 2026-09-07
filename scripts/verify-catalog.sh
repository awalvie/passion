#!/usr/bin/env bash
# Verify the catalog state. Read-only: compares the live database against the most recent
# backup, so "nothing of mine changed" is checked rather than assumed.
#
# Counts live rows, not totals. The importer retires a template's child rows and writes a
# fresh generation on every run, so raw totals climb legitimately and a total-based check
# reads as a failure when nothing is wrong.
set -uo pipefail
cd "${1:-/opt/passion}" || exit 1

LIVE=passion.db
BAK=$(ls -1t passion.db.bak-* 2>/dev/null | head -n1)
if [ -z "$BAK" ]; then
  echo "FAIL  no passion.db.bak-* found to compare against"
  exit 1
fi

echo "live:   $LIVE"
echo "backup: $BAK"
echo

fail=0
q()  { sqlite3 -readonly "$LIVE" "$1"; }
qb() { sqlite3 -readonly "$BAK"  "$1"; }

check() { # label expected actual
  if [ "$2" = "$3" ]; then
    printf 'OK    %-46s %s\n' "$1" "$3"
  else
    printf 'FAIL  %-46s got %s, want %s\n' "$1" "$3" "$2"
    fail=1
  fi
}

CATALOG_TABLES="library_exercises activity_templates session_templates activities exercises exercise_media"
MINE="session_runs run_exercise_completions climbing_ticks scheduled_sessions
      manual_exercise_set_logs exercise_planned_sets run_exercise_choices
      climbing_exercise_meta session_journals training_cycles"

# --- no rows left under the ghost owner --------------------------------------
check "catalog owners remaining" "4" "$(q 'SELECT group_concat(DISTINCT owner_id) FROM library_exercises;')"
for t in $CATALOG_TABLES; do
  check "rows still owned by 1 in $t" "0" "$(q "SELECT COUNT(*) FROM $t WHERE owner_id=1;")"
done

# --- one catalog, the right size ---------------------------------------------
echo
printf '      %-46s %s / %s / %s\n' "library / blocks / sessions (live)" \
  "$(q 'SELECT COUNT(*) FROM library_exercises WHERE deleted_at IS NULL;')" \
  "$(q 'SELECT COUNT(*) FROM activity_templates WHERE deleted_at IS NULL;')" \
  "$(q 'SELECT COUNT(*) FROM session_templates WHERE deleted_at IS NULL;')"

# --- my training data, untouched ---------------------------------------------
echo
for t in $MINE; do
  check "$t unchanged" "$(qb "SELECT COUNT(*) FROM $t;")" "$(q "SELECT COUNT(*) FROM $t;")"
done

# --- owner 4's catalog, live rows only ---------------------------------------
echo
for t in $CATALOG_TABLES; do
  check "owner 4 $t live" \
    "$(qb "SELECT COUNT(*) FROM $t WHERE owner_id=4 AND deleted_at IS NULL;")" \
    "$(q  "SELECT COUNT(*) FROM $t WHERE owner_id=4 AND deleted_at IS NULL;")"
done

# --- orphans: a delta, not a verdict -----------------------------------------
# Live exercises under a retired activity. 426 of these predate all of this, from the
# importer bug fixed in September; --purge-orphans is the tool for them. What matters
# here is only that this operation added none.
echo
ORPH="SELECT COUNT(*) FROM exercises e WHERE e.deleted_at IS NULL AND e.activity_id IS NOT NULL AND e.activity_id NOT IN (SELECT id FROM activities WHERE deleted_at IS NULL);"
check "orphaned exercises (no new ones)" "$(qb "$ORPH")" "$(q "$ORPH")"

BROKEN="SELECT COUNT(*) FROM run_exercise_completions WHERE exercise_id NOT IN (SELECT id FROM exercises);"
check "completions with a missing exercise" "0" "$(q "$BROKEN")"

# --- integrity ----------------------------------------------------------------
echo
SHARED=$(( $(q 'SELECT COUNT(*) FROM library_exercises WHERE shared=1;')
         + $(q 'SELECT COUNT(*) FROM activity_templates WHERE shared=1;')
         + $(q 'SELECT COUNT(*) FROM session_templates WHERE shared=1;') ))
printf '      %-46s %s\n' "shared rows (0 until --publish-catalog)" "$SHARED"
check "integrity_check" "ok" "$(q 'PRAGMA integrity_check;' | head -n1)"

FK_NOW=$(q 'PRAGMA foreign_key_check;' | wc -l)
FK_BAK=$(qb 'PRAGMA foreign_key_check;' | wc -l)
check "foreign key violations (no new ones)" "$FK_BAK" "$FK_NOW"
if [ "$FK_NOW" != "0" ]; then
  echo
  echo "      $FK_NOW violation(s), all pre-existing; first 10:"
  q 'PRAGMA foreign_key_check;' | head -n 10 | sed 's/^/        /'
fi

echo
if [ "$fail" = "0" ]; then
  echo "ALL CHECKS PASSED"
else
  echo "SOMETHING FAILED. Read which check failed before acting: these are live-row"
  echo "counts, so a normal import does not move them."
  echo "If the failure is real: sudo systemctl stop passion && cp $BAK $LIVE"
fi
exit "$fail"
