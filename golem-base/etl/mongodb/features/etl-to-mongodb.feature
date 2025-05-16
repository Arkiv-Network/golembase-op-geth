Feature: ETL to Mongodb

  Scenario: ETL Create to Mongodb
    Given A running Golembase node with WAL enabled
    And A running ETL to Mongodb
    When I create a new entity in Golebase
    Then the entity should be created in the Mongodb database
    And the annotations of the entity should be existing in the Mongodb database

  Scenario: ETL Update to Mongodb
    Given A running Golembase node with WAL enabled
    And A running ETL to Mongodb
    And an existing entity in the Mongodb database
    When update the entity in Golembase
    Then the entity should be updated in the Mongodb database
    And the annotations of the entity should be updated in the Mongodb database

  Scenario: ETL Delete to Mongodb
    Given A running Golembase node with WAL enabled
    And A running ETL to Mongodb
    And an existing entity in the Mongodb database
    When delete the entity in Golembase
    Then the entity should be deleted in the Mongodb database

  Scenario: JSON Payload Deserialization
    Given A running Golembase node with WAL enabled
    And A running ETL to Mongodb
    When I create an entity with a JSON payload to the Golembase
    Then the PayloadAsJSON in the Mongodb database should be populated

  Scenario: Owner Address Tracking
    Given A running Golembase node with WAL enabled
    And A running ETL to Mongodb
    When I create a new entity in Golebase
    Then the entity should be created in the Mongodb database
    And the owner address should be stored in the Mongodb database
    When update the entity in Golembase
    Then the owner address should be preserved in the Mongodb database

  Scenario: Owner Address Preservation on Update
    Given A running Golembase node with WAL enabled
    And A running ETL to Mongodb
    And a new entity in Golebase
    When update the entity in Golembase
    Then the owner address should be preserved in the Mongodb database
    And the entity should be updated in the Mongodb database

  Scenario: ETL BTL Extension
    Given A running Golembase node with WAL enabled
    And A running ETL to Mongodb
    And a new entity in Golebase
    When update the BTL of the entity in Golembase
    Then the BTL of the entity should be extended in the Mongodb database
