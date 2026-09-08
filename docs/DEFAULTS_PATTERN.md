# Shipped defaults plus user content: how real projects model it

Research note. Every claim below comes from a file or documentation page that was
fetched and read. Where a thing could not be confirmed, it says "could not verify".

## What was read

| Project | Files / pages read |
|---|---|
| wger | `wger/exercises/models/base.py`, `wger/exercises/models/deletion_log.py`, `wger/exercises/sync.py`, `wger/exercises/api/views.py`, `wger/utils/models.py`, `wger/manager/models/routine.py`, `wger/manager/models/slot_entry.py`, `wger/manager/views/routine.py`, admin command docs |
| Grafana | `pkg/services/sqlstore/migrations/dashboard_mig.go`, `pkg/services/provisioning/dashboards/file_reader.go`, `pkg/services/dashboards/errors.go`, `pkg/services/sqlstore/migrations/ualert/tables.go`, provisioning docs |
| Odoo | `odoo/tools/convert.py`, `odoo/addons/base/views/ir_model_views.xml`, data-files reference, view-records reference |
| Home Assistant | `homeassistant/helpers/entity_registry.py`, `homeassistant/components/blueprint/schemas.py`, blueprint docs |
| Discourse | `app/models/site_setting.rb`, `app/models/translation_override.rb`, `app/models/badge.rb`, `lib/discourse.rb` |
| Mealie | `mealie/services/seeder/seeder_service.py`, `mealie/repos/seed/seeders.py` |
| Tandoor Recipes | `cookbook/models.py` |
| Redmine | `lib/redmine/default_data/loader.rb`, `app/models/query.rb` |
| Keycloak | `model/jpa/.../entities/AuthenticationFlowEntity.java` |
| Zabbix | template-linking manual, item-object API reference |
| Django | `django/core/serializers/base.py`, fixtures and serialization docs |
| WordPress | child-themes handbook page |
| PostgreSQL | `pg_depend` catalog docs |

---

## 1. The named patterns

### A. Fork on use (deep copy of the whole tree)

The shipped item and the user item are rows in the **same tables**. A flag says which is
which. To use a shipped item you copy it, and every child row with it. After the copy the
two are unrelated.

