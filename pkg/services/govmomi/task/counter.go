package task

import (
	"sync"
)

// Counter tracks the number of tasks currently running in vcenter
type Counter struct {
	value int
	mu    sync.Mutex
}

func (c *Counter) Increment() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
}

func (c *Counter) Decrement() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value--
}

func (c *Counter) Value() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}
