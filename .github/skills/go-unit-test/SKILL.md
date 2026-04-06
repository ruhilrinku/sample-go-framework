---
name: go-unit-test
description: "Use when: writing unit tests, adding test coverage, creating test cases for Go code. Covers manual mock creation, test naming conventions, layer-independent testing for service and gRPC adapter layers. Use for test files, mock structs, table-driven tests."
argument-hint: "Provide the file or function to test"
---

# Go Unit Testing

## When to Use

- Writing unit tests for service layer or gRPC adapter layer
- Adding test coverage to existing code
- User says "add tests", "write tests", "test this function"

## Conventions

### Naming

- Test functions: `Test{Function}_{Scenario}` — e.g., `TestListItems_Success`, `TestCreateItem_EmptyName`
- Test files: `<source>_test.go` in the same directory (or `_test` package for adapter tests)
- Mock structs: `mock<Interface>` — e.g., `mockItemRepository`, `mockItemService`

### Manual Mocks

This project uses **hand-written mock structs** — NO codegen mocks (no mockgen, no testify mocks).

Mock pattern:
```go
type mock<Interface> struct {
	// Pre-configured return values
	items       []domain.<Feature>DomainModel
	total       int
	err         error
	createdItem domain.<Feature>DomainModel
	createErr   error
}

func (m *mock<Interface>) List<Feature>s(_ context.Context, _, _ int) ([]domain.<Feature>DomainModel, int, error) {
	return m.items, m.total, m.err
}

func (m *mock<Interface>) Create<Feature>(_ context.Context, item domain.<Feature>DomainModel) (domain.<Feature>DomainModel, error) {
	if m.createErr != nil {
		return domain.<Feature>DomainModel{}, m.createErr
	}
	result := m.createdItem
	result.Name = item.Name
	result.Description = item.Description
	return result, nil
}
```

Key rules:
- Mock struct fields hold pre-configured return values
- Unused context params use `_`
- Create methods may copy input fields onto the preset result to mimic real behavior
- Each mock lives in the test file, not a separate package

### Layer Independence

| Test Target | Mock This | Package |
|---|---|---|
| Service (`*_service_test.go`) | Repository port | Same package (`service`) |
| gRPC Adapter (`*_server_test.go`) | Service port | `_test` package (external) |

### Assertions

Use standard `testing` package only — no testify, no gomega:
```go
if err != nil {
	t.Fatalf("unexpected error: %v", err)
}
if got != want {
	t.Errorf("field = %v, want %v", got, want)
}
```

- `t.Fatalf` for errors that prevent further checks
- `t.Errorf` for field-level comparisons (test continues)

### Error Type Checking

Use `domain.GetErrorType(err)` to verify domain error classification:
```go
if domain.GetErrorType(err) != domain.ErrorTypeValidation {
	t.Errorf("error type = %v, want ErrorTypeValidation", domain.GetErrorType(err))
}
```

For gRPC adapter tests, verify gRPC status codes:
```go
st, ok := status.FromError(err)
if !ok {
	t.Fatalf("expected gRPC status error, got %T", err)
}
if st.Code() != codes.InvalidArgument {
	t.Errorf("code = %v, want %v", st.Code(), codes.InvalidArgument)
}
```

## Procedure

### Service Layer Tests

1. Create `mock<Interface>` struct implementing the repository port
2. Write test functions covering:
   - **Success path** — valid input returns expected output
   - **Validation errors** — invalid input returns `ErrorTypeValidation`
   - **Repository errors** — repo failure returns `ErrorTypeInternal`
   - **Edge cases** — default pagination, empty results, boundary values
3. Each test: construct mock → create service via `New(mock, slog.Default())` → call method → assert

Template:
```go
func TestCreate<Feature>_Success(t *testing.T) {
	repo := &mockRepository{
		createdItem: domain.<Feature>DomainModel{ID: "new-id"},
	}
	svc := service.New(repo, slog.Default())

	item, err := svc.Create<Feature>(context.Background(), "Test", "Description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ID != "new-id" {
		t.Errorf("ID = %q, want %q", item.ID, "new-id")
	}
}
```

### gRPC Adapter Tests

1. Create `mock<Interface>` struct implementing the service port
2. Use `_test` package (external test package): `package grpc_test`
3. Import the adapter package with alias: `grpcadapter "...adapter/grpc"`
4. Write tests covering:
   - **Success path** — valid request returns correct protobuf response
   - **Input validation** — empty required fields return `codes.InvalidArgument`
   - **Service errors** — each domain error type maps to correct gRPC code
   - **Empty results** — empty list returns 0 items and 0 total
5. Each test: construct mock → create server via `grpcadapter.NewServer(mock, slog.Default())` → call RPC → assert response + status code

Template:
```go
func TestServer_Create<Feature>_Success(t *testing.T) {
	svc := &mockService{
		createdItem: domain.<Feature>DomainModel{
			ID: "c1", Name: "Test", Description: "Desc",
		},
	}
	server := grpcadapter.New<Feature>Server(svc, slog.Default())

	resp, err := server.Create<Feature>(context.Background(), &pb.Create<Feature>Request{
		Name:        "Test",
		Description: "Desc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Item.Id != "c1" {
		t.Errorf("Item.Id = %q, want %q", resp.Item.Id, "c1")
	}
}
```

### Running Tests

```bash
make test                    # All tests (unit + BDD)
go test ./internal/<feature>/... -v   # Feature-specific tests
go test ./internal/<feature>/core/service/... -v  # Service layer only
go test ./internal/<feature>/adapter/grpc/... -v  # gRPC adapter only
```
