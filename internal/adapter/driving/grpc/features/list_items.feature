Feature: List Items via gRPC
  As an API consumer
  I want to list items through the gRPC API
  So that I can retrieve paginated items from the system

  Scenario: Successfully list items
    Given the service has the following items:
      | id   | name  | description |
      | a1   | Alpha | First       |
      | b2   | Beta  | Second      |
    When I list items with page 1 and page_size 10
    Then the response should contain 2 items
    And the total count should be 2
    And item 1 should have id "a1" and name "Alpha"
    And no error should be returned

  Scenario: List items returns empty result
    Given the service has no items
    When I list items with page 1 and page_size 10
    Then the response should contain 0 items
    And the total count should be 0
    And no error should be returned

  Scenario: List items when service fails returns internal error
    Given the service will fail to list with an internal error "database unavailable"
    When I list items with page 1 and page_size 10
    Then a gRPC error with code "Internal" should be returned

  Scenario: List items when service returns not found error
    Given the service will fail to list with a not found error "items not found"
    When I list items with page 1 and page_size 10
    Then a gRPC error with code "NotFound" should be returned
