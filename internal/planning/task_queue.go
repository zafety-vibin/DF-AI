package planning

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/df-ai/orchestrator/internal/llm"
)

// TaskPriority represents task urgency
type TaskPriority int

const (
	PriorityCritical TaskPriority = 10 // Survival threats
	PriorityHigh     TaskPriority = 7  // Basic needs
	PriorityMedium   TaskPriority = 5  // Expansion
	PriorityLow      TaskPriority = 3  // Optimization
)

// TaskStatus represents execution state
type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskReady              // Dependencies met, ready to execute
	TaskExecuting
	TaskCompleted
	TaskFailed
	TaskCancelled
)

// Task represents a planned action with dependencies
type Task struct {
	ID           string
	Type         string       // "dig", "build", "multi_step"
	Priority     TaskPriority
	Dependencies []string     // Task IDs that must complete first
	Timeout      time.Duration
	RetryCount   int
	MaxRetries   int

	// Execution details
	Command     llm.CommandSpec
	Subcommands []llm.CommandSpec // For multi-step tasks

	// Tracking
	Status      TaskStatus
	StartedAt   time.Time
	CompletedAt time.Time
	Error       string

	// Metadata
	Description string
	CreatedBy   string // "ai", "human", "system"
	Metadata    map[string]interface{}
}

// IsReady checks if task's dependencies are met
func (t *Task) IsReady(completed map[string]bool) bool {
	if t.Status != TaskPending {
		return false
	}

	for _, depID := range t.Dependencies {
		if !completed[depID] {
			return false // Dependency not completed
		}
	}

	return true
}

// TaskQueue manages planned, executing, and completed tasks
type TaskQueue struct {
	mu sync.RWMutex

	// Queues by state
	pending   *PriorityQueue // Waiting for dependencies
	ready     *PriorityQueue // Ready to execute
	executing map[string]*Task
	completed map[string]*Task
	failed    map[string]*Task

	// Configuration
	maxQueueSize       int
	enableDependencies bool
}

// NewTaskQueue creates a new task queue
func NewTaskQueue(maxSize int, enableDependencies bool) *TaskQueue {
	return &TaskQueue{
		pending:            &PriorityQueue{},
		ready:              &PriorityQueue{},
		executing:          make(map[string]*Task),
		completed:          make(map[string]*Task),
		failed:             make(map[string]*Task),
		maxQueueSize:       maxSize,
		enableDependencies: enableDependencies,
	}
}

// AddTask adds a task to the queue
func (tq *TaskQueue) AddTask(task *Task) error {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	if len(tq.pending.tasks)+len(tq.ready.tasks) >= tq.maxQueueSize {
		return fmt.Errorf("queue full: %d tasks", tq.maxQueueSize)
	}

	task.Status = TaskPending

	// Check if ready to execute
	if task.IsReady(tq.getCompletedIDs()) {
		task.Status = TaskReady
		heap.Push(tq.ready, task)
	} else {
		heap.Push(tq.pending, task)
	}

	return nil
}

// GetNextTask returns highest priority ready task
func (tq *TaskQueue) GetNextTask() *Task {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	// Check for newly ready tasks in pending queue
	tq.promotePendingTasks()

	if tq.ready.Len() == 0 {
		return nil
	}

	task := heap.Pop(tq.ready).(*Task)
	task.Status = TaskExecuting
	task.StartedAt = time.Now()
	tq.executing[task.ID] = task

	return task
}

// CompleteTask marks a task as completed
func (tq *TaskQueue) CompleteTask(taskID string) error {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	task, exists := tq.executing[taskID]
	if !exists {
		return fmt.Errorf("task not found in executing queue: %s", taskID)
	}

	task.Status = TaskCompleted
	task.CompletedAt = time.Now()
	delete(tq.executing, taskID)
	tq.completed[taskID] = task

	// Check if any pending tasks are now ready
	tq.promotePendingTasks()

	return nil
}

// FailTask marks a task as failed
func (tq *TaskQueue) FailTask(taskID string, err error) error {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	task, exists := tq.executing[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Error = err.Error()

	// Check retry
	if task.RetryCount < task.MaxRetries {
		task.RetryCount++
		task.Status = TaskPending
		heap.Push(tq.pending, task)
		delete(tq.executing, taskID)
		return nil // Will retry
	}

	// Max retries exceeded
	task.Status = TaskFailed
	delete(tq.executing, taskID)
	tq.failed[taskID] = task

	return nil
}

// promotePendingTasks checks pending tasks and promotes ready ones
func (tq *TaskQueue) promotePendingTasks() {
	completedIDs := tq.getCompletedIDs()

	// Scan pending queue for ready tasks
	newPending := &PriorityQueue{}
	for tq.pending.Len() > 0 {
		task := heap.Pop(tq.pending).(*Task)
		if task.IsReady(completedIDs) {
			task.Status = TaskReady
			heap.Push(tq.ready, task)
		} else {
			heap.Push(newPending, task)
		}
	}

	tq.pending = newPending
}

// getCompletedIDs returns set of completed task IDs
func (tq *TaskQueue) getCompletedIDs() map[string]bool {
	ids := make(map[string]bool)
	for id := range tq.completed {
		ids[id] = true
	}
	return ids
}

// GetQueueStatus returns summary of queue state
func (tq *TaskQueue) GetQueueStatus() *QueueStatus {
	tq.mu.RLock()
	defer tq.mu.RUnlock()

	return &QueueStatus{
		Pending:   tq.pending.Len(),
		Ready:     tq.ready.Len(),
		Executing: len(tq.executing),
		Completed: len(tq.completed),
		Failed:    len(tq.failed),
	}
}

// QueueStatus represents queue metrics
type QueueStatus struct {
	Pending   int
	Ready     int
	Executing int
	Completed int
	Failed    int
}

// PriorityQueue implements heap.Interface for task prioritization
type PriorityQueue struct {
	tasks []*Task
}

func (pq PriorityQueue) Len() int { return len(pq.tasks) }

func (pq PriorityQueue) Less(i, j int) bool {
	// Higher priority first
	return pq.tasks[i].Priority > pq.tasks[j].Priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq.tasks[i], pq.tasks[j] = pq.tasks[j], pq.tasks[i]
}

func (pq *PriorityQueue) Push(x interface{}) {
	pq.tasks = append(pq.tasks, x.(*Task))
}

func (pq *PriorityQueue) Pop() interface{} {
	old := pq.tasks
	n := len(old)
	task := old[n-1]
	pq.tasks = old[0 : n-1]
	return task
}
