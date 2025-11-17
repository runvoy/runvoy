# User Migration Script: Add _all Field

This script adds the `_all` field to existing user records in DynamoDB to support the new sorted query functionality.

## Background

The `ListUsers()` function has been optimized to delegate sorting to DynamoDB instead of sorting in memory. This requires:
1. A new Global Secondary Index (GSI) named `all-user_email` with:
   - Partition key: `_all` (constant value "USER")
   - Sort key: `user_email`
2. All user records must have the `_all` field set to "USER"

New users created after the code deployment will automatically have this field. However, existing users need to be migrated.

## When to Run

Run this script **after** deploying the CloudFormation stack update that adds the `all-user_email` GSI, but **before** you rely on the sorted query functionality.

## Prerequisites

- AWS credentials configured (via environment variables, AWS CLI config, or IAM role)
- Go 1.25+ installed
- Permissions to read and update items in the DynamoDB table

## Usage

### Dry Run (Recommended First)

Preview what would be updated without making changes:

```bash
cd scripts/migrate-users-add-all-field
go run main.go -table runvoy-api-keys -dry-run
```

### Actual Migration

Apply the changes:

```bash
go run main.go -table runvoy-api-keys
```

## What It Does

1. Scans all items in the specified DynamoDB table
2. For each user record without the `_all` field:
   - Updates the item to add `_all = "USER"`
3. Reports the number of users updated and any errors

## Safety

- The script only adds the `_all` field; it doesn't modify any other attributes
- Items that already have the `_all` field are skipped
- Use `-dry-run` first to verify what will be updated
- The operation is idempotent - safe to run multiple times

## Example Output

```
Migrating users in table: runvoy-api-keys
DRY RUN MODE - No changes will be made

Updating user: alice@example.com (api_key_hash: abc123def456...)
Updating user: bob@example.com (api_key_hash: 789xyz012345...)

Migration complete!
  Users updated: 2

Run without -dry-run to apply changes
```
