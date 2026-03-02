package types //nolint:revive // this package name is acceptable

func Ptr[T any](val T) *T {
	return &val
}
