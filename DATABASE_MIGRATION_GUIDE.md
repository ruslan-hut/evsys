# Database Migration Guide - OCPP Multi-Version Support

**Migration Version:** 001
**Date Prepared:** 2025-11-06
**Target Schema Version:** 1
**Estimated Downtime:** 0 minutes (zero-downtime migration)
**Estimated Duration:** 1-30 seconds (depending on database size)

---

## Overview

This guide provides step-by-step instructions for migrating the EVSYS database to support multiple OCPP protocol versions (1.6J, 2.0.1, 2.1).

**What This Migration Does:**
- Adds `protocol_version` field to charge points and transactions
- Adds `evse_id` field to connectors (for OCPP 2.0.1+ EVSE hierarchy)
- Adds `metadata` field to transactions (for version-specific data)
- Creates database indexes for improved query performance
- Tracks schema version for future migrations

**Impact:**
- ✅ Zero downtime - application continues running
- ✅ Backward compatible - all existing data preserved
- ✅ Default values applied to existing records
- ✅ Safe to run multiple times (idempotent)

---

## Prerequisites

Before starting the migration:

### 1. System Requirements
- MongoDB 4.0 or higher
- EVSYS application version with migration support
- Database backup tools (`mongodump`, `mongorestore`)
- MongoDB shell (`mongosh` or legacy `mongo`)

### 2. Access Requirements
- MongoDB connection credentials with write permissions
- SSH/console access to application server
- Ability to restart application (for automatic migration)

### 3. Recommended Resources
- Available disk space: 2x database size (for backup)
- Time window: 15-30 minutes (includes backup and verification)

---

## Pre-Migration Checklist

Complete these steps before proceeding:

### ☐ 1. Create Database Backup

**Critical:** Always create a backup before any database migration.

```bash
# Create backup directory with timestamp
BACKUP_DIR="/backup/evsys_$(date +%Y%m%d_%H%M%S)"
mkdir -p $BACKUP_DIR

# Backup entire database
mongodump --host=localhost --port=27017 --db=evsys --out=$BACKUP_DIR

# Verify backup created successfully
ls -lh $BACKUP_DIR/evsys/

# Expected output: Multiple .bson and .metadata.json files
```

**Save the backup path for potential rollback:**
```bash
echo "Backup location: $BACKUP_DIR" > /tmp/migration_backup_path.txt
```

### ☐ 2. Document Current State

Record current database state for comparison:

```bash
# Connect to MongoDB
mongosh mongodb://localhost:27017/evsys

# Check current schema version (should return nothing or version 0)
db.schema_version.findOne()

# Count documents in each collection
db.charge_points.countDocuments()
db.connectors.countDocuments()
db.transactions.countDocuments()

# Sample one document from each collection (check for existing fields)
db.charge_points.findOne()
db.connectors.findOne()
db.transactions.findOne()

# List current indexes
db.charge_points.getIndexes()
db.connectors.getIndexes()
db.transactions.getIndexes()
```

**Save this output for verification after migration.**

### ☐ 3. Verify Application Version

Ensure you're running the correct application version with migration support:

```bash
# Check if migration code exists
ls -la /path/to/evsys/internal/migrations.go

# Verify migration scripts exist
ls -la /path/to/evsys/migrations/

# Expected files:
# - migrations/001_ocpp_multiversion.js
# - migrations/001_rollback.js
# - migrations/README.md
```

### ☐ 4. Notify Stakeholders

- Inform operations team of scheduled migration
- Notify monitoring team (expect brief log activity spike)
- Alert support team (in case of issues)

---

## Migration Methods

Choose **ONE** of the following methods:

---

## Method 1: Automatic Migration (Recommended)

**Best for:** Production deployments, minimal manual intervention
**Pros:** Fully automated, integrated with application startup
**Cons:** Requires application restart

### Step 1: Stop Current Application (Optional)

