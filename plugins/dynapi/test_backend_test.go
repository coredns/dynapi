package dynapi

import "context"

type testBackend struct {
	err    error
	result recordSet
	writes int
}

func (testBackend *testBackend) read(context.Context, string, uint16) (recordSet, error) {
	return testBackend.result, testBackend.err
}

func (testBackend *testBackend) replace(
	_ context.Context,
	_ string,
	_ uint16,
	result recordSet,
) error {
	testBackend.writes++

	testBackend.result = result

	return testBackend.err
}

func (testBackend *testBackend) delete(context.Context, string, uint16) error {
	testBackend.writes++

	return testBackend.err
}
