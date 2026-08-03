package dynquery

import (
	"strings"
	"testing"
	"time"

	sq "github.com/Masterminds/squirrel"
)

func TestBuildIncidentsQuery_NoFilter(t *testing.T) {
	rowsSQL, rowsArgs, countSQL, countArgs, _ := buildBoth(IncidentFilter{}, 1, 25, sq.Dollar)
	if !strings.Contains(rowsSQL, "FROM incidents") {
		t.Errorf("missing FROM incidents: %s", rowsSQL)
	}
	if !strings.Contains(rowsSQL, "ORDER BY created_at DESC") {
		t.Errorf("missing ORDER BY")
	}
	if !strings.Contains(rowsSQL, "LIMIT 25") {
		t.Errorf("missing LIMIT")
	}
	if !strings.Contains(countSQL, "COUNT(*)") {
		t.Errorf("missing COUNT")
	}
	if len(rowsArgs) != 0 {
		t.Errorf("expected no args for empty filter, got %v", rowsArgs)
	}
	if len(countArgs) != 0 {
		t.Errorf("expected no count args, got %v", countArgs)
	}
}

func TestBuildIncidentsQuery_StatusOpen(t *testing.T) {
	rowsSQL, _, _, _, _ := buildBoth(IncidentFilter{Status: strPtr("open")}, 1, 25, sq.Dollar)
	if !strings.Contains(rowsSQL, "resolved_at") || !strings.Contains(rowsSQL, "IS NULL") {
		t.Errorf("expected resolved_at IS NULL clause, got: %s", rowsSQL)
	}
}

func TestBuildIncidentsQuery_StatusResolved(t *testing.T) {
	rowsSQL, _, _, _, _ := buildBoth(IncidentFilter{Status: strPtr("resolved")}, 1, 25, sq.Dollar)
	if !strings.Contains(rowsSQL, "resolved_at") || !strings.Contains(rowsSQL, "IS NOT NULL") {
		t.Errorf("expected resolved_at IS NOT NULL clause, got: %s", rowsSQL)
	}
}

func TestBuildIncidentsQuery_MonitorID(t *testing.T) {
	rowsSQL, rowsArgs, _, _, _ := buildBoth(IncidentFilter{MonitorID: strPtr("mon-123")}, 1, 25, sq.Dollar)
	if !strings.Contains(rowsSQL, "resource_id") {
		t.Errorf("expected resource_id clause")
	}
	if len(rowsArgs) != 1 || rowsArgs[0] != "mon-123" {
		t.Errorf("expected single mon-123 arg, got %v", rowsArgs)
	}
}

func TestBuildIncidentsQuery_FromTo(t *testing.T) {
	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 31, 23, 59, 59, 0, time.UTC)
	rowsSQL, rowsArgs, _, _, _ := buildBoth(IncidentFilter{From: &from, To: &to}, 1, 25, sq.Dollar)
	if !strings.Contains(rowsSQL, ">=") || !strings.Contains(rowsSQL, "<=") {
		t.Errorf("expected range operators: %s", rowsSQL)
	}
	if len(rowsArgs) != 2 {
		t.Errorf("expected 2 args, got %v", rowsArgs)
	}
}

func TestBuildIncidentsQuery_Q(t *testing.T) {
	rowsSQL, rowsArgs, _, _, _ := buildBoth(IncidentFilter{Q: strPtr("timeout")}, 1, 25, sq.Dollar)
	if !strings.Contains(rowsSQL, "LOWER(cause) LIKE LOWER(") || !strings.Contains(rowsSQL, "ESCAPE") {
		t.Errorf("expected LOWER(cause) LIKE … ESCAPE clause, got: %s", rowsSQL)
	}
	if len(rowsArgs) != 1 || rowsArgs[0] != "%timeout%" {
		t.Errorf("expected single %%timeout%% arg, got %v", rowsArgs)
	}
}

func TestBuildIncidentsQuery_Q_EscapesMetachars(t *testing.T) {
	_, rowsArgs, _, _, _ := buildBoth(IncidentFilter{Q: strPtr("50%_x")}, 1, 25, sq.Dollar)
	if len(rowsArgs) != 1 || rowsArgs[0] != `%50\%\_x%` {
		t.Errorf("expected metacharacters escaped, got %v", rowsArgs)
	}
}

func TestBuildIncidentsQuery_AllFilters(t *testing.T) {
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()
	f := IncidentFilter{
		Status:    strPtr("open"),
		MonitorID: strPtr("mon-x"),
		From:      &from,
		To:        &to,
	}
	rowsSQL, _, countSQL, _, err := buildBoth(f, 2, 10, sq.Dollar)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(rowsSQL, "LIMIT 10") || !strings.Contains(rowsSQL, "OFFSET 10") {
		t.Errorf("bad pagination: %s", rowsSQL)
	}
	if !strings.Contains(countSQL, "COUNT(*)") {
		t.Errorf("count SQL missing COUNT(*)")
	}
	if strings.Contains(rowsSQL, "'mon-x'") {
		t.Errorf("monitor ID inlined in SQL")
	}
}

func TestBuildIncidentsQuery_PlaceholderDialects(t *testing.T) {
	f := IncidentFilter{Status: strPtr("open"), MonitorID: strPtr("m1")}
	pg, _, _, _, _ := buildBoth(f, 1, 25, sq.Dollar)
	lite, _, _, _, _ := buildBoth(f, 1, 25, sq.Question)
	if !strings.Contains(pg, "$1") {
		t.Errorf("expected $N for PG, got %s", pg)
	}
	if strings.Contains(lite, "$1") || !strings.Contains(lite, "?") {
		t.Errorf("expected ? for SQLite, got %s", lite)
	}
}

func TestBuildIncidentsQuery_NeverInlinesValues(t *testing.T) {
	mid := "'; DROP TABLE incidents; --"
	rowsSQL, rowsArgs, _, _, _ := buildBoth(IncidentFilter{MonitorID: &mid}, 1, 25, sq.Dollar)
	if strings.Contains(rowsSQL, "DROP TABLE") {
		t.Errorf("injection leak: %s", rowsSQL)
	}
	if !containsAll(rowsArgs, mid) {
		t.Errorf("payload missing from args: %v", rowsArgs)
	}
}

func TestIncidentFilter_Validate(t *testing.T) {
	earlier := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		f        IncidentFilter
		wantErrs int
	}{
		{"empty ok", IncidentFilter{}, 0},
		{"valid status open", IncidentFilter{Status: strPtr("open")}, 0},
		{"valid status resolved", IncidentFilter{Status: strPtr("resolved")}, 0},
		{"bad status", IncidentFilter{Status: strPtr("bogus")}, 1},
		{"from < to ok", IncidentFilter{From: &earlier, To: &later}, 0},
		{"from > to bad", IncidentFilter{From: &later, To: &earlier}, 1},
		{"from == to ok", IncidentFilter{From: &earlier, To: &earlier}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs := c.f.Validate()
			if len(errs) != c.wantErrs {
				t.Errorf("want %d errs, got %d: %v", c.wantErrs, len(errs), errs)
			}
		})
	}
}

func buildBoth(f IncidentFilter, page, perPage int, ph sq.PlaceholderFormat) (string, []any, string, []any, error) {
	rowsSQL, rowsArgs, err := BuildIncidentsQuery(f, page, perPage, ph)
	if err != nil {
		return "", nil, "", nil, err
	}
	countSQL, countArgs, err := BuildIncidentCountQuery(f, ph)
	if err != nil {
		return rowsSQL, rowsArgs, "", nil, err
	}
	return rowsSQL, rowsArgs, countSQL, countArgs, nil
}
