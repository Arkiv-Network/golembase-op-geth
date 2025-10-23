Feature: changing the owner of an entity

  Scenario: changing the owner of an entity
    Given I have created an entity
    When I submit a transaction to change the owner of the entity
    Then the owner of the entity should be changed
    And the entity owner change log should be recorded

  Scenario: changing the owner of an entity by non-owner
    Given I have created an entity
    When I submit a transaction to change the owner of the entity by non-owner
    Then the transaction should fail
