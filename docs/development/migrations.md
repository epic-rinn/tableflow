# Goose migration authoring

No migrations have been authored. Add reviewed `<timestamp>_<description>.sql` files under `src/api/db/migrations/` as domain slices are implemented. Keep guides and AI instructions outside that source directory.

Each migration needs an Up section and an intentional documented reversal/recovery strategy. Transactional migrations are the default; isolate non-transactional operations such as concurrent index creation. See the [schema and migration rules](../../specs/architecture/data-model.md).
