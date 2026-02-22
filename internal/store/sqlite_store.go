package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

type Reminder struct {
	ID          uuid.UUID `json:"id"`
	Message     string    `json:"message"`
	TargetWa    string    `json:"target_wa"`
	Recurrence  string    `json:"recurrence"`
	ScheduledAt time.Time `json:"scheduled_at"`
	IsActive    bool      `json:"is_active"`
}

var ErrReminderNotFound = errors.New("reminder not found")

type SQLiteStore struct {
	db          *sql.DB
	version     atomic.Uint64
	countCacheM sync.RWMutex
	countCache  map[string]countCacheEntry
}

type countCacheEntry struct {
	version uint64
	total   int
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &SQLiteStore{db: db}
	store.countCache = make(map[string]countCacheEntry)
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return store, nil
}

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS settings (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		);

		CREATE TABLE IF NOT EXISTS reminders (
			id           TEXT PRIMARY KEY,
			message      TEXT NOT NULL,
			target_wa    TEXT NOT NULL DEFAULT '',
			recurrence   TEXT NOT NULL DEFAULT '',
			scheduled_at DATETIME NOT NULL,
			is_active    INTEGER NOT NULL DEFAULT 1
		);

		CREATE INDEX IF NOT EXISTS idx_reminders_active_scheduled
			ON reminders(is_active, scheduled_at);

		CREATE INDEX IF NOT EXISTS idx_reminders_scheduled_at
			ON reminders(scheduled_at);

		CREATE INDEX IF NOT EXISTS idx_reminders_message
			ON reminders(message);

		CREATE INDEX IF NOT EXISTS idx_reminders_target_wa
			ON reminders(target_wa);

		CREATE INDEX IF NOT EXISTS idx_reminders_recurrence
			ON reminders(recurrence);

		CREATE TABLE IF NOT EXISTS reminder_dispatch_marks (
			reminder_id  TEXT NOT NULL,
			scheduled_at DATETIME NOT NULL,
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (reminder_id, scheduled_at),
			FOREIGN KEY (reminder_id) REFERENCES reminders(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_dispatch_marks_created_at
			ON reminder_dispatch_marks(created_at);

		CREATE TABLE IF NOT EXISTS reminder_target_dispatch_marks (
			reminder_id  TEXT NOT NULL,
			scheduled_at DATETIME NOT NULL,
			target_wa    TEXT NOT NULL,
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (reminder_id, scheduled_at, target_wa),
			FOREIGN KEY (reminder_id) REFERENCES reminders(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_target_dispatch_marks_created_at
			ON reminder_target_dispatch_marks(created_at);
	`)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) GetWANumber() string {
	var val string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = 'wa_number'").Scan(&val)
	if err != nil {
		return ""
	}
	return val
}

func (s *SQLiteStore) UpdateWANumber(number string) error {
	_, err := s.db.Exec(
		"INSERT INTO settings (key, value) VALUES ('wa_number', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		number,
	)
	if err == nil {
		s.bumpVersion()
	}
	return err
}

func (s *SQLiteStore) CreateReminder(r Reminder) error {
	_, err := s.db.Exec(
		"INSERT INTO reminders (id, message, target_wa, recurrence, scheduled_at, is_active) VALUES (?, ?, ?, ?, ?, ?)",
		r.ID.String(), r.Message, r.TargetWa, r.Recurrence, r.ScheduledAt.UTC(), boolToInt(r.IsActive),
	)
	if err == nil {
		s.bumpVersion()
	}
	return err
}

func (s *SQLiteStore) DeleteReminder(id uuid.UUID) error {
	res, err := s.db.Exec("DELETE FROM reminders WHERE id = ?", id.String())
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrReminderNotFound
	}
	s.bumpVersion()
	return nil
}

func (s *SQLiteStore) UpdateReminder(id uuid.UUID, updated Reminder) error {
	res, err := s.db.Exec(
		"UPDATE reminders SET message = ?, target_wa = ?, recurrence = ?, scheduled_at = ? WHERE id = ?",
		updated.Message, updated.TargetWa, updated.Recurrence, updated.ScheduledAt.UTC(), id.String(),
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrReminderNotFound
	}
	s.bumpVersion()
	return nil
}

func (s *SQLiteStore) DeleteAllReminders() error {
	_, err := s.db.Exec("DELETE FROM reminders")
	if err == nil {
		s.bumpVersion()
	}
	return err
}

func (s *SQLiteStore) ToggleReminderActive(id uuid.UUID) error {
	res, err := s.db.Exec("UPDATE reminders SET is_active = NOT is_active WHERE id = ?", id.String())
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrReminderNotFound
	}
	s.bumpVersion()
	return nil
}

func (s *SQLiteStore) GetReminders(cursor *uuid.UUID, limit int, search, sortBy, order string) ([]Reminder, string, int) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	whereClause := ""
	var args []interface{}
	if search != "" {
		whereClause = " WHERE (message LIKE ? OR target_wa LIKE ?)"
		like := "%" + search + "%"
		args = append(args, like, like)
	}

	total, ok := s.cachedTotal(search, s.version.Load())
	if !ok {
		countQuery := "SELECT COUNT(*) FROM reminders" + whereClause
		if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
			return []Reminder{}, "", 0
		}
		s.putCachedTotal(search, s.version.Load(), total)
	}

	sortCol := "rowid"
	switch sortBy {
	case "message":
		sortCol = "message"
	case "target":
		sortCol = "target_wa"
	case "time":
		sortCol = "scheduled_at"
	case "recurrence":
		sortCol = "recurrence"
	}
	sortDir := "ASC"
	if strings.EqualFold(order, "desc") {
		sortDir = "DESC"
	}
	if sortBy == "" {
		sortDir = "DESC"
	}
	orderClause := reminderOrderClause(sortCol, sortDir)

	cursorClause := ""
	cursorArgs := make([]interface{}, 0, 3)
	if cursor != nil {
		if clause, cArgs, ok := s.reminderCursorClause(*cursor, sortCol, sortDir); ok {
			cursorClause = clause
			cursorArgs = append(cursorArgs, cArgs...)
		}
	}

	queryWhere := whereClause
	if cursorClause != "" {
		if queryWhere == "" {
			queryWhere = " WHERE " + cursorClause
		} else {
			queryWhere += " AND " + cursorClause
		}
	}

	query := fmt.Sprintf(
		"SELECT id, message, target_wa, recurrence, scheduled_at, is_active FROM reminders%s ORDER BY %s LIMIT ?",
		queryWhere,
		orderClause,
	)
	queryArgs := append(append([]interface{}{}, args...), cursorArgs...)
	queryArgs = append(queryArgs, limit+1)

	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		return []Reminder{}, "", total
	}
	defer rows.Close()

	result := make([]Reminder, 0, limit)
	for rows.Next() {
		var r Reminder
		var idStr string
		var active int
		var scheduledAt string

		if err := rows.Scan(&idStr, &r.Message, &r.TargetWa, &r.Recurrence, &scheduledAt, &active); err != nil {
			continue
		}
		r.ID, _ = uuid.Parse(idStr)
		r.IsActive = active != 0
		r.ScheduledAt = parseReminderTime(scheduledAt)
		result = append(result, r)
	}

	nextCursor := ""
	if len(result) > limit {
		nextCursor = result[limit-1].ID.String()
		result = result[:limit]
	}

	return result, nextCursor, total
}

func (s *SQLiteStore) Version() uint64 {
	return s.version.Load()
}

func (s *SQLiteStore) ProcessDueReminders(sendFn func(rem Reminder) error) {
	now := time.Now().UTC()
	batchLimit := dueReminderBatchLimit()

	rows, err := s.db.Query(
		"SELECT id, message, target_wa, recurrence, scheduled_at FROM reminders WHERE is_active = 1 AND scheduled_at <= ? ORDER BY scheduled_at ASC LIMIT ?",
		now,
		batchLimit,
	)
	if err != nil {
		return
	}

	type dueReminder struct {
		id  string
		rem Reminder
	}
	due := make([]dueReminder, 0, 32)

	parsedSchedules := make(map[string]cron.Schedule)
	invalidSchedules := make(map[string]struct{})
	changed := false

	for rows.Next() {
		var r Reminder
		var idStr, scheduledAt string

		if err := rows.Scan(&idStr, &r.Message, &r.TargetWa, &r.Recurrence, &scheduledAt); err != nil {
			continue
		}
		r.ID, _ = uuid.Parse(idStr)
		r.ScheduledAt = parseReminderTime(scheduledAt)
		due = append(due, dueReminder{id: idStr, rem: r})
	}

	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return
	}

	for _, item := range due {
		idStr := item.id
		r := item.rem

		alreadyDispatched, err := s.hasDispatchMark(idStr, r.ScheduledAt)
		if err != nil {
			continue
		}
		shouldDeleteMark := alreadyDispatched

		if !alreadyDispatched {
			if err := sendFn(r); err != nil {
				continue
			}
			if err := s.putDispatchMark(idStr, r.ScheduledAt, now); err == nil {
				shouldDeleteMark = true
			}
		}

		updateApplied := false
		if r.Recurrence == "" {
			res, err := s.db.Exec("UPDATE reminders SET is_active = 0 WHERE id = ? AND is_active = 1", idStr)
			if err == nil {
				if n, rowsErr := res.RowsAffected(); rowsErr == nil && n > 0 {
					updateApplied = true
				}
			}
		} else {
			if _, invalid := invalidSchedules[r.Recurrence]; invalid {
				res, err := s.db.Exec("UPDATE reminders SET is_active = 0 WHERE id = ? AND is_active = 1", idStr)
				if err == nil {
					if n, rowsErr := res.RowsAffected(); rowsErr == nil && n > 0 {
						updateApplied = true
					}
				}
			} else {
				sched, ok := parsedSchedules[r.Recurrence]
				if !ok {
					parsed, err := dueReminderCronParser.Parse(r.Recurrence)
					if err != nil {
						invalidSchedules[r.Recurrence] = struct{}{}
						res, deactiveErr := s.db.Exec("UPDATE reminders SET is_active = 0 WHERE id = ? AND is_active = 1", idStr)
						if deactiveErr == nil {
							if n, rowsErr := res.RowsAffected(); rowsErr == nil && n > 0 {
								updateApplied = true
							}
						}
					} else {
						parsedSchedules[r.Recurrence] = parsed
						sched = parsed
					}
				}

				if !updateApplied && sched != nil {
					next := sched.Next(now)
					res, err := s.db.Exec(
						"UPDATE reminders SET scheduled_at = ? WHERE id = ? AND is_active = 1",
						next.UTC(),
						idStr,
					)
					if err == nil {
						if n, rowsErr := res.RowsAffected(); rowsErr == nil && n > 0 {
							updateApplied = true
						}
					}
				}
			}
		}

		if updateApplied {
			if shouldDeleteMark {
				_ = s.deleteDispatchMark(idStr, r.ScheduledAt)
			}
			_ = s.deleteTargetDispatchMarks(idStr, r.ScheduledAt)
			changed = true
		}
	}

	if changed {
		s.bumpVersion()
	}

	_ = s.cleanupDispatchMarks(now.Add(-14 * 24 * time.Hour))
	_ = s.cleanupTargetDispatchMarks(now.Add(-14 * 24 * time.Hour))
}

func (s *SQLiteStore) HasTargetDispatchMark(reminderID uuid.UUID, scheduledAt time.Time, target string) (bool, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return false, fmt.Errorf("target is required")
	}

	var exists int
	err := s.db.QueryRow(
		"SELECT 1 FROM reminder_target_dispatch_marks WHERE reminder_id = ? AND scheduled_at = ? AND target_wa = ?",
		reminderID.String(),
		scheduledAt.UTC(),
		target,
	).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func (s *SQLiteStore) PutTargetDispatchMark(reminderID uuid.UUID, scheduledAt time.Time, target string, now time.Time) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("target is required")
	}

	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO reminder_target_dispatch_marks (reminder_id, scheduled_at, target_wa, created_at) VALUES (?, ?, ?, ?)",
		reminderID.String(),
		scheduledAt.UTC(),
		target,
		now.UTC(),
	)
	return err
}

func (s *SQLiteStore) hasDispatchMark(reminderID string, scheduledAt time.Time) (bool, error) {
	var exists int
	err := s.db.QueryRow(
		"SELECT 1 FROM reminder_dispatch_marks WHERE reminder_id = ? AND scheduled_at = ?",
		reminderID,
		scheduledAt.UTC(),
	).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func (s *SQLiteStore) putDispatchMark(reminderID string, scheduledAt time.Time, now time.Time) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO reminder_dispatch_marks (reminder_id, scheduled_at, created_at) VALUES (?, ?, ?)",
		reminderID,
		scheduledAt.UTC(),
		now.UTC(),
	)
	return err
}

func (s *SQLiteStore) deleteDispatchMark(reminderID string, scheduledAt time.Time) error {
	_, err := s.db.Exec(
		"DELETE FROM reminder_dispatch_marks WHERE reminder_id = ? AND scheduled_at = ?",
		reminderID,
		scheduledAt.UTC(),
	)
	return err
}

func (s *SQLiteStore) cleanupDispatchMarks(olderThan time.Time) error {
	_, err := s.db.Exec(
		"DELETE FROM reminder_dispatch_marks WHERE created_at < ?",
		olderThan.UTC(),
	)
	return err
}

func (s *SQLiteStore) deleteTargetDispatchMarks(reminderID string, scheduledAt time.Time) error {
	_, err := s.db.Exec(
		"DELETE FROM reminder_target_dispatch_marks WHERE reminder_id = ? AND scheduled_at = ?",
		reminderID,
		scheduledAt.UTC(),
	)
	return err
}

func (s *SQLiteStore) cleanupTargetDispatchMarks(olderThan time.Time) error {
	_, err := s.db.Exec(
		"DELETE FROM reminder_target_dispatch_marks WHERE created_at < ?",
		olderThan.UTC(),
	)
	return err
}

func (s *SQLiteStore) reminderCursorClause(cursor uuid.UUID, sortCol, sortDir string) (string, []interface{}, bool) {
	cursorID := cursor.String()
	comparator := ">"
	if sortDir == "DESC" {
		comparator = "<"
	}

	var exists int
	if err := s.db.QueryRow("SELECT 1 FROM reminders WHERE id = ?", cursorID).Scan(&exists); err != nil {
		return "", nil, false
	}

	if sortCol == "rowid" {
		clause := fmt.Sprintf("rowid %s (SELECT rowid FROM reminders WHERE id = ?)", comparator)
		return clause, []interface{}{cursorID}, true
	}

	clause := fmt.Sprintf(
		"(%s %s (SELECT %s FROM reminders WHERE id = ?) OR (%s = (SELECT %s FROM reminders WHERE id = ?) AND id %s ?))",
		sortCol,
		comparator,
		sortCol,
		sortCol,
		sortCol,
		comparator,
	)
	return clause, []interface{}{cursorID, cursorID, cursorID}, true
}

func reminderOrderClause(sortCol, sortDir string) string {
	if sortCol == "rowid" {
		return "rowid " + sortDir
	}
	return fmt.Sprintf("%s %s, id %s", sortCol, sortDir, sortDir)
}

var dueReminderCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
var countCacheMaxEntries = 256

const (
	defaultDueReminderBatchLimit = 200
	maxDueReminderBatchLimit     = 5000
)

func dueReminderBatchLimit() int {
	raw := strings.TrimSpace(os.Getenv("REMINDER_DUE_BATCH_LIMIT"))
	if raw == "" {
		return defaultDueReminderBatchLimit
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return defaultDueReminderBatchLimit
	}
	if parsed > maxDueReminderBatchLimit {
		return maxDueReminderBatchLimit
	}
	return parsed
}

func (s *SQLiteStore) cachedTotal(search string, version uint64) (int, bool) {
	s.countCacheM.RLock()
	defer s.countCacheM.RUnlock()

	entry, ok := s.countCache[search]
	if !ok || entry.version != version {
		return 0, false
	}
	return entry.total, true
}

func (s *SQLiteStore) putCachedTotal(search string, version uint64, total int) {
	s.countCacheM.Lock()
	defer s.countCacheM.Unlock()

	if len(s.countCache) >= countCacheMaxEntries {
		if _, exists := s.countCache[search]; !exists {
			s.countCache = map[string]countCacheEntry{}
		}
	}

	s.countCache[search] = countCacheEntry{
		version: version,
		total:   total,
	}
}

func parseReminderTime(value string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, value)
	if !t.IsZero() {
		return t
	}
	t, _ = time.Parse("2006-01-02 15:04:05-07:00", value)
	if !t.IsZero() {
		return t
	}
	t, _ = time.Parse("2006-01-02 15:04:05", value)
	return t
}

func ParseTargets(targetWa string) []string {
	raw := strings.Split(targetWa, ",")
	targets := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, t := range raw {
		t = strings.TrimSpace(t)
		if t != "" {
			if _, exists := seen[t]; exists {
				continue
			}
			seen[t] = struct{}{}
			targets = append(targets, t)
		}
	}
	return targets
}

func (s *SQLiteStore) bumpVersion() {
	s.version.Add(1)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
