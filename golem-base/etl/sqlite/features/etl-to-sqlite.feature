Feature: ETL to SQLite

  Scenario: ETL Create to SQLite
    Given A running Golembase node with WAL enabled
    And A running ETL to SQLite
    When I create a new entity in Golebase
    Then the entity should be created in the SQLite database
    And the annotations of the entity should be existing in the SQLite database

  Scenario: ETL Update to SQLite
    Given A running Golembase node with WAL enabled
    And A running ETL to SQLite
    And an existing entity in the SQLite database
    When update the entity in Golembase
    Then the entity should be updated in the SQLite database
    And the annotations of the entity should be updated in the SQLite database

  Scenario: ETL Delete to SQLite
    Given A running Golembase node with WAL enabled
    And A running ETL to SQLite
    And an existing entity in the SQLite database
    When delete the entity in Golembase
    Then the entity should be deleted in the SQLite database
    And the annotations of the entity should be deleted in the SQLite database

  Scenario: Owner Address is Stored on Creation
    Given A running Golembase node with WAL enabled
    And A running ETL to SQLite
    When I create a new entity in Golebase
    Then the entity should be created in the SQLite database
    And the owner address should be stored in the SQLite database

  Scenario: Owner Address is Preserved on Update
    Given A running Golembase node with WAL enabled
    And A running ETL to SQLite
    And an existing entity in the SQLite database
    When update the entity in Golembase
    Then the owner address should be preserved in the SQLite database

  Scenario: ETL BTL Extension
    Given A running Golembase node with WAL enabled
    And A running ETL to SQLite
    And a new entity in Golebase
    When update the BTL of the entity in Golembase
    Then the BTL of the entity should be extended in the SQLite database
