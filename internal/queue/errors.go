package queue

import "errors"

var ErrMaxRetriesExceed = errors.New("queue: max retries exceed")
