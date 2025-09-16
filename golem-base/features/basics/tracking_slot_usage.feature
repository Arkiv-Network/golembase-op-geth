Feature: Tracking slot usage

  Scenario: Tracking slot usage
    Given a new Golem Base instance
    When I get the number of used slots
    Then the number of used slots should be 0

  Scenario: Adding an entity
    Given I have created an entity
    When I get the number of used slots
    Then the number of used slots should be 14

  Scenario: Deleting an entity
    Given I have created an entity
    When I delete the entity
    And I get the number of used slots
    Then the number of used slots should be 0

    Scenario: Updating an entity
    Given I have created an entity
    When I update the entity
    And I get the number of used slots
    Then the number of used slots should be 14

    Scenario: Deleting an updated entity
    Given I have created an entity
    When I update the entity
    And I delete the entity
    And I get the number of used slots
    Then the number of used slots should be 0

  Scenario: Expiring an entity
    Given there is an entity that will expire in the next block
    When there is a new block
    And I get the number of used slots
    Then the number of used slots should be 0
