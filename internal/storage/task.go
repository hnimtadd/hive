package storage

import (
	"errors"
	"fmt"
	"os"

	"github.com/hnimtadd/hive/pkg/types"
	"github.com/hnimtadd/hive/pkg/utils"

	clover "github.com/ostafen/clover/v2"
	document "github.com/ostafen/clover/v2/document"
	"github.com/ostafen/clover/v2/query"
)

type TaskStorage interface {
	List() ([]*types.HiveTask, error)
	Add(task *types.HiveTask) error
	Delete(id string) (*types.HiveTask, error)
	Load(id string) (*types.HiveTask, error)
}

type taskStorage struct {
	db *clover.DB
}

type Options struct {
	Storage string
}

const taskCollection = "tasks"

func NewLocalStorage(opts Options) (TaskStorage, error) {
check:
	stat, err := os.Stat(opts.Storage)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("invalid task storage: %w", err)
		}
		if err = os.MkdirAll(opts.Storage, 0o700); err != nil {
			return nil, fmt.Errorf("failed to create task storage: %w", err)
		}
		goto check
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("task storage is not a dir; %s", opts.Storage)
	}
	db, err := clover.Open(opts.Storage)
	if err != nil {
		return nil, fmt.Errorf("failed to init local db: %w", err)
	}
	exists, err := db.HasCollection(taskCollection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		if err = db.CreateCollection(taskCollection); err != nil {
			return nil, fmt.Errorf("failed to create collection: %w", err)
		}
	}

	return &taskStorage{
		db: db,
	}, nil
}

// Add implements [TaskStorage].
func (t *taskStorage) Add(task *types.HiveTask) error {
	doc := document.NewDocument()
	payload, err := utils.JSONConvert[map[string]any](task)
	if err != nil {
		return fmt.Errorf("failed to convert task to document payload: %w", err)
	}
	doc.SetAll(payload)
	if err = t.db.Insert(taskCollection, doc); err != nil {
		return fmt.Errorf("failed to insert task to the storage: %w", err)
	}
	return nil
}

// Delete implements [TaskStorage].
func (t *taskStorage) Delete(id string) (*types.HiveTask, error) {
	task, err := t.Load(id)
	if err != nil {
		return nil, fmt.Errorf("failed to load task: %w", err)
	}
	if err = t.db.DeleteById(taskCollection, id); err != nil {
		return nil, fmt.Errorf("failed to delete task; %w", err)
	}
	return task, nil
}

// List implements [TaskStorage].
func (t *taskStorage) List() ([]*types.HiveTask, error) {
	tasks := []*types.HiveTask{}
	if err := t.db.ForEach(query.NewQuery(taskCollection), func(doc *document.Document) bool {
		task, err := toTask(doc)
		if err != nil {
			return false
		}
		tasks = append(tasks, task)
		return true
	}); err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	return tasks, nil
}

// Load implements [TaskStorage].
func (t *taskStorage) Load(id string) (*types.HiveTask, error) {
	doc, err := t.db.FindById(taskCollection, id)
	if err != nil {
		return nil, fmt.Errorf("failed to find document: %w", err)
	}
	task, err := toTask(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to convert document to task: %w", err)
	}
	return task, nil
}

func toTask(doc *document.Document) (*types.HiveTask, error) {
	payload := doc.ToMap()
	task, err := utils.JSONConvert[types.HiveTask](payload)
	if err != nil {
		return nil, fmt.Errorf("failed to convert payload to hive task: %w", err)
	}
	return &task, nil
}
