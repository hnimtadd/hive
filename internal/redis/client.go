package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/types"
)

const (
	// Redis keys
	TaskQueueKey      = "hive:task_queue"
	TaskHashPrefix    = "hive:task:"
	TaskUpdatesPrefix = "hive:updates:"
	FeedbackPrefix    = "hive:feedback:"
	AgentHeartbeat    = "hive:agents:heartbeat"
	ActiveTasksSet    = "hive:active_tasks"

	// Channels
	TaskUpdateChannel  = "hive:task_updates"
	FeedbackChannel    = "hive:feedback_requests"
	AgentStatusChannel = "hive:agent_status"
)

// Client wraps the Redis client with Hive-specific operations
type Client struct {
	rdb *redis.Client
}

// NewClient creates a new Redis client for Hive operations
func NewClient() (*Client, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return NewClientWithConfig(&cfg.Redis)
}

// NewClientWithConfig creates a new Redis client with provided config
func NewClientWithConfig(cfg *config.RedisConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.rdb.Close()
}

// SubmitTask adds a new task to the task queue
func (c *Client) SubmitTask(ctx context.Context, task *types.HiveTask) error {
	// Serialize task to JSON
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// Store task in hash
	taskKey := TaskHashPrefix + task.ID
	if err := c.rdb.HSet(ctx, taskKey, "data", taskJSON).Err(); err != nil {
		return fmt.Errorf("failed to store task: %w", err)
	}

	// Add to task queue (FIFO)
	if err := c.rdb.LPush(ctx, TaskQueueKey, task.ID).Err(); err != nil {
		return fmt.Errorf("failed to queue task: %w", err)
	}

	// Add to active tasks set
	if err := c.rdb.SAdd(ctx, ActiveTasksSet, task.ID).Err(); err != nil {
		return fmt.Errorf("failed to add to active tasks: %w", err)
	}

	return nil
}

// GetNextTask retrieves and removes the next task from the queue
func (c *Client) GetNextTask(ctx context.Context) (*types.HiveTask, error) {
	// Pop task ID from queue (blocking with timeout)
	result, err := c.rdb.BRPop(ctx, 10*time.Second, TaskQueueKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No tasks available
		}
		return nil, fmt.Errorf("failed to pop from task queue: %w", err)
	}

	taskID := result[1]
	return c.GetTask(ctx, taskID)
}

// GetTask retrieves a task by ID
func (c *Client) GetTask(ctx context.Context, taskID string) (*types.HiveTask, error) {
	taskKey := TaskHashPrefix + taskID
	taskJSON, err := c.rdb.HGet(ctx, taskKey, "data").Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("task not found: %s", taskID)
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	var task types.HiveTask
	if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return &task, nil
}

// UpdateTask updates an existing task
func (c *Client) UpdateTask(ctx context.Context, task *types.HiveTask) error {
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	taskKey := TaskHashPrefix + task.ID
	if err := c.rdb.HSet(ctx, taskKey, "data", taskJSON).Err(); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	// Publish task update
	updateJSON, _ := json.Marshal(map[string]interface{}{
		"task_id":   task.ID,
		"status":    task.Status,
		"progress":  task.Progress,
		"timestamp": time.Now(),
		"task":      task,
	})

	if err := c.rdb.Publish(ctx, TaskUpdateChannel, updateJSON).Err(); err != nil {
		return fmt.Errorf("failed to publish task update: %w", err)
	}

	// Remove from active tasks if completed or failed
	if task.Status == types.TaskStatusCompleted || task.Status == types.TaskStatusFailed {
		c.rdb.SRem(ctx, ActiveTasksSet, task.ID)
	}

	return nil
}

