package mocks

import "github.com/stretchr/testify/mock"

// ObjectStoreOptions is an autogenerated mock type for the ObjectStoreOptions type
type ObjectStoreOptions struct {
	mock.Mock
}

// GetIndexes provides a mock function with given fields:
func (_m *ObjectStoreOptions) GetIndexes() map[string][]string {
	ret := _m.Called()

	var r0 map[string][]string
	if rf, ok := ret.Get(0).(func() map[string][]string); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string][]string)
		}
	}

	return r0
}
