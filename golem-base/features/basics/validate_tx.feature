Feature: Storage Transaction Validation

  Scenario: Valid transaction with all operation types
    Given I have a storage transaction with create, update, delete, and extend operations
    And all BTL values are greater than 0
    And all annotation keys follow the valid pattern
    And there are no duplicate annotation keys
    When I validate the transaction
    Then the validation should succeed

  Scenario: Create operation with zero BTL
    Given I have a storage transaction with a create operation
    And the create operation has BTL set to 0
    When I validate the transaction
    Then the validation should fail
    And the error should mention "create BTL is 0"

  Scenario: Update operation with zero BTL
    Given I have a storage transaction with an update operation
    And the update operation has BTL set to 0
    When I validate the transaction
    Then the validation should fail
    And the error should mention "update[0] BTL is 0"

  Scenario: Extend operation with zero blocks
    Given I have a storage transaction with an extend operation
    And the extend operation has NumberOfBlocks set to 0
    When I validate the transaction
    Then the validation should fail
    And the error should mention "number of blocks is 0"

  Scenario: Create operation with invalid annotation key starting with dollar sign
    Given I have a storage transaction with a create operation
    And the create operation has a string annotation with key starting with "$"
    When I validate the transaction
    Then the validation should fail
    And the error should mention "invalid annotation identifier"

  Scenario: Create operation with duplicate string annotation keys
    Given I have a storage transaction with a create operation
    And the create operation has duplicate string annotation keys
    When I validate the transaction
    Then the validation should fail
    And the error should mention "string annotation key" and "is duplicated"

  Scenario: Create operation with duplicate numeric annotation keys
    Given I have a storage transaction with a create operation
    And the create operation has duplicate numeric annotation keys
    When I validate the transaction
    Then the validation should fail
    And the error should mention "numeric annotation key" and "is duplicated"

  Scenario: Update operation with duplicate string annotation keys
    Given I have a storage transaction with an update operation
    And the update operation has duplicate string annotation keys
    When I validate the transaction
    Then the validation should fail
    And the error should mention "string annotation key" and "is duplicated"

  Scenario: Update operation with duplicate numeric annotation keys
    Given I have a storage transaction with an update operation
    And the update operation has duplicate numeric annotation keys
    When I validate the transaction
    Then the validation should fail
    And the error should mention "numeric annotation key" and "is duplicated"

  Scenario: Valid annotation keys with various patterns
    Given I have a storage transaction with a create operation
    And the create operation has string annotations with keys "type", "name_with_underscore", "_starts_with_underscore"
    And the create operation has numeric annotations with keys "version123", "size_bytes"
    When I validate the transaction
    Then the validation should succeed

  Scenario: Valid annotation keys with Unicode characters
    Given I have a storage transaction with a create operation
    And the create operation has a string annotation with Unicode key "αβγ"
    When I validate the transaction
    Then the validation should succeed

  Scenario: Invalid annotation key with special characters
    Given I have a storage transaction with a create operation
    And the create operation has a string annotation with key containing special characters like "@" or "#"
    When I validate the transaction
    Then the validation should fail
    And the error should mention "invalid annotation identifier"

  Scenario: Invalid annotation key starting with number
    Given I have a storage transaction with a create operation
    And the create operation has a string annotation with key starting with a number
    When I validate the transaction
    Then the validation should fail
    And the error should mention "invalid annotation identifier"

  Scenario: Empty transaction validation
    Given I have an empty storage transaction
    When I validate the transaction
    Then the validation should succeed

  Scenario: Transaction with mixed valid and invalid operations
    Given I have a storage transaction with multiple create operations
    And one create operation has BTL set to 0
    And another create operation has valid BTL and annotations
    When I validate the transaction
    Then the validation should fail
    And the error should mention "create BTL is 0"

  Scenario: Multiple validation errors in single transaction
    Given I have a storage transaction with a create operation
    And the create operation has BTL set to 0
    And the create operation has duplicate string annotation keys
    When I validate the transaction
    Then the validation should fail
    And the error should mention the first validation error encountered
