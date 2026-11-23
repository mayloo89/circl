package reports

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// --- mock querier ---

type mockReportRow struct {
	scanFn func(dest ...any) error
}

func (r *mockReportRow) Scan(dest ...any) error { return r.scanFn(dest...) }

type mockReportRows struct {
	data    [][]any
	pos     int
	scanErr error
	rowsErr error
}

func (r *mockReportRows) Next() bool                                   { r.pos++; return r.pos <= len(r.data) }
func (r *mockReportRows) Close()                                       {}
func (r *mockReportRows) Err() error                                   { return r.rowsErr }
func (r *mockReportRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *mockReportRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockReportRows) Values() ([]any, error)                       { return nil, nil }
func (r *mockReportRows) RawValues() [][]byte                          { return nil }
func (r *mockReportRows) Conn() *pgx.Conn                              { return nil }
func (r *mockReportRows) TypeMap() *pgtype.Map                         { return nil }
func (r *mockReportRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.data[r.pos-1]
	for i, d := range dest {
		switch v := d.(type) {
		case *string:
			*v = row[i].(string)
		case *bool:
			*v = row[i].(bool)
		case *time.Time:
			if row[i] == nil {
				var zero time.Time
				*v = zero
			} else {
				*v = row[i].(time.Time)
			}
		case **time.Time:
			if row[i] == nil {
				*v = nil
			} else {
				t := row[i].(time.Time)
				*v = &t
			}
		case **string:
			if row[i] == nil {
				*v = nil
			} else {
				s := row[i].(string)
				*v = &s
			}
		}
	}
	return nil
}

type mockReportQuerier struct {
	rowFn  func() pgx.Row
	rowsFn func() (pgx.Rows, error)
	execFn func() (pgconn.CommandTag, error)
}

func (m *mockReportQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return m.rowFn()
}

func (m *mockReportQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return m.rowsFn()
}

func (m *mockReportQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return m.execFn()
}

// fillReport fills a Report via Scan dest pointers in reportColumns order.
func fillReport(r *Report) func(dest ...any) error {
	return func(dest ...any) error {
		*dest[0].(*string) = r.ID
		*dest[1].(**string) = r.ReporterID
		*dest[2].(**string) = r.ReportedUserID
		*dest[3].(**string) = r.MessageID
		*dest[4].(*string) = r.Reason
		*dest[5].(*string) = r.Priority
		*dest[6].(*string) = r.Description
		*dest[7].(*string) = r.Status
		*dest[8].(*time.Time) = r.CreatedAt
		*dest[9].(**time.Time) = r.ReviewedAt
		*dest[10].(**string) = r.ReviewedBy
		return nil
	}
}

