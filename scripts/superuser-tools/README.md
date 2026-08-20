# Superuser Tools — Platform Operator Scripts

Interactive bash scripts for platform superusers (system owners) to manage tenant environments (schools) and their admin accounts directly against PostgreSQL from your local terminal.

These are **platform owner tools**, distinct from school admins and teachers.

All scripts automatically connect to your production PostgreSQL container on the database server (`155.248.241.83`) over SSH using your OCI SSH key (`ssh-key-2026-06-29.key`) and hash passwords locally via `bin/pwtool.exe` without installing Go or psql on the database server.

---

## Interactive Prompts

All scripts prompt for their required parameters **step-by-step**:
```bash
cd scripts/superuser-tools
./create-school.sh
```

You can also pass arguments directly on the command line if preferred.

---

## Available Scripts

| Script | Purpose |
|---|---|
| `./create-school.sh` | Provision a brand-new tenant school environment AND its initial admin user |
| `./list-schools.sh` | Overview of all schools, with interactive detail view for any school |
| `./update-school.sh` | Update school information (name, school code, active/inactive status) |
| `./update-admin.sh` | Update admin account details (full name, email, password, phone) |
| `./manage-whatsapp.sh` | Enable/disable WhatsApp messaging per school and set monthly rate limits |
| `./gen-jwt-secret.sh` | Generate a cryptographically secure random `JWT_SECRET` for `.env` |
| `./backup-db.sh` | Dump the remote database directly to a local timestamped `.sql.gz` archive |
| `./restore-db.sh` | Restore from a backup archive (destructive, asks for confirmation) |

> **Note**: Superusers manage schools and their admins. Teacher accounts, class assignment, and student records are managed by each school's admin via the REST API or admin web portal.

---

## Interactive Usage Examples

### 1. Provision a New School & Admin
```bash
./create-school.sh
```
Prompts step-by-step:
1. `Enter School Name`: e.g. `Oakridge High School`
2. `Enter School Code`: e.g. `oakridge-hs`
3. `Enter Admin Full Name`: e.g. `Sarah Connor`
4. `Enter Admin Email Address`: e.g. `admin@oakridge.edu`
5. `Enter Admin Password`: (press Enter to auto-generate a secure random password)
6. `Enter Admin Phone Number`: (optional, e.g. `+1-555-0199`)

### 2. View All Tenant Schools & Inspect Details
```bash
./list-schools.sh
```
Shows the tenant summary table and asks:
`Enter school code to view details (or press Enter to finish):`

### 3. Update School Details
```bash
./update-school.sh
```
Prompts for school code, shows current values, and asks for new name, code, or status (`active`/`inactive`).

### 4. Update Admin Account
```bash
./update-admin.sh
```
Prompts for school code and admin email, then asks which fields to update (name, email, password, phone). If the password is changed, it is hashed locally and active sessions are revoked.

### 5. Manage WhatsApp Messaging & Limits
```bash
./manage-whatsapp.sh
```
Shows current usage and offers an interactive menu:
1. Enable WhatsApp messaging
2. Disable WhatsApp messaging
3. Update monthly message limit
4. Enable WhatsApp AND set monthly limit
5. Done / Exit

### 6. Local Database Backup
```bash
./backup-db.sh
```
Prompts for output directory (default `./backups`) and streams compressed database dump to local disk.
