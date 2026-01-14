// Package th provides some simple type helpers.
package th

// Must(T, error) takes any T, panics if there is an error, and returns T.
func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
