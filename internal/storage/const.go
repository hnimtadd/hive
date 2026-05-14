package storage

import "time"

const aquireLockTimeout = time.Second * 2

type Options struct {
	Storage string
}
