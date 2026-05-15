package reports

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
func (r *mockReportRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.data[r.pos-1]
	for i, d := range dest {
		switch v := d.(type) {
		case *string:
			*v = row[i].(string)
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

// scanReport is a helper that fills a Report via Scan dest pointers.
func scanReport(r *Report) func(dest ...any) error {
	return func(dest ...any) error {
		*dest[0].(*string) = r.ID
		*dest[1].(*string) = r.ReporterID
		*dest[2].(*string) = r.ReportedUserID
		*dest[3].(*string) = r.Reason
		*dest[4].(*string) = r.Priority
		*dest[5].(*string) = r.Description
		*dest[6].(*string) = r.Status
		*dest[7].(*time.Time) = r.CreatedAt
		*dest[8].(**time.Time) = r.ReviewedAt
		*dest[9].(**string) = r.ReviewedBy
		return nil
	}
}

// --- Store.Create ---

func TestStore_Create_Success(t *testing.T) {
	now := time.Now()
	report := &Report{
		ID:             "r-1",
		ReporterID:     "u-1",
		ReportedUserID: "u-2",
		Reason:         "harassment",
		Description:    "Test description",
		Status:         "pending",
		CreatedAt:      now,
	}
	q := &mockReportQuerier{
		rowFn: func() pgx.Row {
			return &mockReportRow{scanFn: scanReport(report)}
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
		ReporterID:     "u-1",
		ReportedUserID: "u-2",
		Reason:         "spam",
		Status:         "reviewed",
		CreatedAt:      now,
		ReviewedAt:     &now,
		ReviewedBy:     &reviewedBy,
	}
	q := &mockReportQuerier{
		rowFn: func() pgx.Row {
			return &mockReportRow{scanFn: scanReport(report)}
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
		ReporterID:     "u-1",
		ReportedUserID: "u-2",
		Reason:         "spam",
		Status:         "pending",
		CreatedAt:      now,
	}
	q := &mockReportQuerier{
		rowFn: func() pgx.Row {
			return &mockReportRow{scanFn: scanReport(report)}
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
		{ID: "r-1", ReporterID: "u-1", ReportedUserID: "u-2", ReportedEmail: "user2@test.com", ReportedName: "User 2", ReportedAvatar: "", Reason: "spam", Status: "pending", CreatedAt: now},
		{ID: "r-2", ReporterID: "u-2", ReportedUserID: "u-3", ReportedEmail: "user3@test.com", ReportedName: "User 3", ReportedAvatar: "", Reason: "harassment", Status: "reviewed", CreatedAt: now},
	}
	rows := &mockReportRows{
		data: make([][]any, len(reports)),
	}
	for i, r := range reports {
		rows.data[i] = []any{r.ID, r.ReporterID, r.ReportedUserID, r.ReportedEmail, r.ReportedName, r.ReportedAvatar, r.Reason, r.Priority, r.Description, r.Status, r.CreatedAt, nil, nil}
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
		{ID: "r-1", ReporterID: "u-1", ReportedUserID: "u-2", ReportedEmail: "user2@test.com", ReportedName: "User 2", ReportedAvatar: "", Reason: "spam", Status: "pending", CreatedAt: now},
	}
	rows := &mockReportRows{data: make([][]any, len(reports))}
	for i, r := range reports {
		rows.data[i] = []any{r.ID, r.ReporterID, r.ReportedUserID, r.ReportedEmail, r.ReportedName, r.ReportedAvatar, r.Reason, r.Priority, r.Description, r.Status, r.CreatedAt, nil, nil}
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
		ReporterID:     "u-1",
		ReportedUserID: "u-2",
		Reason:         "harassment",
		Status:         "reviewed",
		ReviewedAt:     &reviewedAt,
		ReviewedBy:     &reviewedBy,
		CreatedAt:      now,
	}
	q := &mockReportQuerier{
		rowFn: func() pgx.Row {
			return &mockReportRow{scanFn: scanReport(report)}
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