```bash
# If running as systemd service
sudo systemctl stop evsys

# If running in screen/tmux
# Find the process and stop gracefully
pkill -SIGTERM evsys

# If running in background
kill $(cat /var/run/evsys.pid)
```

**Note:** Application can remain running - migration happens on next startup.

### Step 2: Deploy New Application Version

```bash
# Backup current binary
cp /path/to/evsys /path/to/evsys.backup

# Deploy new binary with migration support
cp /path/to/new/evsys /path/to/evsys

# Verify binary
/path/to/evsys -version  # if version flag exists
```

### Step 3: Start Application (Migration Runs Automatically)

```bash
# Start application with configuration
/path/to/evsys -conf=/etc/conf/config.yml

# Or if using systemd
sudo systemctl start evsys
```

### Step 4: Monitor Migration Logs

Watch application logs for migration progress:

```bash
# If using systemd
sudo journalctl -u evsys -f

# Or tail log file
tail -f /var/log/evsys/app.log
```

**Expected log output:**
```
2025-11-06 10:30:00 mongodb is configured and enabled
2025-11-06 10:30:00 checking for pending database migrations...
2025-11-06 10:30:00 Current schema version: 0, Available migrations: 1
2025-11-06 10:30:00 Running migration 1: Add OCPP multi-version support fields
2025-11-06 10:30:01 Running migration: Add OCPP multi-version support fields
2025-11-06 10:30:01 Updated 15 charge points with protocol_version
2025-11-06 10:30:01 Updated 45 connectors with evse_id field
2025-11-06 10:30:01 Updated 1234 transactions with multi-version fields
2025-11-06 10:30:01 Creating indexes for new fields...
2025-11-06 10:30:02 Migration completed successfully
2025-11-06 10:30:02 Migration 1 completed successfully
2025-11-06 10:30:02 All migrations completed
2025-11-06 10:30:02 database schema is up to date (version 1)
```

**Warning signs to watch for:**
- `WARNING: database migration failed:` - Migration encountered an error
- Connection errors - Database connectivity issues
- Timeout errors - Migration taking too long (investigate)

### Step 5: Verify Migration Success

See **Post-Migration Verification** section below.

---

## Method 2: Manual Migration via MongoDB Shell

**Best for:** Maintenance windows, manual control, testing
**Pros:** Full control, can run without app restart
**Cons:** Requires manual execution and verification

### Step 1: Locate Migration Script

```bash
cd /path/to/evsys/migrations
ls -la 001_ocpp_multiversion.js

# Verify script contents
head -20 001_ocpp_multiversion.js
```

### Step 2: Execute Migration Script

**Using mongosh (MongoDB 5.0+):**
```bash
mongosh mongodb://localhost:27017/evsys 001_ocpp_multiversion.js
```

**Using legacy mongo shell (MongoDB 4.x):**
```bash
mongo mongodb://localhost:27017/evsys 001_ocpp_multiversion.js
```

**Or load interactively:**
```bash
mongosh mongodb://localhost:27017/evsys

# In mongosh prompt:
load("001_ocpp_multiversion.js")
```

### Step 3: Review Migration Output

**Expected output:**
```
========================================
Migration 001: OCPP Multi-Version Support
========================================

1. Updating charge_points collection...
   - Modified documents: 15
   - Matched documents: 15
   - Creating index on protocol_version...
   ✓ Index created successfully

2. Updating connectors collection...
   - Modified documents: 45
   - Matched documents: 45
   - Creating compound index on (charge_point_id, evse_id)...
   ✓ Index created successfully

3. Updating transactions collection...
   - Modified documents: 1234
   - Matched documents: 1234
   - Creating index on protocol_version...
   ✓ Index created successfully

4. Updating schema version...
   ✓ Schema version set to 1

========================================
Migration Summary:
========================================
Charge Points updated:  15
Connectors updated:     45
Transactions updated:   1234

✓ Migration completed successfully
========================================
```

### Step 4: Verify Migration Success