**wger** does exactly this, on a tree the same shape as the one in question.
`Routine` has `user` (FK, CASCADE), `is_template` with help text
*"Marking a workout as a template will freeze it and allow you to make copies of it"*, and
`is_public` with help text *"A public template is available to other users"*
([`wger/manager/models/routine.py`](https://github.com/wger-project/wger/blob/master/wger/manager/models/routine.py)).
The copy is a hand-written deep walk in
[`wger/manager/views/routine.py`](https://github.com/wger-project/wger/blob/master/wger/manager/views/routine.py):
`copy_routine` sets `routine_copy.user = request.user`, then loops
`routine.days` → `day.slots` → `slot.entries` → the per-entry configs (weight, reps, RIR,
rest, sets, and their `max` variants), setting `pk = None` on each copy before save.

**Home Assistant** calls the same move "take control". The docs say:
*"You can tweak an imported blueprint by 'taking control' of this blueprint. Home Assistant
then converts the blueprint automation into a regular automation"* and
*"By taking control, the blueprint is converted into an automation. You won't be able to
convert this back into a blueprint."*
([using blueprints](https://www.home-assistant.io/docs/automation/using_blueprints/))

**Keycloak** flags shipped authentication flows with a column, not a separate table:
`@Column(name="BUILT_IN") protected boolean builtIn;`
([`AuthenticationFlowEntity.java`](https://github.com/keycloak/keycloak/blob/main/model/jpa/src/main/java/org/keycloak/models/jpa/entities/AuthenticationFlowEntity.java)).
The UI route for changing a built-in flow is to duplicate it.

### B. Reference plus inputs (never copy; parametrise)

The user row holds a pointer to the shipped item and a bag of values. Nothing is copied.

**Home Assistant** blueprints. `BLUEPRINT_INSTANCE_FIELDS` is
`use_blueprint: {path: <yaml file>, input: {<key>: <value>}}`
([`homeassistant/components/blueprint/schemas.py`](https://github.com/home-assistant/core/blob/dev/homeassistant/components/blueprint/schemas.py)).
The shipped tree stays on disk in `blueprints/automation/`; only the pointer and the inputs
are the user's. The docs are explicit about the consequence:
*"Automations inherit from the blueprint they were built on, so if the blueprint is updated,
all automations using it pick up the change the next time Home Assistant reloads them."*
([blueprint docs](https://www.home-assistant.io/docs/blueprint/))

This is the only pattern found where a shipped update reaches existing user items
automatically. It buys that by allowing no structural edit at all — only the declared inputs.

### C. Sparse override table keyed to the shipped row

The shipped values are **not in the database**. Only the rows a user changed are.

**Discourse** site settings. Defaults are loaded from files —
`load_settings(Rails.root.join("config/site_settings.yml").to_s)` plus
`Dir[Rails.root.join("plugins/*/config/settings.yml").to_s]` — and the `site_settings`
table holds only what an admin changed
([`app/models/site_setting.rb`](https://github.com/discourse/discourse/blob/main/app/models/site_setting.rb)).

**Discourse** translation overrides is the same idea for shipped text, and it is the one
project read here that handles staleness honestly. The table is:

```
# Table name: translation_overrides
#  locale               :string  not null
#  original_translation :text
#  status               :integer default("up_to_date"), not null
#  translation_key      :string  not null
#  value                :string  not null
# Indexes
#  index_translation_overrides_on_locale_and_translation_key (locale,translation_key) UNIQUE
```

([`app/models/translation_override.rb`](https://github.com/discourse/discourse/blob/main/app/models/translation_override.rb))
It stores the shipped string it was written against in `original_translation`, and
`refresh_status` re-compares that against the currently shipped string to mark an override
`outdated` (shipped text changed), `deprecated` (shipped key gone), or invalid interpolation
keys. It does not resolve the conflict; it flags it for a human.

**WordPress** child themes are the file-system version: *"you have the option to overwrite
any template, part, or pattern that exists in the parent theme by adding a file of the same
name in your child theme"*, so as to *"Allow parent themes to be updated without losing your
modifications"*
([child themes](https://developer.wordpress.org/themes/advanced-topics/child-themes/)).
Override is by name, and a renamed parent file silently orphans the override.

### D. Same-row overlay (`original_*` columns beside user columns)

One row holds both the shipped value and the user's value, in different columns.

**Home Assistant**'s entity registry does this. `RegistryEntry` carries user fields `name`
and `icon` next to integration-supplied `original_name`, `original_icon` and
`original_device_class`
([`homeassistant/helpers/entity_registry.py`](https://github.com/home-assistant/core/blob/dev/homeassistant/helpers/entity_registry.py)).
The integration rewrites `original_*` freely on every restart; the user's `name` is untouched.
Reset to default is `name = None`. This is the cleanest of the patterns and it works only
because the overlay is a fixed, small set of scalar fields on one row.

### E. Side table that records provenance / external identity

The content row is ordinary. A second table says who owns it and where it came from.

**Odoo**, `ir.model.data`. Fields seen in its own admin views: `module`, `name`, `model`,
`res_id`, `noupdate`, `complete_name`, `reference`
([`ir_model_views.xml`](https://github.com/odoo/odoo/blob/master/odoo/addons/base/views/ir_model_views.xml)).
So `module.name` → `(model, res_id)`. Shipped records live in the normal business tables;
identity across upgrades lives in this side table. `noupdate` is the "do not clobber" bit.

**Grafana**, twice. `dashboard_provisioning` is
`id, dashboard_id, name, external_id (text), updated (int), check_sum (nvarchar 32)`, indexed
on `dashboard_id` and `(dashboard_id, name)`
([`dashboard_mig.go`](https://github.com/grafana/grafana/blob/main/pkg/services/sqlstore/migrations/dashboard_mig.go)).
Grafana alerting has a general one: `provenance_type` is
`id, org_id, record_key, record_type, provenance` with a unique index on
`(record_type, record_key, org_id)`
([`ualert/tables.go`](https://github.com/grafana/grafana/blob/main/pkg/services/sqlstore/migrations/ualert/tables.go)).
That is a generic "this row is owned by the file, not by you" marker, attachable to any
record type.

### F. Overlay as a new row that patches the shipped row

Not a copy and not a column — a separate record that describes a modification.

**Odoo** views. `ir.ui.view.inherit_id` points at the parent view, and the child holds xpath
inheritance specs. Resolution is: *"if the view has a parent, the parent is fully resolved,
then the current view's inheritance specs are applied; if the view has no parent, its arch is
used as-is"*
([view records](https://www.odoo.com/documentation/17.0/developer/reference/user_interface/view_records.html)).
Customisation is delivered as a new inheriting view, so the shipped view can still be updated
under it. The cost is that a patch is expressed against the parent's structure, and a parent
restructure breaks the xpath.

### G. Copy with a back-pointer to the origin

Copy the tree, but keep a foreign key on every copied child row saying which shipped row it
came from. Some fields stay editable locally; the rest are read-only and are re-pushed from
the parent.

**Zabbix** templates. A host's item carries `templateid`, the parent template item. Many
fields of a linked item are locked on the host; a few (enabled state, update interval,
history length, and *"some other parameters"*) can be changed locally. The manual is blunt
about the failure mode when a template is linked over existing entities:
*"previously existing identical entities on the host are updated as entities of the template,
and any existing host-level customizations to the entity are lost"*
([template linking](https://www.zabbix.com/documentation/current/en/manual/config/templates/linking)).
Unlink keeps the copies as plain host entities; "unlink and clear" deletes them.

This is the only pattern read that supports a per-field tweak inside a copied tree, and it is
also the only one with a documented data-loss path.

### H. Shared global library with no owner at all

Not on the list in the brief, but it is what the closest analogue actually does.

**wger** exercises have **no user column**. `Exercise` has `uuid` (unique, non-editable),
`category`, `muscles`, `equipment`, `variation_group`, `created`, `last_update`, and
`HistoricalRecords()`; it inherits `AbstractLicenseModel`, whose author information is
`license_author` (a `TextField`), not a foreign key to a user
([`base.py`](https://github.com/wger-project/wger/blob/master/wger/exercises/models/base.py),
[`utils/models.py`](https://github.com/wger-project/wger/blob/master/wger/utils/models.py)).
The API guards writes with a global permission, `permission_classes = (CanContributeExercises,)`
([`exercises/api/views.py`](https://github.com/wger-project/wger/blob/master/wger/exercises/api/views.py)).
So exercises are a wiki: one shared row, versioned history, no per-user variant. The "user
tweaks a shipped exercise" case is not modelled. What a user owns is the *routine*, and a
routine's `SlotEntry` references the shared `Exercise` and carries its own per-entry numbers —
`repetition_unit`, `repetition_rounding`, `weight_unit`, `weight_rounding`, `order`, `comment`,
`type`, `class_name`, `config` (JSON)
([`slot_entry.py`](https://github.com/wger-project/wger/blob/master/wger/manager/models/slot_entry.py)).
The numbers live on the user's side of the reference, so the library never needs a per-user
override.

### I. Reserved identity for system rows

Shipped rows sit in the user table but at reserved ids, or owned by a reserved account.

**Discourse** badges: a `system :boolean default(FALSE), not null` column, plus
`ensure_not_system` which does `self.id = [Badge.maximum(:id) + 1, 100].max unless id`, so
user badges start at 100 and shipped ones keep their low constant ids. Editing is partly
blocked by `self.protected_system_fields` =
`%i[name badge_type_id multiple_grant target_posts show_posts query trigger auto_revoke listable]`
([`app/models/badge.rb`](https://github.com/discourse/discourse/blob/main/app/models/badge.rb)).
So a shipped badge is partly editable: cosmetics yes, definition no.

**Discourse** also keeps a reserved actor: `SYSTEM_USER_ID = -1` with
`Discourse.system_user` looking up `User.find_by(id: SYSTEM_USER_ID)`
([`lib/discourse.rb`](https://github.com/discourse/discourse/blob/main/lib/discourse.rb)).
That is how a NOT NULL owner column can still express "the app owns this".

**PostgreSQL** does the same thing in its own catalogs: *"Most objects created during initdb
are considered 'pinned', which means that the system itself depends on them. Therefore, they
are never allowed to be dropped."*
([`pg_depend`](https://www.postgresql.org/docs/current/catalog-pg-depend.html))

### J. Nullable owner means "belongs to the app"

**Redmine**. `Query` declares plain `belongs_to :user`, and the seeded default queries are
created with no user at all — e.g.

```ruby
IssueQuery.create!(
  :name => l(:label_assigned_to_me_issues),
  :filters => {...},
  :visibility => Query::VISIBILITY_PUBLIC
)
```

([`lib/redmine/default_data/loader.rb`](https://github.com/redmine/redmine/blob/master/lib/redmine/default_data/loader.rb),
[`app/models/query.rb`](https://github.com/redmine/redmine/blob/master/app/models/query.rb)).
Those rows therefore have `user_id IS NULL`, and visibility is what makes them usable by
everyone. Redmine uses a flag for the other shipped thing it owns: roles have a `builtin`
integer, and the seeding guard tests `Role.where(:builtin => 0).exists?`.

This was the only clear nullable-owner case found in the code read. It is a minority choice.

### K. Seed into each tenant on creation

Shipped content is copied into the user's own tenant when the tenant is made, and from then
on it is simply the user's data.

**Mealie**. `SeederService` delegates to `IngredientFoodsSeeder`, `IngredientUnitsSeeder`,
`MultiPurposeLabelSeeder`; each queries the existing rows, compares on **name**, skips
matches, and writes new rows with `group_id=self.repos.group_id`
([`mealie/repos/seed/seeders.py`](https://github.com/mealie-recipes/mealie/blob/mealie-next/mealie/repos/seed/seeders.py),
[`seeder_service.py`](https://github.com/mealie-recipes/mealie/blob/mealie-next/mealie/services/seeder/seeder_service.py)).
No shipped row is ever updated afterwards. Mealie ships foods, units and labels this way; it
does not ship recipes.

### L. Ship nothing

**Tandoor Recipes** has no shipped recipe library. `Recipe` has
`space = ForeignKey(Space, on_delete=CASCADE)` and `created_by = ForeignKey(User, on_delete=PROTECT)`
(not nullable), with `ScopedManager(space='space', ...)`. There is no built-in flag and no
system-owned row
([`cookbook/models.py`](https://github.com/TandoorRecipes/recipes/blob/develop/cookbook/models.py)).
The `private` boolean is about sharing inside a space, not about shipped content. Worth stating
plainly: one of the two recipe managers named in the brief solves this problem by not having it.

### Not found

**Separate tables for shipped and user content** — no project read here does this. Every one
of them puts shipped and user rows in the same tables and separates them with a flag, a
reserved id, a null owner, or a side table. Could not verify any counter-example.

**A per-user diff against a shipped tree** — not found either. Override tables exist (C, D, F)
but only for flat, single-valued things: settings, strings, an entity's display name, one
view's xpath. For trees, every project read copies the tree.

---

## 2. Trade-offs, honestly

| | Shipped item updated in a release | Shipped item deleted | Can user version go stale? | "Reset to default" | One indexed query for "everything I can use" |
|---|---|---|---|---|---|
| **A. Fork on use** | Never reaches the fork | Fork survives, unaffected | Yes, always and silently | Easy: delete the fork, copy again | Yes: `WHERE owner_id = ? OR is_template` |
| **B. Reference + inputs** | Reaches every user item at once, wanted or not | Every user item breaks; HA fails to load the automation | No | Nothing to reset | Yes, but rendering needs the shipped tree at read time |
| **C. Sparse override table** | Shipped value moves under the override; override may now be wrong (Discourse marks it `outdated`) | Override orphaned; Discourse marks it `deprecated` | Yes, and it is detectable if you store the original | Easy: delete the override row | Needs a LEFT JOIN or two queries |
| **D. Same-row overlay** | Fine — `original_*` is rewritten, user column untouched | Row goes away with the user's edit inside it | No | Easy: set the user column to NULL | Yes, single table, no join |
| **E. Provenance side table** | You decide per row; Grafana lets the file win | Grafana deletes the row unless `disableDeletion` | n/a — the file is the truth | n/a | Yes, join or flag column |
| **F. Overlay row patching parent** | Parent updates apply under the patch, unless the patch's anchor moved | Patch is dead | Not the content, but the patch can break | Delete the patch row | Needs resolution at read time |
| **G. Copy + back-pointer** | Re-push can silently discard local edits (Zabbix says so) | Copies remain as plain local rows | Partly — locked fields track, local fields do not | Re-link, and lose local edits | Yes, the copies are the user's own rows |
| **H. Shared global library** | Everyone sees the change immediately | wger repoints references, see §4 | No | n/a | Yes — the cheapest of all |
| **K. Seed per tenant** | Never reaches existing tenants (Mealie only skips, never updates) | n/a | Yes | No mechanism | Yes, single tenant-scoped index |

Two specifics worth pulling out:

- Grafana blocks the edit rather than merging it. `ErrDashboardCannotSaveProvisionedDashboard`
  is *"Cannot save provisioned dashboard"*, status 400, alongside
  `ErrDashboardCannotDeleteProvisionedDashboard`
  ([`errors.go`](https://github.com/grafana/grafana/blob/main/pkg/services/dashboards/errors.go)).
  And if you do allow the edit, the docs say the file still wins: *"If you save a provisioned
  dashboard in the UI and then later update the provisioning source, Grafana always overwrites
  the database dashboard with the one from the provisioning file."* Also: *"If you save a
  provisioned dashboard in the UI and remove the provisioning source, Grafana deletes the
  dashboard in the database unless you have set the option `disableDeletion` to `true`."*
  ([provisioning](https://grafana.com/docs/grafana/latest/administration/provisioning/))
- Home Assistant's fork is one-way, by design and by documentation. There is no merge story
  anywhere in the systems read. Nobody three-way-merges shipped content.

---

## 3. What the majority actually do

There is a dominant convention, and it has three parts.

1. **Where shipped content is in the database at all, it shares the tables with user
   content.** That holds for every one of wger, Odoo, Grafana dashboards, Discourse badges,
   Keycloak, Redmine, Mealie and Zabbix. None of them used separate tables. The
   distinction is a flag (`wger.Routine.is_template`, `badges.system`,
   `AUTHENTICATION_FLOW.BUILT_IN`, `roles.builtin`), a reserved id or owner
   (Discourse badge ids under 100, `SYSTEM_USER_ID = -1`, PostgreSQL pinned OIDs), a null
   owner (Redmine queries), or a side table (`ir_model_data`, `dashboard_provisioning`,
   `provenance_type`). The other projects read — Discourse site settings, Home Assistant
   blueprints, Grafana provisioning files, WordPress parent themes — keep the shipped values
   out of the database entirely, so the question does not arise for them.
2. **Shipped rows are identified by a stable non-numeric key, never by autoincrement id.**
   wger uses `uuid`. Odoo uses `module.name`. Grafana uses the file path in
   `external_id`. Discourse uses `(locale, translation_key)` and fixed badge ids. Mealie uses
   the name. This is the single most consistent finding in the whole exercise.
3. **A user tweak of a shipped item is a copy, not an edit** — unless the tweakable surface is
   a small fixed set of scalars, in which case it is an overlay column or an override row.
   wger copies. Home Assistant copies ("take control"). Keycloak duplicates. Grafana refuses
   the edit outright. Only Zabbix attempts per-field local override inside a copied tree, and
   it documents losing those overrides on re-link.

Where projects genuinely diverge: **whether a shipped update should reach existing user
items.** Home Assistant blueprints say yes and forbid structural edits. Grafana says yes and
forbids edits. wger, Keycloak and Mealie say no. Odoo makes it a per-record decision via
`noupdate`. There is no consensus here, and the choice is a product decision, not a schema one.

How this was established: by reading the model, migration and seeding files listed in the
table at the top, and counting. It is a sample of a dozen projects chosen for closeness to the
problem, not a survey. Take the count as "everything I looked at", not as a statistic.

---

## 4. The re-seeding problem

This is the part with the clearest answers.

| Project | Match key | On re-run | Protects user edits by |
|---|---|---|---|
| **wger** | `uuid` | `Exercise.objects.update_or_create(uuid=uuid, defaults={...})` in `sync.py` | Shipped exercises are overwritten by design (they are shared, not personal). Locally created exercises have UUIDs the remote has never heard of, so they are never touched. The docs put it as: *"Exercises that you added manually to the database are not touched."* |
| **wger deletions** | `DeletionLog.uuid` + `replaced_by` | Remote publishes a deletion log; local looks up `Exercise.objects.get(uuid=uuid)` and calls `old_exercise.delete(replace_by=...)` | Before deleting, it repoints `SlotEntry` and `WorkoutLog` rows to the replacement UUID, then resets the affected routine caches, in one transaction. `DeletionLog` fields: `model_type`, `uuid` (unique), `replaced_by` (nullable UUID, *"UUID of the object replaced by the deleted one"*), `timestamp`, `comment`. |
| **Odoo** | external id `module.name` → `(model, res_id)` in `ir_model_data` | Data files reload on every module upgrade and **update the same row** rather than duplicating, because the external id resolves to it | The `noupdate` flag. From `odoo/tools/convert.py`: `if self.noupdate and self.mode != 'init': ... if record := env['ir.model.data']._load_xmlid(xid): self.idref[xid] = record.id; return None`, with the comment *"if the resource already exists, don't update it but store its database id (can be useful)"*. Docs: *"If the content of the data file is expected to be applied only once, you can specify the odoo flag `noupdate` set to 1"*, and `forcecreate` — *"in update mode whether the record should be created if it doesn't exist. Requires an external id, defaults to `True`."* Customisations are also kept out of the way by being new inheriting rows (`inherit_id`) rather than edits to the shipped row. |
| **Grafana** | file path in `dashboard_provisioning.external_id`, plus `check_sum` | `file_reader.go` looks up `provisionedDashboardRefs[path]` and computes `upToDate = jsonFile.checkSum == provisionedData.CheckSum`; unchanged files are skipped, changed files re-saved | It does not protect user edits. It prevents them (`allowUiUpdates: false` → 400 `Cannot save provisioned dashboard`), and where allowed, the file wins on the next pass. |
| **Mealie** | `name`, within the group | Query all existing, build a set of seen names, `continue` past any match; write the rest with `group_id` | Skip-if-exists. Nothing is ever updated, so a user edit is safe, and a shipped correction never lands. |
| **Redmine** | none — an all-or-nothing guard | `raise DataAlreadyLoaded.new("Some configuration data is already loaded.") unless no_data?`, where `no_data?` is `!Role.where(:builtin => 0).exists? && !Tracker.exists? && !IssueStatus.exists? && !Enumeration.exists? && !Query.exists?` | Seed exactly once, refuse forever after. Simplest possible answer; no upgrade path for shipped data. |
| **Discourse** | `(locale, translation_key)`; settings by name | Shipped values are re-read from YAML on boot. The DB holds only overrides. | Structural: there is nothing in the DB to clobber. Staleness is surfaced instead of resolved — `original_translation` is compared against the shipped string to set `status` to `outdated` / `deprecated`. |
| **Home Assistant** | blueprint file path | Re-import the file; every automation referencing that path picks up the change on the next reload | The user's data is only inputs, so a blueprint update cannot destroy it — but it can change behaviour under it. Users who need more fork out with "take control". |
| **Django (general)** | primary key | `loaddata` deserialises and calls `models.Model.save_base(self.object, using=using, raw=True)` in `DeserializedObject.save()` — an explicit-PK save, so a matching row is overwritten | Nothing. The docs warn against it: *"You should never include automatically generated objects in a fixture or other serialized data. By chance, the primary keys in the fixture may match those in the database and loading the fixture will have no effect. In the more likely case that they don't match, the fixture loading will fail with an `IntegrityError`."* The recommended fix is natural keys — *"a tuple of values that can be used to uniquely identify an object instance without using the primary key value"*. Fixtures are for tests. Real shipped data goes through data migrations or a management command, which is what wger does. |
| **Rails / Redmine seeds** | whatever the seed code checks | Idempotency is hand-written | The Redmine guard above is the convention: check, then refuse or skip. Rails ships no upsert-by-natural-key mechanism for `db/seeds.rb`. |
| **WordPress** | file name | Parent theme updates freely | The override is a different file in a different directory. *"Allow parent themes to be updated without losing your modifications."* |

**The short answer.** Two mechanisms, and only two:

- **Upsert on a stable external key** — UUID (wger), `module.name` (Odoo), file path (Grafana),
  name (Mealie). This handles both "don't duplicate" and "don't lose the link". A per-row
  opt-out (`noupdate`) is how you carve out the rows a user is allowed to own.
- **Keep the shipped values out of the database entirely** — Discourse settings, Home Assistant
  blueprints, Grafana provisioning files, WordPress parent themes. Then re-seeding is not a
  database problem at all. This is strictly the easiest of the two, and it is why it keeps
  turning up.

Matching on autoincrement primary keys is the one approach with a documented warning against
it, in Django's own docs.

---

## 5. Recommendation for a Go/GORM app, Postgres and SQLite, shipped content is a tree

Restating the problem: sessions contain blocks, blocks contain exercises; a user wants to
change one number on one exercise inside one shipped session.

**Use fork on use (pattern A) at the whole-session level, with a stable UUID on every shipped
row and an origin back-pointer on the fork.** This is what wger does with the same tree shape,
and it is the pattern to copy.

Concretely, the pieces that matter:

- One set of tables for shipped and user sessions. An owner column, and a `uuid` unique column
  on every row in the tree.
- Shipped rows are identified by `uuid`, never by id. Re-seeding is an upsert on `uuid` — wger's
  `update_or_create(uuid=...)`.
- The exercise library is referenced, not inlined, and the tunable numbers live on the
  *referencing* row (wger's `SlotEntry`), not on the library row. That removes the whole
  "user tweaks a library exercise" case, which is the awkward one.
- On fork, copy the tree and record where it came from — one `origin_uuid` on the forked
  session is enough. Keep it as a UUID value, not a foreign key, so a deleted shipped session
  does not cascade into a user's fork. wger's `DeletionLog.replaced_by` is exactly this
  shape and exists for exactly this reason.
- The one indexed query stays trivial: `WHERE owner_id = ? OR is_template = true`, with a
  partial index on the template side. It is one table, no join, and it works identically on
  Postgres and SQLite.

**What it costs, plainly:**

1. **A fork is frozen.** Improve a shipped session in the next release and nobody who forked it
   sees the improvement. There is no merge, and not one of the projects read has one. The
   mitigation everyone uses is a version or checksum on the origin so you can *tell the user*
   the source moved — Discourse's `original_translation` / `status` trio is the model, and it
   only ever flags, never merges.
2. **Copy amplification.** One tweaked number duplicates the entire session subtree. wger's
   `copy_routine` walks days, slots, entries and five or six config types by hand. That code is
   dull, it is easy to forget a new child table in it, and it is the thing that will silently
   break the next time the tree grows a level. Budget a test that forks a fully populated
   session and diffs the copy against the original.
3. **Two things called "session" in the UI.** The user now has "the shipped one" and "my copy of
   the shipped one", and every list, search and history view has to decide how to present that.
   This is a real product cost, not just a schema one, and it is where Home Assistant put an
   explicit one-way door and a warning.
4. **Reset to default is easy; partial reset is not.** Deleting the fork restores the default.
   Restoring one block inside a fork means re-copying that subtree from the origin, matching on
   `uuid` — doable, and worth deferring until someone asks.

**Why not the alternatives, given this specific shape:**

- **An override table keyed to the shipped exercise row** (`user_id, shipped_uuid, field, value`)
  is genuinely cheaper for exactly the stated case — one number, one row, trivial reset, and the
  shipped tree keeps improving underneath. It stops working the moment a user wants to reorder a
  block, drop an exercise, or add one, because there is no row to hang the override on. Every
  read also needs a LEFT JOIN per node. If the product promise is truly "tweak numbers only,
  never structure", this is the better answer and it is worth asking before building. Note that
  no project read here does this for a tree; the override tables found are all flat.
- **Reference plus inputs** (Home Assistant blueprints) fits only if the shipped session declares
  its tunable numbers up front and the user can change nothing else.
- **Copy with per-field back-pointers** (Zabbix) gives both, and Zabbix's own manual documents
  that re-linking loses the local customisations. That is the complexity of a merge engine
  without the safety of one.

