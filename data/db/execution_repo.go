package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/go-lib/db"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

var _ domain.ExecutionRepository = (*ExecutionRepository)(nil)

// ErrExecutionNotFound is returned when an execution is not found
var ErrExecutionNotFound = errors.New("execution not found")

// ExecutionRepository implements domain.ExecutionRepository using SQL
type ExecutionRepository struct {
	db     db.Database
	tracer trace.Tracer
	logger logrus.FieldLogger
}

// NewExecutionRepository creates a new SQL-backed execution repository
func NewExecutionRepository(database db.Database, opts ...Option) *ExecutionRepository {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &ExecutionRepository{
		db:     database,
		tracer: o.t,
		logger: o.l,
	}
}

type executionRow struct {
	ID         string     `db:"id"`
	WorkflowID string     `db:"workflow_id"`
	Status     string     `db:"status"`
	StartedAt  *time.Time `db:"started_at"`
	FinishedAt *time.Time `db:"finished_at"`
	Error      string     `db:"error"`
}

func (r *ExecutionRepository) Create(ctx context.Context, execution *domain.Execution) error {
	ctx, span := r.tracer.Start(ctx, "ExecutionRepository.Create")
	defer span.End()

	execution.ID = uuid.New()

	query := sq.Insert("executions").
		Columns("id", "workflow_id", "status", "started_at", "finished_at", "error").
		Values(
			execution.ID.String(),
			execution.WorkflowID.String(),
			string(execution.Status),
			execution.StartedAt,
			execution.FinishedAt,
			execution.Error,
		).
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		r.logger.WithError(err).Error("failed to build insert query")
		return err
	}

	_, err = r.db.Exec(ctx, querySQL, args...)
	if err != nil {
		r.logger.WithError(err).Error("failed to insert execution")
		return err
	}

	return nil
}

func (r *ExecutionRepository) Get(ctx context.Context, id string) (*domain.Execution, error) {
	ctx, span := r.tracer.Start(ctx, "ExecutionRepository.Get")
	defer span.End()

	parsedID, err := uuid.Parse(id)
	if err != nil {
		r.logger.WithError(err).WithField("id", id).Error("invalid execution ID")
		return nil, err
	}

	query := sq.Select("id", "workflow_id", "status", "started_at", "finished_at", "error").
		From("executions").
		Where(sq.Eq{"id": parsedID.String()}).
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		r.logger.WithError(err).Error("failed to build select query")
		return nil, err
	}

	var row executionRow
	err = r.db.Get(ctx, &row, querySQL, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrExecutionNotFound
		}
		r.logger.WithError(err).WithField("id", id).Error("failed to get execution")
		return nil, err
	}

	return rowToExecution(&row)
}

func (r *ExecutionRepository) ListByWorkflow(
	ctx context.Context,
	workflowID string,
	opts domain.ListOptions,
) ([]*domain.Execution, error) {
	ctx, span := r.tracer.Start(ctx, "ExecutionRepository.ListByWorkflow")
	defer span.End()

	parsedWorkflowID, err := uuid.Parse(workflowID)
	if err != nil {
		r.logger.WithError(err).WithField("workflow_id", workflowID).Error("invalid workflow ID")
		return nil, err
	}

	query := sq.Select("id", "workflow_id", "status", "started_at", "finished_at", "error").
		From("executions").
		Where(sq.Eq{"workflow_id": parsedWorkflowID.String()}).
		PlaceholderFormat(sq.Dollar)

	if opts.OrderBy != "" {
		query = query.OrderBy(opts.OrderBy)
	} else {
		query = query.OrderBy("created_at DESC")
	}

	if opts.Limit > 0 {
		query = query.Limit(uint64(opts.Limit))
	}

	if opts.Offset > 0 {
		query = query.Offset(uint64(opts.Offset))
	}

	querySQL, args, err := query.ToSql()
	if err != nil {
		r.logger.WithError(err).Error("failed to build list query")
		return nil, err
	}

	var rows []executionRow
	err = r.db.Select(ctx, &rows, querySQL, args...)
	if err != nil {
		r.logger.WithError(err).Error("failed to list executions")
		return nil, err
	}

	executions := make([]*domain.Execution, 0, len(rows))
	for i := range rows {
		execution, err := rowToExecution(&rows[i])
		if err != nil {
			r.logger.WithError(err).WithField("row", rows[i]).Error("failed to convert row to execution")
			return nil, err
		}
		executions = append(executions, execution)
	}

	return executions, nil
}

func (r *ExecutionRepository) UpdateStatus(ctx context.Context, id string, status domain.ExecutionStatus) error {
	ctx, span := r.tracer.Start(ctx, "ExecutionRepository.UpdateStatus")
	defer span.End()

	parsedID, err := uuid.Parse(id)
	if err != nil {
		r.logger.WithError(err).WithField("id", id).Error("invalid execution ID")
		return err
	}

	query := sq.Update("executions").
		Set("status", string(status)).
		Where(sq.Eq{"id": parsedID.String()}).
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		r.logger.WithError(err).Error("failed to build update query")
		return err
	}

	result, err := r.db.Exec(ctx, querySQL, args...)
	if err != nil {
		r.logger.WithError(err).WithField("id", id).Error("failed to update execution status")
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.logger.WithError(err).Error("failed to get rows affected")
		return err
	}

	if rowsAffected == 0 {
		return ErrExecutionNotFound
	}

	return nil
}

func rowToExecution(row *executionRow) (*domain.Execution, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return nil, err
	}

	workflowID, err := uuid.Parse(row.WorkflowID)
	if err != nil {
		return nil, err
	}

	return &domain.Execution{
		ID:         id,
		WorkflowID: workflowID,
		Status:     domain.ExecutionStatus(row.Status),
		StartedAt:  row.StartedAt,
		FinishedAt: row.FinishedAt,
		Error:      row.Error,
	}, nil
}