See **Post-Migration Verification** section below.

---

## Method 3: Manual Migration via MongoDB Commands

**Best for:** Custom environments, scripted deployments
**Pros:** Can be automated in deployment scripts
**Cons:** More error-prone, requires MongoDB expertise

### Step 1: Connect to MongoDB

```bash
mongosh mongodb://localhost:27017/evsys
```

### Step 2: Execute Migration Commands

Copy and paste each section:

```javascript
// 1. Update charge_points
db.charge_points.updateMany(
    { protocol_version: { $exists: false } },
    { $set: {
        protocol_version: "ocpp1.6",
        device_model: {}
    }}
);

// Create index
db.charge_points.createIndex(
    { protocol_version: 1 },
    { name: "protocol_version_1", background: true }
);

// 2. Update connectors
db.connectors.updateMany(
    { evse_id: { $exists: false } },
    { $set: { evse_id: null }}
);

// Create compound index
db.connectors.createIndex(
    { charge_point_id: 1, evse_id: 1 },
    { name: "charge_point_evse_1", background: true }
);

// 3. Update transactions
db.transactions.updateMany(
    { protocol_version: { $exists: false } },
    { $set: {
        protocol_version: "ocpp1.6",
        evse_id: null,
        metadata: {}
    }}
);

// Create index
db.transactions.createIndex(
    { protocol_version: 1 },
    { name: "protocol_version_1", background: true }
);

// 4. Update schema version
db.schema_version.replaceOne(
    {},
    { version: 1, updated_at: new Date() },
    { upsert: true }
);
```

### Step 3: Verify Each Command

After each `updateMany()` command, check the result:
```javascript
// Should show: { acknowledged: true, modifiedCount: N, ... }
```

---

## Post-Migration Verification

**Perform these checks regardless of migration method used:**

### ☐ 1. Verify Schema Version

```javascript
// Connect to MongoDB
mongosh mongodb://localhost:27017/evsys

// Check schema version
db.schema_version.findOne()

// Expected output:
{
    _id: ObjectId("..."),
    version: 1,
    updated_at: ISODate("2025-11-06T...")
}
```

✅ **Success:** `version: 1` is present

### ☐ 2. Verify Charge Points Collection

```javascript
// Check one charge point document
db.charge_points.findOne()

// Verify new fields exist:
// - protocol_version: "ocpp1.6"
// - device_model: {}

// Count documents with new fields
db.charge_points.countDocuments({ protocol_version: { $exists: true } })
db.charge_points.countDocuments({ device_model: { $exists: true } })

// Should match total charge point count
```

✅ **Success:** All documents have `protocol_version` and `device_model`

### ☐ 3. Verify Connectors Collection

```javascript
// Check one connector document
db.connectors.findOne()

// Verify new field:
// - evse_id: null (or integer)

// Count documents with new field
db.connectors.countDocuments({ evse_id: { $exists: true } })

// Should match total connector count
```

✅ **Success:** All documents have `evse_id` field

### ☐ 4. Verify Transactions Collection

```javascript
// Check one transaction document
db.transactions.findOne()

// Verify new fields:
// - protocol_version: "ocpp1.6"
// - evse_id: null (or integer)
// - metadata: {}

// Count documents with new fields
db.transactions.countDocuments({ protocol_version: { $exists: true } })
db.transactions.countDocuments({ evse_id: { $exists: true } })
db.transactions.countDocuments({ metadata: { $exists: true } })

// Should match total transaction count
```

✅ **Success:** All documents have new fields

### ☐ 5. Verify Indexes Created

```javascript
// Check charge_points indexes
db.charge_points.getIndexes()
// Should include: { name: "protocol_version_1", key: { protocol_version: 1 } }

// Check connectors indexes
db.connectors.getIndexes()
// Should include: { name: "charge_point_evse_1", key: { charge_point_id: 1, evse_id: 1 } }

// Check transactions indexes
db.transactions.getIndexes()
// Should include: { name: "protocol_version_1", key: { protocol_version: 1 } }
```

