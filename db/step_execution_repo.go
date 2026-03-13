package db

import (
	"context"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/go-lib/db"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

var _ domain.StepExecutionRepository = (*StepExecutionRepository)(nil)

// ErrStepExecutionNotFound is returned when a step execution is not found
var ErrStepExecutionNotFound = errors.New("step execution not found")

// StepExecutionRepository implements domain.StepExecutionRepository using SQL
type StepExecutionRepository struct {
	db     db.Database
	ph     sq.PlaceholderFormat
	tracer trace.Tracer
	logger logrus.FieldLogger
}

// NewStepExecutionRepository creates a new SQL-backed step execution repository
func NewStepExecutionRepository(database db.Database, opts ...Option) *StepExecutionRepository {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &StepExecutionRepository{
		db:     database,
		ph:     getPlaceholderFormat(),
		tracer: o.t,
		logger: o.l,
	}
}

type stepExecutionRow struct {
	ID          string     `db:"id"`
	ExecutionID string     `db:"execution_id"`
	StepName    string     `db:"step_name"`
	Status      string     `db:"status"`
	StartedAt   *time.Time `db:"started_at"`
	FinishedAt  *time.Time `db:"finished_at"`
	ExitCode    *int       `db:"exit_code"`
	Error       string     `db:"error"`
}

func (r *StepExecutionRepository) Create(ctx context.Context, stepExecution *domain.StepExecution) error {
	ctx, span := r.tracer.Start(ctx, "StepExecutionRepository.Create")
	defer span.End()

	stepExecution.ID = uuid.New()

	query := sq.Insert("step_executions").
		Columns("id", "execution_id", "step_name", "status", "started_at", "finished_at", "exit_code", "error").
		Values(
			stepExecution.ID.String(),
			stepExecution.ExecutionID.String(),
			stepExecution.StepName,
			string(stepExecution.Status),
			stepExecution.StartedAt,
			stepExecution.FinishedAt,
			stepExecution.ExitCode,
			stepExecution.Error,
		).
		PlaceholderFormat(r.ph)

	querySQL, args, err := query.ToSql()
	if err != nil {
		r.logger.WithError(err).Error("failed to build insert query")
		return err
	}

	_, err = r.db.Exec(ctx, querySQL, args...)
	if err != nil {
		r.logger.WithError(err).Error("failed to insert step execution")
		return err
	}

	return nil
}

func (r *StepExecutionRepository) ListByExecution(
	ctx context.Context,
	executionID string,
) ([]*domain.StepExecution, error) {
	ctx, span := r.tracer.Start(ctx, "StepExecutionRepository.ListByExecution")
	defer span.End()

	parsedExecutionID, err := uuid.Parse(executionID)
	if err != nil {
		r.logger.WithError(err).WithField("execution_id", executionID).Error("invalid execution ID")
		return nil, err
	}

	query := sq.Select("id", "execution_id", "step_name", "status", "started_at", "finished_at", "exit_code", "error").
		From("step_executions").
		Where(sq.Eq{"execution_id": parsedExecutionID.String()}).
		OrderBy("created_at ASC").
		PlaceholderFormat(r.ph)

	querySQL, args, err := query.ToSql()
	if err != nil {
		r.logger.WithError(err).Error("failed to build list query")
		return nil, err
	}

	var rows []stepExecutionRow
	err = r.db.Select(ctx, &rows, querySQL, args...)
	if err != nil {
		r.logger.WithError(err).Error("failed to list step executions")
		return nil, err
	}

	stepExecutions := make([]*domain.StepExecution, 0, len(rows))
	for i := range rows {
		stepExecution, err := rowToStepExecution(&rows[i])
		if err != nil {
			r.logger.WithError(err).WithField("row", rows[i]).Error("failed to convert row to step execution")
			return nil, err
		}
		stepExecutions = append(stepExecutions, stepExecution)
	}

	return stepExecutions, nil
}

func (r *StepExecutionRepository) UpdateStatus(
	ctx context.Context,
	id string,
	status domain.StepExecutionStatus,
	exitCode *int,
) error {
	ctx, span := r.tracer.Start(ctx, "StepExecutionRepository.UpdateStatus")
	defer span.End()

	parsedID, err := uuid.Parse(id)
	if err != nil {
		r.logger.WithError(err).WithField("id", id).Error("invalid step execution ID")
		return err
	}

	query := sq.Update("step_executions").
		Set("status", string(status)).
		Where(sq.Eq{"id": parsedID.String()}).
		PlaceholderFormat(r.ph)

	if exitCode != nil {
		query = query.Set("exit_code", *exitCode)
	}

	querySQL, args, err := query.ToSql()
	if err != nil {
		r.logger.WithError(err).Error("failed to build update query")
		return err
	}

	result, err := r.db.Exec(ctx, querySQL, args...)
	if err != nil {
		r.logger.WithError(err).WithField("id", id).Error("failed to update step execution status")
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.logger.WithError(err).Error("failed to get rows affected")
		return err
	}

	if rowsAffected == 0 {
		return ErrStepExecutionNotFound
	}

	return nil
}

func rowToStepExecution(row *stepExecutionRow) (*domain.StepExecution, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return nil, err
	}

	executionID, err := uuid.Parse(row.ExecutionID)
	if err != nil {
		return nil, err
	}

	return &domain.StepExecution{
		ID:          id,
		ExecutionID: executionID,
		StepName:    row.StepName,
		Status:      domain.StepExecutionStatus(row.Status),
		StartedAt:   row.StartedAt,
		FinishedAt:  row.FinishedAt,
		ExitCode:    row.ExitCode,
		Error:       row.Error,
	}, nil
}
