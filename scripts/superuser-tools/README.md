# Admin Tools — Operator Scripts

Bash scripts for platform superusers to manage tenant environments (schools) and their admin accounts directly against PostgreSQL. These are **operator tools**, not part of the public REST API.

All scripts connect using `MIGRATE_DATABASE_URL` (falls back to `DATABASE_URL`) from your `.env` file — this should be the **owner/superuser** role (`attendance_owner`).

## Setup

```bash
cd scripts/admin-tools
chmod +x *.sh          # if not already executable
```

The first time you run a password-related script, it will automatically build `bin/pwtool` (a small Go binary that hashes passwords using the exact same argon2id parameters as the running API, so every hash it produces works with `POST /auth/login`).

## Scripts

| Script | Purpose |
|---|---|
| `create-school.sh` | Provision a brand-new tenant school environment AND its initial admin user |
| `update-school.sh` | Update school information (name, school_code, active status) |
| `update-admin.sh` | Update admin account details (full_name, email, password, phone) |
| `manage-whatsapp.sh` | Enable/disable WhatsApp messaging per school and set monthly rate limits |
| `list-schools.sh` | Overview of all schools, or detail view of one |
| `gen-jwt-secret.sh` | Generate a random `JWT_SECRET` for `.env` |
| `backup-db.sh` | Dump the full database to a timestamped `.sql.gz` |
| `restore-db.sh` | Restore from a backup (destructive, asks for confirmation) |

> **Note**: Superusers do not manage teachers directly via scripts; teacher account creation, class assignment, and student management are handled by the school's admin via the REST API endpoints.

## Examples

```bash
# Provision a new tenant environment (school + admin user)
./create-school.sh oakridge-hs "Oakridge High School" admin@oakridge.edu "Oakridge Admin" "AdminPass123!" "+1-555-0199"

# Update school details (name, code, active status)
./update-school.sh oakridge-hs --name "Oakridge International School" --status inactive

# Enable WhatsApp messaging and set monthly limit to 1500 messages
./manage-whatsapp.sh oakridge-hs --enable --limit 1500

# Disable WhatsApp messaging for a school
./manage-whatsapp.sh oakridge-hs --disable

# Check WhatsApp configuration and monthly usage for a school
./manage-whatsapp.sh oakridge-hs --status

# Update admin details (name, email, password, phone)
./update-admin.sh oakridge-hs admin@oakridge.edu --name "Principal Reyes" --password "NewSecurePass123!"

# List all tenant schools
./list-schools.sh

# Detail view of a specific school
./list-schools.sh oakridge-hs

# Nightly backup
./backup-db.sh /var/backups/attendance
```

## Notes

- **All scripts enforce PostgreSQL Tenant RLS**: Tenant-specific actions automatically set `app.current_school_id` so that operations execute safely within the tenant context.
- **Passwords are shown once**: If an optional password parameter is omitted during `create-school.sh`, a strong random password is generated and printed.