✅ **Success:** All three new indexes are present

### ☐ 6. Verify Document Counts Match

```javascript
// Document counts should not change (only fields added)
db.charge_points.countDocuments()    // Should match pre-migration count
db.connectors.countDocuments()       // Should match pre-migration count
db.transactions.countDocuments()     // Should match pre-migration count
```

✅ **Success:** All counts match pre-migration state

### ☐ 7. Test Application Functionality

```bash
# Test charge point connection (if test charge point available)
# - Connect charge point
# - Send BootNotification
# - Verify connection accepted

# Check application logs for errors
tail -100 /var/log/evsys/app.log | grep -i error

# Test transaction creation (if possible)
# - Start a test transaction
# - Check transaction record in database
# - Verify new fields are populated
```

✅ **Success:** Application functions normally, no errors

### ☐ 8. Monitor Application Performance

```bash
# Check CPU and memory usage
top -p $(pgrep evsys)

# Monitor database queries (if slow query log enabled)
# Look for slow queries using new indexes

# Check application response times
# - API endpoints should respond normally
# - WebSocket connections stable
```

✅ **Success:** Performance is normal or improved

---

## Rollback Procedures

**If migration fails or issues are detected, use one of these rollback methods:**

### Option 1: Restore from Backup (Safest)

```bash
# Stop application
sudo systemctl stop evsys

# Restore database from backup
BACKUP_DIR=$(cat /tmp/migration_backup_path.txt)
mongorestore --host=localhost --port=27017 --db=evsys --drop $BACKUP_DIR/evsys

# Verify restoration
mongosh mongodb://localhost:27017/evsys --eval "db.schema_version.findOne()"
# Should show version: 0 or no document

# Restart application with old version
cp /path/to/evsys.backup /path/to/evsys
sudo systemctl start evsys

# Verify application works
sudo journalctl -u evsys -f
```

### Option 2: Run Rollback Script

```bash
# Navigate to migrations directory
cd /path/to/evsys/migrations

# Execute rollback script
mongosh mongodb://localhost:27017/evsys 001_rollback.js

# Verify rollback
mongosh mongodb://localhost:27017/evsys --eval "db.schema_version.findOne()"
# Should show version: 0

# Check fields removed
mongosh mongodb://localhost:27017/evsys --eval "db.charge_points.findOne()"
# Should NOT have protocol_version or device_model

# Restart application
sudo systemctl restart evsys
```

### Option 3: Manual Rollback

```javascript
// Connect to MongoDB
mongosh mongodb://localhost:27017/evsys

// Remove fields from charge_points
db.charge_points.updateMany({}, {
    $unset: { protocol_version: "", device_model: "" }
});

// Remove fields from connectors
db.connectors.updateMany({}, {
    $unset: { evse_id: "" }
});

// Remove fields from transactions
db.transactions.updateMany({}, {
    $unset: { protocol_version: "", evse_id: "", metadata: "" }
});

// Drop indexes
db.charge_points.dropIndex("protocol_version_1");
db.connectors.dropIndex("charge_point_evse_1");
db.transactions.dropIndex("protocol_version_1");

// Reset schema version
db.schema_version.replaceOne(
    {},
    { version: 0, updated_at: new Date() },
    { upsert: true }
);
```

---

## Troubleshooting

### Issue: Migration Times Out

**Symptoms:**
- Migration runs for > 5 minutes
- Application startup hangs
- High database CPU usage

**Solutions:**
1. **Check database connection:** Ensure MongoDB is accessible and responsive
2. **Check database size:** Very large databases may need more time
3. **Run manual migration:** Use Method 2 or 3 with increased timeout
4. **Run during low-traffic period:** Reduce concurrent operations