// SubscribeToTaskUpdates subscribes to task update notifications
func (c *Client) SubscribeToTaskUpdates(ctx context.Context, taskID string) (<-chan *types.HiveTask, error) {
	pubsub := c.rdb.Subscribe(ctx, TaskUpdateChannel)

	ch := make(chan *types.HiveTask, 10)

	go func() {
		defer close(ch)
		defer pubsub.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-pubsub.Channel():
				var update struct {
					TaskID string          `json:"task_id"`
					Task   *types.HiveTask `json:"task"`
				}

				if err := json.Unmarshal([]byte(msg.Payload), &update); err != nil {
					continue
				}

				// Only send updates for the requested task
				if update.TaskID == taskID {
					select {
					case ch <- update.Task:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return ch, nil
}

// ProvideFeedback sends feedback response for a paused task
func (c *Client) ProvideFeedback(ctx context.Context, taskID, response string) error {
	feedbackKey := FeedbackPrefix + taskID

	feedback := map[string]interface{}{
		"task_id":   taskID,
		"response":  response,
		"timestamp": time.Now(),
	}

	feedbackJSON, err := json.Marshal(feedback)
	if err != nil {
		return fmt.Errorf("failed to marshal feedback: %w", err)
	}

	// Store feedback
	if err := c.rdb.Set(ctx, feedbackKey, feedbackJSON, time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to store feedback: %w", err)
	}

	// Publish feedback notification
	return c.rdb.Publish(ctx, FeedbackChannel, feedbackJSON).Err()
}

// WaitForFeedback waits for human feedback on a task
func (c *Client) WaitForFeedback(ctx context.Context, taskID string) (string, error) {
	feedbackKey := FeedbackPrefix + taskID

	// Subscribe to feedback channel
	pubsub := c.rdb.Subscribe(ctx, FeedbackChannel)
	defer pubsub.Close()

	// Check if feedback already exists
	if existing, err := c.rdb.Get(ctx, feedbackKey).Result(); err == nil {
		var feedback struct {
			Response string `json:"response"`
		}
		if json.Unmarshal([]byte(existing), &feedback) == nil {
			return feedback.Response, nil
		}
	}

	// Wait for new feedback
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case msg := <-pubsub.Channel():
			var feedback struct {
				TaskID   string `json:"task_id"`
				Response string `json:"response"`
			}

			if err := json.Unmarshal([]byte(msg.Payload), &feedback); err != nil {
				continue
			}

			if feedback.TaskID == taskID {
				return feedback.Response, nil
			}
		}
	}
}

// ListActiveTasks returns all currently active tasks
func (c *Client) ListActiveTasks(ctx context.Context) ([]*types.HiveTask, error) {
	taskIDs, err := c.rdb.SMembers(ctx, ActiveTasksSet).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get active task IDs: %w", err)
	}

	tasks := make([]*types.HiveTask, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		task, err := c.GetTask(ctx, taskID)
		if err != nil {
			continue // Skip tasks that can't be loaded
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// RegisterAgent registers an agent and starts heartbeat
func (c *Client) RegisterAgent(ctx context.Context, agentID, agentType string) error {
	agentInfo := map[string]interface{}{
		"id":         agentID,
		"type":       agentType,
		"registered": time.Now(),
		"last_seen":  time.Now(),
		"status":     "active",
	}

	agentJSON, err := json.Marshal(agentInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal agent info: %w", err)
	}

	// Store agent info with expiration
	agentKey := fmt.Sprintf("%s:%s", AgentHeartbeat, agentID)
	return c.rdb.Set(ctx, agentKey, agentJSON, 30*time.Second).Err()
}

// Heartbeat updates agent's last seen timestamp
func (c *Client) Heartbeat(ctx context.Context, agentID string) error {
	agentKey := fmt.Sprintf("%s:%s", AgentHeartbeat, agentID)

	// Update last_seen field
	return c.rdb.HSet(ctx, agentKey, "last_seen", time.Now()).Err()
}

// GetActiveAgents returns all currently active agents
func (c *Client) GetActiveAgents(ctx context.Context) ([]map[string]interface{}, error) {
	pattern := AgentHeartbeat + ":*"
	keys, err := c.rdb.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get agent keys: %w", err)
	}

	agents := make([]map[string]interface{}, 0, len(keys))
	for _, key := range keys {
		agentJSON, err := c.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var agent map[string]interface{}
		if json.Unmarshal([]byte(agentJSON), &agent) == nil {
			agents = append(agents, agent)
		}
	}

	return agents, nil
}

