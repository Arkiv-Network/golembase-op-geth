Feature: Extend BTL

  Scenario: Extend BTL
    Given I have created an entity
    When I submit a transaction to extend BTL of the entity by 100 blocks    
    Then the entity's BTL should be extended by 100 blocks
    And the entity extend log should be recorded