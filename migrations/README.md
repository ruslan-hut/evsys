# Database Migrations

This directory contains database migration scripts for EVSYS.

## Overview

Migrations are used to evolve the database schema as the application grows and changes. Each migration is numbered sequentially and should be run in order.

## Migration Methods

### Method 1: Automatic Migration (Recommended)

The application automatically runs pending migrations on startup when database is enabled.

```bash
# Migrations run automatically when you start the application
./evsys -conf=config.yml
```

The migration system:
- Checks current schema version
- Runs only pending migrations
- Tracks migration status in `schema_version` collection
- Provides detailed logging

### Method 2: Manual Migration via CLI

You can manually run migrations using the provided CLI tool:

```bash
# Run all pending migrations
go run cmd/migrate/main.go -conf=config.yml

# Check current schema version
go run cmd/migrate/main.go -conf=config.yml -status

# Rollback to specific version (use with caution!)
go run cmd/migrate/main.go -conf=config.yml -rollback -version=0
```

### Method 3: Direct MongoDB Scripts

For scenarios where you need to run migrations without the Go application:

```bash
# Using mongosh (MongoDB 5.0+)
mongosh mongodb://localhost:27017/evsys 001_ocpp_multiversion.js

# Using legacy mongo shell
mongo evsys 001_ocpp_multiversion.js

# Or load manually
mongosh
use evsys
load("001_ocpp_multiversion.js")
```

## Available Migrations

### Migration 001: OCPP Multi-Version Support
**Status:** Available
**Version:** 1
**Files:**
- `001_ocpp_multiversion.js` - Forward migration
- `001_rollback.js` - Rollback script

**Changes:**
- Adds `protocol_version` field to `charge_points` (default: "ocpp1.6")
- Adds `device_model` field to `charge_points` (empty object)
- Adds `evse_id` field to `connectors` (null for OCPP 1.6 compatibility)
- Adds `protocol_version`, `evse_id`, and `metadata` to `transactions`
- Creates indexes on `protocol_version` fields
- Creates compound index on `(charge_point_id, evse_id)`

**Rollback:**
```bash
# Manual rollback
mongosh evsys 001_rollback.js

# Via CLI
go run cmd/migrate/main.go -conf=config.yml -rollback -version=0
```

## Schema Version Tracking

The current schema version is stored in the `schema_version` collection:

```javascript
{
  "version": 1,
  "updated_at": ISODate("2025-11-06T...")
}
```

To check current version:
```bash
mongosh evsys --eval "db.schema_version.findOne()"
```

## Best Practices

### Before Running Migrations

1. **Backup your database:**
   ```bash
   mongodump --db=evsys --out=/backup/evsys_$(date +%Y%m%d)
   ```

2. **Test on staging environment first**

3. **Review the migration script:**
   ```bash
   cat migrations/001_ocpp_multiversion.js
   ```

4. **Check current schema version:**
   ```bash
   mongosh evsys --eval "db.schema_version.findOne()"
   ```

### During Migration

1. **Monitor migration progress:**
   - Watch application logs
   - Monitor database performance
   - Check for errors

2. **Migration runs in background:**
   - Indexes are created with `background: true`
   - Application remains responsive

### After Migration

1. **Verify changes:**
   ```bash
   # Check a charge point document
   mongosh evsys --eval "db.charge_points.findOne()"

   # Check schema version
   mongosh evsys --eval "db.schema_version.findOne()"

   # Verify indexes
   mongosh evsys --eval "db.charge_points.getIndexes()"
   ```

2. **Monitor application logs** for any errors

3. **Test critical functionality:**
   - Charge point connections
   - Transaction creation
   - Billing calculations

## Rollback Procedure

If you need to rollback a migration:

1. **Stop the application**

2. **Restore from backup (safest):**
   ```bash
   mongorestore --db=evsys --drop /backup/evsys_20251106/evsys
   ```

3. **Or run rollback script:**
   ```bash
   mongosh evsys 001_rollback.js
   ```

4. **Verify rollback:**
   ```bash
   mongosh evsys --eval "db.schema_version.findOne()"
   ```

5. **Restart application**

## Creating New Migrations

When adding new migrations:

1. **Increment version number:** `002`, `003`, etc.

2. **Create migration file:** `002_description.js`

3. **Add to Go migrations:** Update `internal/migrations.go`

4. **Include:**
   - Clear description
   - Up function (apply changes)
   - Down function (rollback changes)
   - Index creation/removal
   - Schema version update

5. **Test thoroughly:**
   - Fresh database
   - Database with existing data
   - Rollback scenario

## Troubleshooting

### Migration Fails

1. Check database connectivity
2. Review error logs
3. Verify database permissions
4. Check for data inconsistencies
5. Consider rollback and retry

### Duplicate Key Errors on Indexes

If index creation fails due to duplicate keys:
```bash
# Find duplicates
mongosh evsys --eval "db.charge_points.aggregate([
  { $group: { _id: '$protocol_version', count: { $sum: 1 } } },
  { $match: { count: { $gt: 1 } } }
])"

# Clean up duplicates manually before rerunning migration
```

### Schema Version Mismatch

If schema version is out of sync:
```bash
# Reset to specific version
mongosh evsys --eval "db.schema_version.replaceOne({}, {version: 0, updated_at: new Date()}, {upsert: true})"

# Then rerun migrations
./evsys -conf=config.yml
```

## Support

For issues with migrations:
1. Check application logs
2. Review this README
3. Consult OCPP_MIGRATION_PLAN.md
4. Create issue in GitHub repository

---

**Last Updated:** 2025-11-06
**Current Schema Version:** 1
