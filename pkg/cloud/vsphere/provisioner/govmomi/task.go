package govmomi

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

type Task struct {
	task *object.Task
	err  error
	ctx  context.Context
}

func (t *Task) WaitFor(ctx ...context.Context) error {
	if t.err != nil {
		return t.err
	}
	_ctx := t.ctx
	if len(ctx) > 0 {
		_ctx = ctx[0]
	}

	taskinfo, err := t.task.WaitForResult(_ctx, nil)
	if err != nil {
		return err
	}
	if taskinfo.State != types.TaskInfoStateSuccess {
		return fmt.Errorf("task %s, with result: %s %s", taskinfo.State, taskinfo.Result, taskinfo.Error)
	}
	return nil
}
