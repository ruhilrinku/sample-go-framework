Feature: Create Item via gRPC
  As an API consumer
  I want to create items through the gRPC API
  So that new items are persisted and returned with a generated ID

  Scenario: Successfully create an item
    Given the service will return a created item with id "uuid-001"
    When I create an item with name "Widget" and description "A test widget"
    Then the response should contain an item with id "uuid-001"
    And the response item name should be "Widget"
    And the response item description should be "A test widget"
    And no error should be returned

  Scenario: Create item with empty name returns validation error
    When I create an item with name "" and description "Some description"
    Then a gRPC error with code "InvalidArgument" should be returned

  Scenario: Create item when service fails returns internal error
    Given the service will fail to create with an internal error "database connection lost"
    When I create an item with name "Widget" and description "desc"
    Then a gRPC error with code "Internal" should be returned

  Scenario: Create item when service returns validation error
    Given the service will fail to create with a validation error "name too long"
    When I create an item with name "Widget" and description "desc"
    Then a gRPC error with code "InvalidArgument" should be returned
