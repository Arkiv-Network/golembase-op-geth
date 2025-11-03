Feature: Search

  Scenario: finding multiple entities by common string annotation
    Given I have an entity "e1" with string annotations:
      | foo | bar |
    And I have an entity "e2" with string annotations:
      | foo | bar |
    And I have an entity "e3" with string annotations:
      | foo | baz |
    When I search for entities with the string annotation "foo" equal to "bar"
    Then I should find 2 entities

  Scenario: finding multiple entities by common numeric annotation
    Given I have an entity "e1" with numeric annotations:
      | foo | 42 |
    And I have an entity "e2" with numeric annotations:
      | foo | 42 |
    And I have an entity "e3" with numeric annotations:
      | foo | 43 |
    When I search for entities with the numeric annotation "foo" equal to "42"
    Then I should find 2 entities

  Scenario: finding multiple entities with a complex query
    Given I have an entity "e1" with string annotations:
      | foo | bar |
    And I have an entity "e2" with string annotations:
      | foo | bar |
    And I have an entity "e3" with string annotations:
      | foo | baz |
    When I search for entities with the query
      """
      (foo = "bar" || foo = "baz") && foo = "bar"
      """
    Then I should find 2 entities
    When I search for entities with the query
      """
      (foo = "bar" or foo = "baz") and foo = "bar"
      """
    Then I should find 2 entities

  Scenario: invalid query
    When I search for entities with the invalid query
      """
      key = 8e
      """
    Then I should see an error containing "unexpected token"

  Scenario: no extraneous fields in response
    Given I have an entity "e1" with string annotations:
      | foo | bar |
    When I search for entities without requesting columns
    Then the response would be empty

  Scenario: no extraneous fields in response
    Given I have an entity "e1" with string annotations:
      | foo | bar |
    Given I have an entity "e2" with numeric annotations:
      | qux | 5 |
    When I search for all entities
    Then I should find 2 entities

  Scenario: search for entities of an owner
    Given I have created an entity
    When I search for entities of an owner
    Then I should find 1 entity

  Scenario: finding multiple entities with a numeric range query
    Given I have an entity "e1" with numeric annotations:
      | foo | 5 |
    And I have an entity "e2" with numeric annotations:
      | foo | 50 |
    And I have an entity "e3" with numeric annotations:
      | foo | 60 |
    And I have an entity "e4" with numeric annotations:
      | foo | 3 |
    And I have an entity "e5" with numeric annotations:
      | foo | 2 |
    When I search for entities with the query
      """
      foo <= 50 && foo > 3
      """
    Then I should find 2 entities
    When I search for entities with the query
      """
      foo <= 50 and foo > 3
      """
    Then I should find 2 entities

  Scenario: finding multiple entities with a numeric inclusion query
    Given I have an entity "e1" with numeric annotations:
      | foo | 5 |
    And I have an entity "e2" with numeric annotations:
      | foo | 50 |
    And I have an entity "e3" with numeric annotations:
      | foo | 60 |
    And I have an entity "e4" with numeric annotations:
      | foo | 3 |
    And I have an entity "e5" with numeric annotations:
      | foo | 2 |
    When I search for entities with the query
      """
      foo IN (50 3)
      """
    Then I should find 2 entities

  Scenario: finding multiple entities with a numeric inclusion query
    Given I have an entity "e1" with numeric annotations:
      | foo | 5 |
    And I have an entity "e2" with numeric annotations:
      | foo | 50 |
    And I have an entity "e3" with numeric annotations:
      | foo | 60 |
    And I have an entity "e4" with numeric annotations:
      | foo | 3 |
    And I have an entity "e5" with numeric annotations:
      | foo | 2 |
    When I search for entities with the query
      """
      foo NOT IN (50 3)
      """
    Then I should find 3 entities

  Scenario: finding multiple entities with a glob query
    Given I have an entity "e1" with string annotations:
      | foo | bar |
    And I have an entity "e2" with string annotations:
      | foo | foobarquz |
    And I have an entity "e3" with string annotations:
      | foo | fooborquz |
    When I search for entities with the query
      """
      foo ~ "*b?r*"
      """
    Then I should find 3 entities

  Scenario: finding multiple entities with a glob query (bis)
    Given I have an entity "e1" with string annotations:
      | foo | bar |
    And I have an entity "e2" with string annotations:
      | foo | foobarquz |
    And I have an entity "e3" with string annotations:
      | foo | fooborquz |
    And I have an entity "e4" with string annotations:
      | foo | bor |
    When I search for entities with the query
      """
      foo ~ "b?r"
      """
    Then I should find 2 entities

  Scenario: finding multiple entities with a string range query
    Given I have an entity "e1" with string annotations:
      | foo | bar |
    And I have an entity "e2" with string annotations:
      | foo | foobarquz |
    And I have an entity "e3" with string annotations:
      | foo | fooborquz |
    And I have an entity "e4" with string annotations:
      | foo | bor |
    And I have an entity "e5" with string annotations:
      | foo | a |
    When I search for entities with the query
      """
      foo <= "bor"
      """
    Then I should find 3 entities

  Scenario: finding multiple entities with a string inclusion query
    Given I have an entity "e1" with string annotations:
      | foo | bar |
    And I have an entity "e2" with string annotations:
      | foo | foobarquz |
    And I have an entity "e3" with string annotations:
      | foo | fooborquz |
    And I have an entity "e4" with string annotations:
      | foo | bor |
    And I have an entity "e5" with string annotations:
      | foo | a |
    When I search for entities with the query
      """
      foo IN ("bor" "a" "bar")
      """
    Then I should find 3 entities

  Scenario: finding multiple entities with a negative string inclusion query
    Given I have an entity "e1" with string annotations:
      | foo | bar |
    And I have an entity "e2" with string annotations:
      | foo | foobarquz |
    And I have an entity "e3" with string annotations:
      | foo | fooborquz |
    And I have an entity "e4" with string annotations:
      | foo | bor |
    And I have an entity "e5" with string annotations:
      | foo | a |
    When I search for entities with the query
      """
      foo not in ("bor" "a" "bar")
      """
    Then I should find 2 entities

  Scenario: finding entities with a query with negations
    Given I have an entity "e1" with string annotations:
      | foo | bar |
    And I have an entity "e2" with string annotations:
      | foo | foobarquz |
    And I have an entity "e3" with string annotations:
      | foo | fooborquz |
    And I have an entity "e4" with string annotations:
      | foo | bor |
    And I have an entity "e5" with numeric annotations:
      | foo | 5 |
    When I search for entities with the query
      """
      !(foo != "bar" || foo = "foo") && !(foo = "a" || foo ~ "foob?rquz")
      """
    Then I should find 1 entities
    When I search for entities with the query
      """
      not (foo != "bar" or foo = "foo") and not (foo = "a" or foo glob "foob?rquz")
      """
    Then I should find 1 entities