**Commands:**
```bash
# Check MongoDB status
systemctl status mongod

# Check database size
mongosh --eval "db.stats(1024*1024)"  # Size in MB

# Check current operations
mongosh --eval "db.currentOp()"
```

### Issue: Duplicate Key Error on Index Creation

**Symptoms:**
```
Error: Index creation failed: duplicate key error
```

**Solutions:**
1. **Check for duplicate data:** Some documents may have duplicate values
2. **Review existing indexes:** Conflicting index may exist
3. **Skip index creation:** Migration will continue (indexes can be added later)

**Commands:**
```javascript
// Find duplicates in charge_points
db.charge_points.aggregate([
    { $group: { _id: "$charge_point_id", count: { $sum: 1 } } },
    { $match: { count: { $gt: 1 } } }
])

// List existing indexes
db.charge_points.getIndexes()
db.connectors.getIndexes()
db.transactions.getIndexes()

// Drop conflicting index if found
db.charge_points.dropIndex("index_name")
```

### Issue: Permission Denied

**Symptoms:**
```
Error: not authorized on evsys to execute command
```

**Solutions:**
1. **Check user permissions:** MongoDB user needs write permissions
2. **Grant permissions:** Add necessary roles to user
3. **Use admin credentials:** Temporarily use admin user for migration

**Commands:**
```javascript
// Check current user
db.runCommand({ connectionStatus: 1 })

// Grant permissions (as admin)
use admin
db.grantRolesToUser("evsys_user", [
    { role: "readWrite", db: "evsys" },
    { role: "dbAdmin", db: "evsys" }
])
```

### Issue: Application Won't Start After Migration

**Symptoms:**
- Application crashes on startup
- Error: "migration failed"
- Connection refused errors

**Solutions:**
1. **Check migration logs:** Review detailed error message
2. **Verify schema version:** Ensure version was updated correctly
3. **Rollback migration:** Use rollback procedures above
4. **Check configuration:** Verify database connection settings

**Commands:**
```bash
# Check detailed application logs
tail -200 /var/log/evsys/app.log

# Try starting in foreground for detailed output
/path/to/evsys -conf=/etc/conf/config.yml

# Verify database connection
mongosh mongodb://localhost:27017/evsys --eval "db.serverStatus()"
```

---

## Post-Migration Tasks

### ☐ 1. Document Migration Completion

Record migration details:
```bash
# Create migration completion record
cat > /var/log/evsys/migration_001_completed.txt << EOF
Migration: 001 - OCPP Multi-Version Support
Date: $(date)
Performed by: $(whoami)
Method: [Automatic/Manual MongoDB/Manual Commands]
Duration: [Time taken]
Documents updated:
  - Charge Points: [count]
  - Connectors: [count]
  - Transactions: [count]
Status: Success
Backup location: $(cat /tmp/migration_backup_path.txt)
EOF
```

### ☐ 2. Clean Up Temporary Files

```bash
# Remove temporary files
rm /tmp/migration_backup_path.txt

# Keep backup for 30 days before deletion
# DO NOT delete immediately - keep for rollback safety
```

### ☐ 3. Update Documentation

- Update deployment documentation with migration completion
- Note schema version in operations runbook
- Document any issues encountered and resolutions

### ☐ 4. Monitor Application

Monitor application for 24-48 hours:
- Check for any unexpected errors in logs
- Monitor database performance
- Verify charge point connections remain stable
- Check transaction processing

### ☐ 5. Notify Stakeholders

Inform teams of successful migration:
- Operations team: Migration complete
- Development team: Schema updated to v1
- Support team: System operating normally

---

## Quick Reference

### Check Schema Version
```bash
mongosh evsys --eval "db.schema_version.findOne()"
```

### Verify Migration Applied
```bash
mongosh evsys --eval "db.charge_points.findOne({}, {protocol_version: 1, device_model: 1})"
```

### List All Indexes
```bash
mongosh evsys --eval "db.charge_points.getIndexes()"
mongosh evsys --eval "db.connectors.getIndexes()"
mongosh evsys --eval "db.transactions.getIndexes()"
```

