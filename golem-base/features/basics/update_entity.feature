
Feature: update entity

  Scenario: updating the payload of the entity
    Given I have created an entity
    When I submit a transaction to update the entity, changing the paylod
    Then the payload of the entity should be changed
    And the entity update log should be recorded

  Scenario: updating the annotations of the entity
    Given I have created an entity
    When I submit a transaction to update the entity, changing the annotations
    Then the annotations of the entity should be changed

  Scenario: updating the btl of the entity
    Given I have created an entity
    When I submit a transaction to update the entity, changing the btl of the entity
    Then the btl of the entity should be changed

  Scenario: updating entity by non-owner
    Given I have created an entity
    When I submit a transaction to update the entity by non-owner
    Then the transaction should fail
