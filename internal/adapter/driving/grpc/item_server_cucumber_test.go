package grpc_test

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"testing"

	"github.com/cucumber/godog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	itemv1 "github.com/sample-go/item-service/gen/pb/item/v1"
	grpcadapter "github.com/sample-go/item-service/internal/adapter/driving/grpc"
	"github.com/sample-go/item-service/internal/core/domain"
)

// testContext holds state shared across step definitions within a scenario.
type testContext struct {
	mock           *mockItemService
	server         *grpcadapter.ItemServer
	createResponse *itemv1.CreateItemResponse
	listResponse   *itemv1.ListItemsResponse
	err            error
}

func newTestContext() *testContext {
	tc := &testContext{
		mock: &mockItemService{},
	}
	tc.server = grpcadapter.NewItemServer(tc.mock, slog.Default())
	return tc
}

// --- CreateItem steps ---

func (tc *testContext) theServiceWillReturnACreatedItemWithID(id string) error {
	tc.mock.createdItem = domain.ItemDomainModel{ID: id}
	return nil
}

func (tc *testContext) iCreateAnItemWithNameAndDescription(name, description string) error {
	// Populate name/description on the mock's preset item so it mirrors real service behavior
	tc.mock.createdItem.Name = name
	tc.mock.createdItem.Description = description
	tc.createResponse, tc.err = tc.server.CreateItem(context.Background(), &itemv1.CreateItemRequest{
		Name:        name,
		Description: description,
	})
	return nil
}

func (tc *testContext) theResponseShouldContainAnItemWithID(id string) error {
	if tc.createResponse == nil || tc.createResponse.Item == nil {
		return fmt.Errorf("expected response with item, got nil")
	}
	if tc.createResponse.Item.Id != id {
		return fmt.Errorf("expected item id %q, got %q", id, tc.createResponse.Item.Id)
	}
	return nil
}

func (tc *testContext) theResponseItemNameShouldBe(name string) error {
	if tc.createResponse.Item.Name != name {
		return fmt.Errorf("expected item name %q, got %q", name, tc.createResponse.Item.Name)
	}
	return nil
}

func (tc *testContext) theResponseItemDescriptionShouldBe(desc string) error {
	if tc.createResponse.Item.Description != desc {
		return fmt.Errorf("expected item description %q, got %q", desc, tc.createResponse.Item.Description)
	}
	return nil
}

func (tc *testContext) theServiceWillFailToCreateWithAnInternalError(msg string) error {
	tc.mock.createErr = domain.NewInternalError(msg, nil)
	return nil
}

func (tc *testContext) theServiceWillFailToCreateWithAValidationError(msg string) error {
	tc.mock.createErr = domain.NewValidationError(msg)
	return nil
}

// --- ListItems steps ---

