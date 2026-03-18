package types

import (
	"fmt"
	"testing"
)

func TestTaskSelfDescription(t *testing.T) {
	selfDescription := TaskSelfDescription()
	fmt.Println(selfDescription)
}