### Rollback Commands
```bash
# Using rollback script
cd /path/to/evsys/migrations
mongosh evsys 001_rollback.js

# Or restore from backup
mongorestore --db=evsys --drop /backup/path/evsys
```

---

## Support Contacts

For migration issues or questions:

1. **Review Documentation:**
   - `migrations/README.md` - Detailed migration documentation
   - `OCPP_MIGRATION_PLAN.md` - Overall migration strategy
   - `PHASE2_TASK2.7_IMPLEMENTATION.md` - Implementation details

2. **Check Logs:**
   - Application logs: `/var/log/evsys/app.log`
   - MongoDB logs: `/var/log/mongodb/mongod.log`
   - System logs: `journalctl -u evsys`

3. **Create Issue:**
   - GitHub repository: [Include issue tracker URL]
   - Include: logs, error messages, database stats

---

## Appendix: Expected Database State

### Before Migration (Schema Version 0)

**charge_points document:**
```javascript
{
    _id: "CP001",
    location_id: "LOC1",
    vendor: "ABB",
    model: "Terra 54",
    // ... other fields
    // NO protocol_version
    // NO device_model
}
```

**connectors document:**
```javascript
{
    _id: ObjectId("..."),
    charge_point_id: "CP001",
    connector_id: 1,
    status: "Available",
    // ... other fields
    // NO evse_id
}
```

**transactions document:**
```javascript
{
    _id: ObjectId("..."),
    transaction_id: 12345,
    charge_point_id: "CP001",
    connector_id: 1,
    id_tag: "USER001",
    // ... other fields
    // NO protocol_version
    // NO evse_id
    // NO metadata
}
```

### After Migration (Schema Version 1)

**charge_points document:**
```javascript
{
    _id: "CP001",
    location_id: "LOC1",
    vendor: "ABB",
    model: "Terra 54",
    protocol_version: "ocpp1.6",      // NEW
    device_model: {},                  // NEW
    // ... other fields
}
```

**connectors document:**
```javascript
{
    _id: ObjectId("..."),
    charge_point_id: "CP001",
    connector_id: 1,
    evse_id: null,                    // NEW
    status: "Available",
    // ... other fields
}
```

**transactions document:**
```javascript
{
    _id: ObjectId("..."),
    transaction_id: 12345,
    charge_point_id: "CP001",
    connector_id: 1,
    id_tag: "USER001",
    protocol_version: "ocpp1.6",      // NEW
    evse_id: null,                    // NEW
    metadata: {},                     // NEW
    // ... other fields
}
```

**schema_version collection:**
```javascript
{
    _id: ObjectId("..."),
    version: 1,                       // NEW COLLECTION
    updated_at: ISODate("2025-11-06T...")
}
```

---

**Document Version:** 1.0
**Last Updated:** 2025-11-06
**Next Review:** Before next migration

---

## Checklist Summary

Use this quick checklist when performing the migration:

**Pre-Migration:**
- [ ] Create database backup
- [ ] Document current state
- [ ] Verify application version
- [ ] Notify stakeholders

**Migration:**
- [ ] Choose migration method
- [ ] Execute migration
- [ ] Monitor migration progress
- [ ] Review migration output

**Verification:**
- [ ] Verify schema version = 1
- [ ] Verify charge_points updated
- [ ] Verify connectors updated
- [ ] Verify transactions updated
- [ ] Verify indexes created
- [ ] Verify document counts match
- [ ] Test application functionality
- [ ] Monitor performance

**Post-Migration:**
- [ ] Document completion
- [ ] Clean up temporary files
- [ ] Update documentation
- [ ] Monitor application 24-48h
- [ ] Notify stakeholders

**In Case of Issues:**
- [ ] Review troubleshooting section
- [ ] Consider rollback
- [ ] Contact support if needed

---

**END OF MIGRATION GUIDE**
