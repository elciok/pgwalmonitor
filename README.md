# pgwalmonitor

Checks PostgreSQL backup using WAL archiving status. If a problem is detected, it sends a notification e-mail.

It runs its checks, sends e-mails if needed and returns, so you probablu need to run it as a cron job to keep monitoring backups.

Compatible with PostgreSQL version >= 10.

## Configuration

You can use environment variables to configure pgwalmonitor.

### Environment variables

- WALMON_ORIGIN: String to identify server being monitored. It's sent along with the notification e-mail.
- WALMON_DATA_SOURCE_STRING: PostgreSQL data source connection string, containing information like host, user and password. Example: host=localhost port=5432 user=dba password=dba123 dbname=data_customers
- WALMON_SMTP_FROM: Sender e-mail address used to send notification e-mails.
- WALMON_SMTP_TO: E-mail address that will receive e-mail notifications.
- WALMON_SMTP_ADDRESS: STMP server address used to send e-mails.
- WALMON_SMTP_PORT: SMTP server port. Default: 587.
- WALMON_SMTP_USERNAME: User to authenticate to SMTP server.
- WALMON_SMTP_PASSWORD: Password to authenticate to SMTP server.
- WALMON_SMTP_DOMAIN: SMTP domain (hello).
- WALMON_SMTP_AUTH: SMTP authentication method. Default: *plain*.
- WALMON_MAX_WAL_FILES: Maximum number of WAL files in pg_wal directory. Don't set it or set it to zero if file count should not be checked. Default: 0.
- WALMON_COMMAND_FULL_BACKUP_DATE: Shell comand that will return last successful full backup date (format: YYYY-MM-DD). It needs to be a bash command. Example when using wal-g for backups: wal-g backup-list | awk 'END{print}' | awk '{print $2}' | cut -c1-10
- WALMON_FULL_BACKUP_DAYS: Maximum number of days for full backups. Default: 7.

## How it checks WAL archiving

WAL archiving is checked by querying *pg_stat_archiver*, which contains information about WAL archiving like when it has last failed and succeeded. The user in the data source string must be able to query *pg_stat_archiver*.

It can also check if number of WAL files in the pg_wal directory exceeds a threshold set useing *WALMON_MAX_WAL_FILES*. The user in the data source string must be able to run *pg_ls_dir*.

## How it checks Full backups

The shell command provided in *WALMON_COMMAND_FULL_BACKUP_DATE* is used to return the date when the last full backup was successful. It considers something is wrong if more than *WALMON_FULL_BACKUP_DAYS* has passed.