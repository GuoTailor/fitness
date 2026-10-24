#!/usr/bin/env bash
# MySQL 每日备份（crontab -e）：
#   30 3 * * * /opt/fitness/scripts/backup_db.sh >> /var/log/fitness-backup.log 2>&1
# 保留 14 天；备份目录建议异机同步（rsync/对象存储）。
set -euo pipefail

BACKUP_DIR=/var/backups/fitness
KEEP_DAYS=14
DB=fitness
DB_USER=fitness
# 密码走环境变量或 ~/.my.cnf（[client] password=...），不要写在脚本里
mkdir -p "$BACKUP_DIR"
STAMP=$(date +%F)
mysqldump --single-transaction --routines --triggers -u"$DB_USER" "$DB" | gzip > "$BACKUP_DIR/$DB-$STAMP.sql.gz"
find "$BACKUP_DIR" -name "$DB-*.sql.gz" -mtime +$KEEP_DAYS -delete
echo "$(date '+%F %T') backup ok: $BACKUP_DIR/$DB-$STAMP.sql.gz"
