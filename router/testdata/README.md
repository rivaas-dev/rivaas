# Router Test Data

This directory contains test fixtures and golden files used by router package tests.

## Directory Structure

```
testdata/
├── fixtures/         # Input test data
│   ├── valid_user_request.json      # Valid JSON request body
│   ├── invalid_user_request.json    # Invalid JSON for error testing
│   └── multipart_form.txt           # Multipart form data sample
├── golden/           # Expected output files
│   ├── routes_introspection.json    # Expected route listing output
│   ├── error_response_404.json      # Expected 404 error response
│   └── error_response_405.json      # Expected 405 error response
└── README.md         # This file
```

## Usage

### Loading Test Fixtures

```go
func TestHandler(t *testing.T) {
    t.Parallel()

    // Load test fixture
    input, err := os.ReadFile("testdata/fixtures/valid_user_request.json")
    require.NoError(t, err)

    // Use in test...
}
```

### Golden File Testing

Golden files store expected output for comparison. Use the `-update` flag pattern to regenerate:

```go
var updateGolden = flag.Bool("update", false, "update golden files")

func TestRouteIntrospection_Golden(t *testing.T) {
    result := router.Routes()
    goldenPath := "testdata/golden/routes_introspection.json"

    if *updateGolden {
        err := os.WriteFile(goldenPath, []byte(result), 0644)
        require.NoError(t, err)
        return
    }

    expected, err := os.ReadFile(goldenPath)
    require.NoError(t, err)
    assert.JSONEq(t, string(expected), string(result))
}
```

Update golden files with:

```bash
go test -update ./...
```

## Adding New Test Data

1. **Fixtures**: Add sample input files under `fixtures/`
2. **Golden files**: Add expected output files under `golden/`
3. **Document**: Update this README with the new file's purpose

## Guidelines

- Keep test data files small and focused
- Use descriptive file names that indicate their purpose
- Document the structure of complex test data files
- Version control all test data (it's part of the test suite)