func (tc *testContext) theServiceHasTheFollowingItems(table *godog.Table) error {
	items := make([]domain.ItemDomainModel, 0, len(table.Rows)-1)
	for _, row := range table.Rows[1:] { // skip header
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

func (tc *testContext) theServiceHasNoItems() error {
	tc.mock.items = nil
	tc.mock.total = 0
	return nil
}

func (tc *testContext) iListItemsWithPageAndPageSize(page, pageSize int) error {
	tc.listResponse, tc.err = tc.server.ListItems(context.Background(), &itemv1.ListItemsRequest{
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	return nil
}

func (tc *testContext) theResponseShouldContainNItems(count int) error {
	if tc.listResponse == nil {
		return fmt.Errorf("expected response, got nil")
	}
	if len(tc.listResponse.Items) != count {
		return fmt.Errorf("expected %d items, got %d", count, len(tc.listResponse.Items))
	}
	return nil
}

func (tc *testContext) theTotalCountShouldBe(total int) error {
	if tc.listResponse == nil {
		return fmt.Errorf("expected response, got nil")
	}
	if int(tc.listResponse.TotalCount) != total {
		return fmt.Errorf("expected total count %d, got %d", total, tc.listResponse.TotalCount)
	}
	return nil
}

func (tc *testContext) itemNShouldHaveIDAndName(n int, id, name string) error {
	idx := n - 1
	if idx < 0 || idx >= len(tc.listResponse.Items) {
		return fmt.Errorf("item index %d out of range (have %d items)", n, len(tc.listResponse.Items))
	}
	item := tc.listResponse.Items[idx]
	if item.Id != id {
		return fmt.Errorf("item %d: expected id %q, got %q", n, id, item.Id)
	}
	if item.Name != name {
		return fmt.Errorf("item %d: expected name %q, got %q", n, name, item.Name)
	}
	return nil
}

func (tc *testContext) theServiceWillFailToListWithAnInternalError(msg string) error {
	tc.mock.err = domain.NewInternalError(msg, nil)
	return nil
}

func (tc *testContext) theServiceWillFailToListWithANotFoundError(msg string) error {
	tc.mock.err = domain.NewNotFoundError(msg)
	return nil
}

// --- Shared steps ---

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

func InitializeScenario(ctx *godog.ScenarioContext) {
	tc := newTestContext()

	// Reset state before each scenario
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		*tc = *newTestContext()
		return ctx, nil
	})

	// CreateItem steps
	ctx.Step(`^the service will return a created item with id "([^"]*)"$`, tc.theServiceWillReturnACreatedItemWithID)
	ctx.Step(`^I create an item with name "([^"]*)" and description "([^"]*)"$`, tc.iCreateAnItemWithNameAndDescription)
	ctx.Step(`^the response should contain an item with id "([^"]*)"$`, tc.theResponseShouldContainAnItemWithID)
	ctx.Step(`^the response item name should be "([^"]*)"$`, tc.theResponseItemNameShouldBe)
	ctx.Step(`^the response item description should be "([^"]*)"$`, tc.theResponseItemDescriptionShouldBe)
	ctx.Step(`^the service will fail to create with an internal error "([^"]*)"$`, tc.theServiceWillFailToCreateWithAnInternalError)
	ctx.Step(`^the service will fail to create with a validation error "([^"]*)"$`, tc.theServiceWillFailToCreateWithAValidationError)

	// ListItems steps
	ctx.Step(`^the service has the following items:$`, tc.theServiceHasTheFollowingItems)
	ctx.Step(`^the service has no items$`, tc.theServiceHasNoItems)
	ctx.Step(`^I list items with page (\d+) and page_size (\d+)$`, func(page, pageSize string) error {
		p, _ := strconv.Atoi(page)
		ps, _ := strconv.Atoi(pageSize)
		return tc.iListItemsWithPageAndPageSize(p, ps)
	})
	ctx.Step(`^the response should contain (\d+) items$`, func(count string) error {
		c, _ := strconv.Atoi(count)
		return tc.theResponseShouldContainNItems(c)
	})
	ctx.Step(`^the total count should be (\d+)$`, func(total string) error {
		t, _ := strconv.Atoi(total)
		return tc.theTotalCountShouldBe(t)
	})
	ctx.Step(`^item (\d+) should have id "([^"]*)" and name "([^"]*)"$`, func(n, id, name string) error {
		idx, _ := strconv.Atoi(n)
		return tc.itemNShouldHaveIDAndName(idx, id, name)
	})
	ctx.Step(`^the service will fail to list with an internal error "([^"]*)"$`, tc.theServiceWillFailToListWithAnInternalError)
	ctx.Step(`^the service will fail to list with a not found error "([^"]*)"$`, tc.theServiceWillFailToListWithANotFoundError)

	// Shared steps
	ctx.Step(`^no error should be returned$`, tc.noErrorShouldBeReturned)
	ctx.Step(`^a gRPC error with code "([^"]*)" should be returned$`, tc.aGRPCErrorWithCodeShouldBeReturned)
}

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