// deref turns an optional ID into the mock row value (nil stays nil).
func deref(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// --- Store.Create ---

func TestStore_Create_Success(t *testing.T) {
	now := time.Now()
	report := &Report{
		ID:             "r-1",
		ReporterID:     new("u-1"),
		ReportedUserID: new("u-2"),
		Reason:         "harassment",
		Description:    "Test description",
		Status:         "pending",
		CreatedAt:      now,
	}
	q := &mockReportQuerier{
		rowFn: func() pgx.Row {
			return &mockReportRow{scanFn: fillReport(report)}
		},
	}
	s := &pgStore{db: q}

	got, err := s.Create(context.Background(), "u-1", "u-2", "harassment", PriorityNormal, "Test description")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got.ID != "r-1" {
		t.Errorf("ID = %q, want r-1", got.ID)
	}
	if got.Status != "pending" {
		t.Errorf("Status = %q, want pending", got.Status)
	}
}

func TestStore_Create_WithReviewedFields(t *testing.T) {
	now := time.Now()
	reviewedBy := "admin-1"
	report := &Report{
		ID:             "r-1",
		ReporterID:     new("u-1"),
		ReportedUserID: new("u-2"),
		Reason:         "spam",
		Status:         "reviewed",
		CreatedAt:      now,
		ReviewedAt:     &now,
		ReviewedBy:     &reviewedBy,
	}
	q := &mockReportQuerier{
		rowFn: func() pgx.Row {
			return &mockReportRow{scanFn: fillReport(report)}
		},
	}
	s := &pgStore{db: q}

	got, err := s.Create(context.Background(), "u-1", "u-2", "spam", PriorityNormal, "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got.ReviewedAt == nil {
		t.Error("ReviewedAt should not be nil")
	}
	if got.ReviewedBy == nil || *got.ReviewedBy != "admin-1" {
		t.Errorf("ReviewedBy = %v, want admin-1", got.ReviewedBy)
	}
}

func TestStore_Create_StoreError(t *testing.T) {
	q := &mockReportQuerier{
		rowFn: func() pgx.Row {
			return &mockReportRow{scanFn: func(dest ...any) error {
				return errors.New("db error")
			}}
		},
	}
	s := &pgStore{db: q}

	_, err := s.Create(context.Background(), "u-1", "u-2", "spam", PriorityNormal, "")
	if err == nil {
		t.Fatal("Create() expected error")
	}
}

// --- Store.GetByID ---

func TestStore_GetByID_Success(t *testing.T) {
	now := time.Now()
	report := &Report{
		ID:             "r-1",
		ReporterID:     new("u-1"),
		ReportedUserID: new("u-2"),
		Reason:         "spam",
		Status:         "pending",
		CreatedAt:      now,
	}
	q := &mockReportQuerier{
		rowFn: func() pgx.Row {
			return &mockReportRow{scanFn: fillReport(report)}
		},
	}
	s := &pgStore{db: q}

	got, err := s.GetByID(context.Background(), "r-1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.ID != "r-1" {
		t.Errorf("ID = %q, want r-1", got.ID)
	}
}

func TestStore_GetByID_NotFound(t *testing.T) {
	q := &mockReportQuerier{
		rowFn: func() pgx.Row {
			return &mockReportRow{scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	s := &pgStore{db: q}

	_, err := s.GetByID(context.Background(), "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestStore_GetByID_StoreError(t *testing.T) {
	q := &mockReportQuerier{
		rowFn: func() pgx.Row {
			return &mockReportRow{scanFn: func(dest ...any) error {
				return errors.New("db error")
			}}
		},
	}
	s := &pgStore{db: q}

	_, err := s.GetByID(context.Background(), "r-1")
	if err == nil {
		t.Fatal("GetByID() expected error")
	}
}

// --- Store.List ---

func TestStore_List_All(t *testing.T) {
	now := time.Now()
	reports := []ReportWithUserInfo{
		{ID: "r-1", ReporterID: new("u-1"), ReportedUserID: new("u-2"), ReportedEmail: "user2@test.com", ReportedName: "User 2", ReportedAvatar: "", Reason: "spam", Status: "pending", CreatedAt: now},
		{ID: "r-2", ReporterID: new("u-2"), ReportedUserID: new("u-3"), ReportedEmail: "user3@test.com", ReportedName: "User 3", ReportedAvatar: "", Reason: "harassment", Status: "reviewed", CreatedAt: now},
	}
	rows := &mockReportRows{
		data: make([][]any, len(reports)),
	}
	for i, r := range reports {
		rows.data[i] = []any{r.ID, deref(r.ReporterID), deref(r.ReportedUserID), r.ReportedEmail, r.ReportedName, r.ReportedUsername, r.ReportedAvatar, r.ReportedAccountGone, nil, nil, nil, nil, nil, nil, r.Reason, r.Priority, r.Description, r.Status, r.CreatedAt, nil, nil}
	}

	q := &mockReportQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return rows, nil
		},
	}
	s := &pgStore{db: q}

	got, err := s.List(context.Background(), ListFilter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestStore_List_WithStatusFilter(t *testing.T) {
	now := time.Now()
	reports := []ReportWithUserInfo{
		{ID: "r-1", ReporterID: new("u-1"), ReportedUserID: new("u-2"), ReportedEmail: "user2@test.com", ReportedName: "User 2", ReportedAvatar: "", Reason: "spam", Status: "pending", CreatedAt: now},
	}
	rows := &mockReportRows{data: make([][]any, len(reports))}
	for i, r := range reports {
		rows.data[i] = []any{r.ID, deref(r.ReporterID), deref(r.ReportedUserID), r.ReportedEmail, r.ReportedName, r.ReportedUsername, r.ReportedAvatar, r.ReportedAccountGone, nil, nil, nil, nil, nil, nil, r.Reason, r.Priority, r.Description, r.Status, r.CreatedAt, nil, nil}
	}

	q := &mockReportQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return rows, nil
		},
	}
	s := &pgStore{db: q}

	got, err := s.List(context.Background(), ListFilter{Status: "pending"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}

func TestStore_List_Empty(t *testing.T) {
	rows := &mockReportRows{data: nil}

	q := &mockReportQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return rows, nil
		},
	}
	s := &pgStore{db: q}

	got, err := s.List(context.Background(), ListFilter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestStore_List_StoreError(t *testing.T) {
	q := &mockReportQuerier{
		rowsFn: func() (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}
	s := &pgStore{db: q}

	_, err := s.List(context.Background(), ListFilter{})
	if err == nil {
		t.Fatal("List() expected error")
	}
}

// --- Store.UpdateStatus ---

func TestStore_UpdateStatus_Success(t *testing.T) {
	now := time.Now()
	reviewedAt := now
	reviewedBy := "admin-1"
	report := &Report{
		ID:             "r-1",
		ReporterID:     new("u-1"),
		ReportedUserID: new("u-2"),
		Reason:         "harassment",
		Status:         "reviewed",
		ReviewedAt:     &reviewedAt,
		ReviewedBy:     &reviewedBy,
		CreatedAt:      now,
	}
	q := &mockReportQuerier{
		rowFn: func() pgx.Row {
			return &mockReportRow{scanFn: fillReport(report)}
		},
	}
	s := &pgStore{db: q}

	got, err := s.UpdateStatus(context.Background(), "r-1", "reviewed", "admin-1")
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if got.Status != "reviewed" {
		t.Errorf("Status = %q, want reviewed", got.Status)
	}
}

func TestStore_UpdateStatus_NotFound(t *testing.T) {
	q := &mockReportQuerier{
		rowFn: func() pgx.Row {
			return &mockReportRow{scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	s := &pgStore{db: q}

	_, err := s.UpdateStatus(context.Background(), "nonexistent", "reviewed", "admin-1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestStore_UpdateStatus_StoreError(t *testing.T) {
	q := &mockReportQuerier{
		rowFn: func() pgx.Row {
			return &mockReportRow{scanFn: func(dest ...any) error {
				return errors.New("db error")
			}}
		},
	}
	s := &pgStore{db: q}

	_, err := s.UpdateStatus(context.Background(), "r-1", "reviewed", "admin-1")
	if err == nil {
		t.Fatal("UpdateStatus() expected error")
	}
}

// --- Evidence snapshots ---

func TestClassifyCreateErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"no row", pgx.ErrNoRows, ErrMessageNotFound},
		{"self report", &pgconn.PgError{Code: "23514", ConstraintName: "no_self_report"}, ErrSelfReport},
		{"duplicate", &pgconn.PgError{Code: "23505", ConstraintName: "idx_reports_reporter_message"}, ErrDuplicateReport},
		{"malformed uuid", &pgconn.PgError{Code: "22P02"}, ErrMessageNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyCreateErr(tc.err, ErrMessageNotFound); !errors.Is(got, tc.want) {
				t.Errorf("classifyCreateErr(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
	other := errors.New("boom")
	if got := classifyCreateErr(other, ErrMessageNotFound); !errors.Is(got, other) || errors.Is(got, ErrMessageNotFound) {
		t.Errorf("unexpected classification for a generic error: %v", got)
	}
}

func TestStore_CreateForMessage_Success(t *testing.T) {
	report := &Report{ID: "r-1", ReporterID: new("u-1"), ReportedUserID: new("u-2"), MessageID: new("m-1"), Status: "pending"}
	s := &pgStore{db: &mockReportQuerier{rowFn: func() pgx.Row { return &mockReportRow{scanFn: fillReport(report)} }}}

	got, err := s.CreateForMessage(t.Context(), "u-1", "m-1", ReasonSpam, PriorityNormal, "")
	if err != nil {
		t.Fatalf("CreateForMessage() error = %v", err)
	}
	if got.MessageID == nil || *got.MessageID != "m-1" || *got.ReportedUserID != "u-2" {
		t.Errorf("got %+v, want message m-1 reported against u-2", got)
	}
}

func TestStore_CreateForMessage_NotVisible(t *testing.T) {
	s := &pgStore{db: &mockReportQuerier{rowFn: func() pgx.Row {
		return &mockReportRow{scanFn: func(...any) error { return pgx.ErrNoRows }}
	}}}
	if _, err := s.CreateForMessage(t.Context(), "u-1", "m-1", ReasonSpam, PriorityNormal, ""); !errors.Is(err, ErrMessageNotFound) {
		t.Errorf("error = %v, want ErrMessageNotFound", err)
	}
}

func TestStore_Create_UnknownUser(t *testing.T) {
	s := &pgStore{db: &mockReportQuerier{rowFn: func() pgx.Row {
		return &mockReportRow{scanFn: func(...any) error { return pgx.ErrNoRows }}
	}}}
	if _, err := s.Create(t.Context(), "u-1", "u-404", ReasonSpam, PriorityNormal, ""); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("error = %v, want ErrUserNotFound", err)
	}
}

func TestStore_List_MapsMessageSnapshotAndGoneAccount(t *testing.T) {
	sent := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	purged := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	rows := &mockReportRows{data: [][]any{
		{"r-1", "u-1", nil, "bea@example.com", "Bea", "bea", "", true,
			"m-1", "room-1", "text", nil, sent, purged,
			"harassment", "normal", "", "dismissed", sent, nil, nil},
		{"r-2", "u-1", "u-3", "cam@example.com", "Cam", "cam", "", false,
			nil, nil, nil, nil, nil, nil,
			"spam", "normal", "", "pending", sent, nil, nil},
	}}
	s := &pgStore{db: &mockReportQuerier{rowsFn: func() (pgx.Rows, error) { return rows, nil }}}

	got, err := s.List(t.Context(), ListFilter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	first := got[0]
	if !first.ReportedAccountGone || first.ReportedUserID != nil || first.ReportedUsername != "bea" {
		t.Errorf("gone account not mapped from snapshot: %+v", first)
	}
	if m := first.Message; m == nil || m.ID != "m-1" || m.RoomID != "room-1" || m.Content != nil || m.PurgedAt == nil || !m.SentAt.Equal(sent) {
		t.Errorf("message snapshot = %+v, want purged copy of m-1", first.Message)
	}
	if got[1].Message != nil {
		t.Errorf("user-level report has message %+v, want nil", got[1].Message)
	}
}
