package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/go-lib/db"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

var _ domain.WorkflowRepository = (*WorkflowRepository)(nil)

// ErrWorkflowNotFound is returned when a workflow is not found
var ErrWorkflowNotFound = errors.New("workflow not found")

// WorkflowRepository implements domain.WorkflowRepository using SQL
type WorkflowRepository struct {
	db     db.Database
	tracer trace.Tracer
	logger logrus.FieldLogger
}

// NewWorkflowRepository creates a new SQL-backed workflow repository
func NewWorkflowRepository(database db.Database, opts ...Option) *WorkflowRepository {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &WorkflowRepository{
		db:     database,
		tracer: o.t,
		logger: o.l,
	}
}

type workflowRow struct {
	ID          string    `db:"id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	Steps       string    `db:"steps"`
	Status      string    `db:"status"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (r *WorkflowRepository) Create(ctx context.Context, workflow *domain.Workflow) error {
	ctx, span := r.tracer.Start(ctx, "WorkflowRepository.Create")
	defer span.End()

	stepsJSON, err := json.Marshal(workflow.Steps)
	if err != nil {
		r.logger.WithError(err).Error("failed to marshal workflow steps")
		return err
	}

	workflow.ID = uuid.New()
	workflow.CreatedAt = time.Now().UTC()
	workflow.UpdatedAt = workflow.CreatedAt

	query := sq.Insert("workflows").
		Columns("id", "name", "description", "steps", "status", "created_at", "updated_at").
		Values(
			workflow.ID.String(),
			workflow.Name,
			workflow.Description,
			string(stepsJSON),
			string(workflow.Status),
			workflow.CreatedAt,
			workflow.UpdatedAt,
		).
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		r.logger.WithError(err).Error("failed to build insert query")
		return err
	}

	_, err = r.db.Exec(ctx, querySQL, args...)
	if err != nil {
		r.logger.WithError(err).Error("failed to insert workflow")
		return err
	}

	return nil
}

func (r *WorkflowRepository) Get(ctx context.Context, id string) (*domain.Workflow, error) {
	ctx, span := r.tracer.Start(ctx, "WorkflowRepository.Get")
	defer span.End()

	parsedID, err := uuid.Parse(id)
	if err != nil {
		r.logger.WithError(err).WithField("id", id).Error("invalid workflow ID")
		return nil, err
	}

	query := sq.Select("id", "name", "description", "steps", "status", "created_at", "updated_at").
		From("workflows").
		Where(sq.Eq{"id": parsedID.String()}).
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		r.logger.WithError(err).Error("failed to build select query")
		return nil, err
	}

	var row workflowRow
	err = r.db.Get(ctx, &row, querySQL, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWorkflowNotFound
		}
		r.logger.WithError(err).WithField("id", id).Error("failed to get workflow")
		return nil, err
	}

	return rowToWorkflow(&row)
}

func (r *WorkflowRepository) List(ctx context.Context, opts domain.ListOptions) ([]*domain.Workflow, error) {
	ctx, span := r.tracer.Start(ctx, "WorkflowRepository.List")
	defer span.End()

	query := sq.Select("id", "name", "description", "steps", "status", "created_at", "updated_at").
		From("workflows").
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

	var rows []workflowRow
	err = r.db.Select(ctx, &rows, querySQL, args...)
	if err != nil {
		r.logger.WithError(err).Error("failed to list workflows")
		return nil, err
	}

	workflows := make([]*domain.Workflow, 0, len(rows))
	for i := range rows {
		workflow, err := rowToWorkflow(&rows[i])
		if err != nil {
			r.logger.WithError(err).WithField("row", rows[i]).Error("failed to convert row to workflow")
			return nil, err
		}
		workflows = append(workflows, workflow)
	}

	return workflows, nil
}

func (r *WorkflowRepository) Update(ctx context.Context, workflow *domain.Workflow) error {
	ctx, span := r.tracer.Start(ctx, "WorkflowRepository.Update")
	defer span.End()

	stepsJSON, err := json.Marshal(workflow.Steps)
	if err != nil {
		r.logger.WithError(err).Error("failed to marshal workflow steps")
		return err
	}

	workflow.UpdatedAt = time.Now().UTC()

	query := sq.Update("workflows").
		Set("name", workflow.Name).
		Set("description", workflow.Description).
		Set("steps", string(stepsJSON)).
		Set("status", string(workflow.Status)).
		Set("updated_at", workflow.UpdatedAt).
		Where(sq.Eq{"id": workflow.ID.String()}).
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		r.logger.WithError(err).Error("failed to build update query")
		return err
	}

	result, err := r.db.Exec(ctx, querySQL, args...)
	if err != nil {
		r.logger.WithError(err).WithField("id", workflow.ID).Error("failed to update workflow")
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.logger.WithError(err).Error("failed to get rows affected")
		return err
	}

	if rowsAffected == 0 {
		return ErrWorkflowNotFound
	}

	return nil
}

func rowToWorkflow(row *workflowRow) (*domain.Workflow, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return nil, err
	}

	var steps []domain.StepDefinition
	if err := json.Unmarshal([]byte(row.Steps), &steps); err != nil {
		return nil, err
	}

	return &domain.Workflow{
		ID:          id,
		Name:        row.Name,
		Description: row.Description,
		Steps:       steps,
		Status:      domain.WorkflowStatus(row.Status),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}
