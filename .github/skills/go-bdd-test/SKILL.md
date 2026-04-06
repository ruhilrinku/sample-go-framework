---
name: go-bdd-test
description: "Use when: writing BDD tests, Cucumber tests, Gherkin feature files, godog step definitions. Creates .feature files and step definitions using the testContext pattern with mocked service layer. Use for behavior-driven tests, acceptance tests, scenario testing."
argument-hint: "Provide the feature or RPC to write BDD tests for"
---

# Go BDD / Cucumber Testing

## When to Use

- Writing behavior-driven tests for gRPC adapter layer
- User says "add BDD tests", "write Cucumber tests", "add feature file", "add scenarios"
- Testing complete request/response flows through the gRPC adapter with mocked service

## Architecture

BDD tests live under `test/`:
```
test/
  features/
    <feature_operation>.feature    # Gherkin scenarios
  <feature>_server_cucumber_test.go  # Step definitions + test runner
```

Tests exercise the **gRPC adapter with mocked service** — no database, no network.

## Procedure

### 1. Write Gherkin Feature File

File: `test/features/<operation>.feature`

Structure:
```gherkin
Feature: <Operation> via gRPC
  As an API consumer
  I want to <operation> through the gRPC API
  So that <business value>

  Scenario: Successfully <operation>
    Given <precondition using mock>
    When <action calling RPC>
    Then <assertion on response>
    And no error should be returned

  Scenario: <Operation> with invalid input returns validation error
    When <action with bad input>
    Then a gRPC error with code "InvalidArgument" should be returned

  Scenario: <Operation> when service fails returns internal error
    Given the service will fail to <operation> with an internal error "<message>"
    When <action>
    Then a gRPC error with code "Internal" should be returned
```

Rules:
- Step text uses double-quoted strings for parameters: `"([^"]*)"`
- Numeric parameters use `(\d+)`
- Data tables for structured input (header row + data rows)
- Cover: success, empty result, validation error, internal error, not-found error

### 2. Write Step Definitions

File: `test/<feature>_server_cucumber_test.go`

Package: `grpc_test` (external test package)

#### testContext Pattern

All state shared across steps lives in a `testContext` struct:
```go
type testContext struct {
	mock            *mockService
	server          *grpcadapter.<Feature>Server
	createResponse  *pb.Create<Feature>Response
	listResponse    *pb.List<Feature>sResponse
	err             error
}

func newTestContext() *testContext {
	tc := &testContext{
		mock: &mockService{},
	}
	tc.server = grpcadapter.New<Feature>Server(tc.mock, slog.Default())
	return tc
}
```

#### Mock Service

Define the same `mockService` struct as in unit tests (implements the service port):
```go
type mockService struct {
	items       []domain.<Feature>DomainModel
	total       int
	err         error
	createdItem domain.<Feature>DomainModel
	createErr   error
}
```

#### Step Functions

Step functions are methods on `testContext`:
```go
// Given steps — configure mock
func (tc *testContext) theServiceWillReturnACreatedItemWithID(id string) error {
	tc.mock.createdItem = domain.<Feature>DomainModel{ID: id}
	return nil
}

// When steps — call the gRPC server
func (tc *testContext) iCreate<Feature>WithNameAndDescription(name, desc string) error {
	tc.createResponse, tc.err = tc.server.Create<Feature>(context.Background(), &pb.Create<Feature>Request{
		Name:        name,
		Description: desc,
	})
	return nil
}

// Then steps — assert on response
func (tc *testContext) theResponseShouldContainAnItemWithID(id string) error {
	if tc.createResponse == nil || tc.createResponse.Item == nil {
		return fmt.Errorf("expected response with item, got nil")
	}
	if tc.createResponse.Item.Id != id {
		return fmt.Errorf("expected item id %q, got %q", id, tc.createResponse.Item.Id)
	}
	return nil
}
```

Rules:
- Given steps set up mock return values
- When steps call the gRPC server method and store response + error on `tc`
- Then steps validate `tc.createResponse`, `tc.listResponse`, or `tc.err`
- Step functions return `error` — return `nil` on success, `fmt.Errorf(...)` on assertion failure
- Never use `t.Fatal` or `t.Error` inside steps — only return errors

#### Shared Steps

Reuse these shared steps across features:
```go
func (tc *testContext) noErrorShouldBeReturned() error {
	if tc.err != nil {
		return fmt.Errorf("expected no error, got: %v", tc.err)
	}
	return nil
}

func (tc *testContext) aGRPCErrorWithCodeShouldBeReturned(codeStr string) error {
	if tc.err == nil {
		return fmt.Errorf("expected gRPC error with code %s, got nil", codeStr)
	}
	st, ok := status.FromError(tc.err)
	if !ok {
		return fmt.Errorf("expected gRPC status error, got %T: %v", tc.err, tc.err)
	}
	codeMap := map[string]codes.Code{
		"InvalidArgument": codes.InvalidArgument,
		"NotFound":        codes.NotFound,
		"Internal":        codes.Internal,
	}
	expected, exists := codeMap[codeStr]
	if !exists {
		return fmt.Errorf("unknown gRPC code: %s", codeStr)
	}
	if st.Code() != expected {
		return fmt.Errorf("expected gRPC code %s (%d), got %s (%d)", codeStr, expected, st.Code().String(), st.Code())
	}
	return nil
}
```

### 3. Register Steps in InitializeScenario

```go
func InitializeScenario(ctx *godog.ScenarioContext) {
	tc := newTestContext()

	// Reset state before each scenario
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		*tc = *newTestContext()
		return ctx, nil
	})

	// Register steps with regex patterns
	ctx.Step(`^the service will return a created item with id "([^"]*)"$`, tc.theServiceWillReturnACreatedItemWithID)
	ctx.Step(`^I create an item with name "([^"]*)" and description "([^"]*)"$`, tc.iCreateItemWithNameAndDescription)
	// ... more steps

	// Shared steps
	ctx.Step(`^no error should be returned$`, tc.noErrorShouldBeReturned)
	ctx.Step(`^a gRPC error with code "([^"]*)" should be returned$`, tc.aGRPCErrorWithCodeShouldBeReturned)
}
```

Rules:
- Regex patterns use `"([^"]*)"` for string params, `(\d+)` for ints
- `ctx.Before` resets `testContext` before each scenario for isolation
- Step regex must end with `$` to anchor
- Numeric params from regex are strings — parse with `strconv.Atoi`

### 4. Test Runner

```go
func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
```

### 5. Running BDD Tests

```bash
go test ./test/... -run TestFeatures -v
```

## Data Table Steps

For steps with tabular data:
```gherkin
Given the service has the following items:
  | id | name  | description |
  | a1 | Alpha | First       |
  | b2 | Beta  | Second      |
```

Step function receives `*godog.Table`:
```go
func (tc *testContext) theServiceHasTheFollowingItems(table *godog.Table) error {
	items := make([]domain.ItemDomainModel, 0, len(table.Rows)-1)
	for _, row := range table.Rows[1:] { // skip header row
		items = append(items, domain.ItemDomainModel{
			ID:          row.Cells[0].Value,
			Name:        row.Cells[1].Value,
			Description: row.Cells[2].Value,
		})
	}
	tc.mock.items = items
	tc.mock.total = len(items)
	return nil
}
```
