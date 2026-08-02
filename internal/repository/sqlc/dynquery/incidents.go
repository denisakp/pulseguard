package dynquery

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	dtoV1 "github.com/denisakp/ogoune/internal/dto/v1"
)

// IncidentFilter holds parsed ?status=, ?monitor_id=, ?from=, ?to=, ?q= filters
// for the v1 /incidents list endpoint. Pointer fields: nil means no filter.
type IncidentFilter struct {
	Status    *string
	MonitorID *string
	From      *time.Time
	To        *time.Time
	Q         *string // free-text search over the incident cause (spec 084)
}

// Validate returns field-level errors. Cross-field: if both From and To are
// set, From must be <= To.
func (f *IncidentFilter) Validate() []dtoV1.FieldError {
	var errs []dtoV1.FieldError
	if f.Status != nil && !isValidIncidentStatus(*f.Status) {
		errs = append(errs, dtoV1.FieldError{Field: "status", Message: "must be 'open' or 'resolved'"})
	}
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		errs = append(errs, dtoV1.FieldError{Field: "from", Message: "must be before or equal to 'to'"})
	}
	if f.Q != nil && len(*f.Q) > maxQLength {
		errs = append(errs, dtoV1.FieldError{Field: "q", Message: "query too long"})
	}
	return errs
}

// BuildIncidentsQuery builds the rows SELECT for the v1 incidents list
// endpoint. Column names and keywords are hardcoded; user-derived values are
// parameterised via squirrel.
func BuildIncidentsQuery(f IncidentFilter, page, perPage int, ph sq.PlaceholderFormat) (string, []any, error) {
	preds := incidentPredicates(f)
	b := sq.Select(
		"id", "resource_id", "cause", "resolved_at", "started_at",
		"details", "created_at", "updated_at",
	).
		From("incidents").
		Where(sq.And(preds)).
		OrderBy("created_at DESC").
		Limit(uint64(perPage)).
		Offset(uint64((page - 1) * perPage)).
		PlaceholderFormat(ph)
	return b.ToSql()
}

// BuildIncidentCountQuery builds the COUNT(*) for the same filter.
func BuildIncidentCountQuery(f IncidentFilter, ph sq.PlaceholderFormat) (string, []any, error) {
	preds := incidentPredicates(f)
	b := sq.Select("COUNT(*)").
		From("incidents").
		Where(sq.And(preds)).
		PlaceholderFormat(ph)
	return b.ToSql()
}

func incidentPredicates(f IncidentFilter) sq.And {
	preds := sq.And{}
	if f.Status != nil {
		switch *f.Status {
		case "open":
			preds = append(preds, sq.Eq{"resolved_at": nil})
		case "resolved":
			preds = append(preds, sq.NotEq{"resolved_at": nil})
		}
	}
	if f.MonitorID != nil {
		preds = append(preds, sq.Eq{"resource_id": *f.MonitorID})
	}
	if f.From != nil {
		preds = append(preds, sq.GtOrEq{"started_at": *f.From})
	}
	if f.To != nil {
		preds = append(preds, sq.LtOrEq{"started_at": *f.To})
	}
	if f.Q != nil {
		like := "%" + likeEscape(*f.Q) + "%"
		preds = append(preds, sq.Expr(`LOWER(cause) LIKE LOWER(?) ESCAPE '\'`, like))
	}
	if len(preds) == 0 {
		// squirrel sq.And{} renders as `()` which is invalid; pass a tautology.
		preds = append(preds, sq.Expr("1=1"))
	}
	return preds
}
