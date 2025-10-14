package golembase_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/golemtype"
	arkivlogs "github.com/ethereum/go-ethereum/golem-base/logs"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/golem-base/testutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/holiman/uint256"
	"github.com/spf13/pflag" // godog v0.11.0 and later
)

var opts = godog.Options{
	Output:      colors.Uncolored(os.Stdout),
	Format:      "progress",
	Strict:      true,
	Concurrency: 4,

	Paths: []string{"features"},
}

func init() {
	godog.BindCommandLineFlags("godog.", &opts)

	if os.Getenv("CUCUMBER_WIP_ONLY") == "true" {
		opts.Tags = "@wip"
		opts.Concurrency = 1
		opts.Format = "pretty"
	}
}

func compileGeth() (string, func(), error) {
	td, err := os.MkdirTemp("", "golem-base")
	if err != nil {
		panic(fmt.Errorf("failed to create temp dir: %w", err))
	}

	gethBinaryPath := filepath.Join(td, "geth")

	cmd := exec.Command("go", "build", "-o", gethBinaryPath, "../cmd/geth")
	out := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = out
	err = cmd.Run()
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to compile geth: %w\n%s", err, out.String())
	}

	return gethBinaryPath, func() {
		os.RemoveAll(td)
	}, nil
}

func TestMain(m *testing.M) {
	pflag.Parse()
	opts.Paths = pflag.Args()

	gethPath, cleanupCompiled, err := compileGeth()
	if err != nil {
		log.Fatal(fmt.Errorf("failed to compile geth: %w", err))
	}

	suite := godog.TestSuite{
		Name: "cucumber",
		ScenarioInitializer: func(sctx *godog.ScenarioContext) {
			InitializeScenario(sctx)
			sctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {

				world, err := testutil.NewWorld(ctx, gethPath)
				if err != nil {
					return ctx, fmt.Errorf("failed to start geth instance: %w", err)
				}

				timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

				sctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
					world.Shutdown()
					cancel()
					return ctx, world.AddLogsToTestError(err)
				})

				return testutil.WithWorld(timeoutCtx, world), nil

			})

		},
		// ScenarioInitializer:  InitializeScenario,
		Options: &opts,
	}

	status := suite.Run()

	// // Optional: Run `testing` package's logic besides godog.
	// if st := m.Run(); st > status {
	// 	status = st
	// }

	cleanupCompiled()

	os.Exit(status)
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^I have enough funds to pay for the transaction$`, iHaveEnoughFundsToPayForTheTransaction)
	ctx.Step(`^submit a transaction to create an entity$`, submitATransactionToCreateAnEntity)
	ctx.Step(`^the entity should be created$`, theEntityShouldBeCreated)
	ctx.Step(`^the expiry of the entity should be recorded$`, theExpiryOfTheEntityShouldBeRecorded)
	ctx.Step(`^I should be able to retrieve the entity by the numeric annotation$`, iShouldBeAbleToRetrieveTheEntityByTheNumericAnnotation)
	ctx.Step(`^I should be able to retrieve the entity by the string annotation$`, iShouldBeAbleToRetrieveTheEntityByTheStringAnnotation)
	ctx.Step(`^I store an entity with a string annotation$`, iStoreAnEntityWithAStringAnnotation)
	ctx.Step(`^I store an entity with a numerical annotation$`, iStoreAnEntityWithANumericalAnnotation)
	ctx.Step(`^I have an entity "([^"]*)" with string annotations:$`, iHaveAnEntityWithStringAnnotations)
	ctx.Step(`^I search for entities with the string annotation "([^"]*)" equal to "([^"]*)"$`, iSearchForEntitiesWithTheStringAnnotationEqualTo)
	ctx.Step(`^I should find (\d+) entit(y|ies)$`, iShouldFindEntity)
	ctx.Step(`^I have an entity "([^"]*)" with numeric annotations:$`, iHaveAnEntityWithNumericAnnotations)
	ctx.Step(`^I search for entities with the numeric annotation "([^"]*)" equal to "([^"]*)"$`, iSearchForEntitiesWithTheNumericAnnotationEqualTo)
	ctx.Step(`^I have created an entity$`, iHaveCreatedAnEntity)
	ctx.Step(`^I submit a transaction to delete the entity$`, iSubmitATransactionToDeleteTheEntity)
	ctx.Step(`^the entity should be deleted$`, theEntityShouldBeDeleted)
	ctx.Step(`^I submit a transaction to update the entity, changing the paylod$`, iSubmitATransactionToUpdateTheEntityChangingThePaylod)
	ctx.Step(`^the payload of the entity should be changed$`, thePayloadOfTheEntityShouldBeChanged)
	ctx.Step(`^I submit a transaction to update the entity, changing the annotations$`, iSubmitATransactionToUpdateTheEntityChangingTheAnnotations)
	ctx.Step(`^the annotations of the entity should be changed$`, theAnnotationsOfTheEntityShouldBeChanged)
	ctx.Step(`^I submit a transaction to update the entity, changing the btl of the entity$`, iSubmitATransactionToUpdateTheEntityChangingTheBtlOfTheEntity)
	ctx.Step(`^the btl of the entity should be changed$`, theBtlOfTheEntityShouldBeChanged)
	ctx.Step(`^submit a transaction to create an entity of (\d+)K$`, submitATransactionToCreateAnEntityOfK)
	ctx.Step(`^the entity creation should not fail$`, theEntityCreationShouldNotFail)
	ctx.Step(`^I search for entities with the query$`, iSearchForEntitiesWithTheQuery)
	ctx.Step(`^the housekeeping transaction should be submitted$`, theHousekeepingTransactionShouldBeSubmitted)
	ctx.Step(`^the housekeeping transaction should be successful$`, theHousekeepingTransactionShouldBeSuccessful)
	ctx.Step(`^there is a new block$`, thereIsANewBlock)
	ctx.Step(`^the expired entity should be deleted$`, theExpiredEntityShouldBeDeleted)
	ctx.Step(`^there is an entity that will expire in the next block$`, thereIsAnEntityThatWillExpireInTheNextBlock)
	ctx.Step(`^the number of entities should be (\d+)$`, theNumberOfEntitiesShouldBe)
	ctx.Step(`^the entity should be in the list of all entities$`, theEntityShouldBeInTheListOfAllEntities)
	ctx.Step(`^the list of all entities should be empty$`, theListOfAllEntitiesShouldBeEmpty)
	ctx.Step(`^I search for entities with the invalid query$`, iSearchForEntitiesWithTheInvalidQuery)
	ctx.Step(`^I should see an error containing "([^"]*)"$`, iShouldSeeAnErrorContaining)
	ctx.Step(`^the entity should be in the list of entities of the owner$`, theEntityShouldBeInTheListOfEntitiesOfTheOwner)
	ctx.Step(`^the sender should be the owner of the entity$`, theSenderShouldBeTheOwnerOfTheEntity)
	ctx.Step(`^the owner should not have any entities$`, theOwnerShouldNotHaveAnyEntities)
	ctx.Step(`^I submit a transaction to extend BTL of the entity by (\d+) blocks$`, iSubmitATransactionToExtendBTLOfTheEntityByBlocks)
	ctx.Step(`^the entity\'s BTL should be extended by (\d+) blocks$`, theEntitysBTLShouldBeExtendedByBlocks)
	ctx.Step(`^I submit a transaction to delete the entity by non-owner$`, iSubmitATransactionToDeleteTheEntityByNonowner)
	ctx.Step(`^the transaction should fail$`, theTransactionShouldFail)
	ctx.Step(`^I submit a transaction to update the entity by non-owner$`, iSubmitATransactionToUpdateTheEntityByNonowner)
	ctx.Step(`^the expired entities should be deleted$`, theExpiredEntitiesShouldBeDeleted)
	ctx.Step(`^there are two entities that will expire in the next block$`, thereAreTwoEntitiesThatWillExpireInTheNextBlock)
	ctx.Step(`^I search for entities of an owner$`, iSearchForEntitiesOfAnOwner)
	ctx.Step(`^a new Golem Base instance$`, aNewGolemBaseInstance)
	ctx.Step(`^I delete the entity$`, iDeleteTheEntity)
	ctx.Step(`^I get the number of used slots$`, iGetTheNumberOfUsedSlots)
	ctx.Step(`^the number of used slots should be (\d+)$`, theNumberOfUsedSlotsShouldBe)
	ctx.Step(`^I update the entity$`, iUpdateTheEntity)
	ctx.Step(`^I trace the transaction that created the entity$`, iTraceTheTransactionThatCreatedTheEntity)
	ctx.Step(`^the trace should be empty$`, theTraceShouldBeEmpty)

	ctx.Step(`^the entity update log should be recorded$`, theEntityUpdateLogShouldBeRecorded)
	ctx.Step(`^the entity delete log should be recorded$`, theEntityDeleteLogShouldBeRecorded)
	ctx.Step(`^the entity extend log should be recorded$`, theEntityExtendLogShouldBeRecorded)

	// Storage Transaction Validation Steps
	ctx.Step(`^I have a storage transaction with create, update, delete, and extend operations$`, iHaveAStorageTransactionWithCreateUpdateDeleteAndExtendOperations)
	ctx.Step(`^all BTL values are greater than (\d+)$`, allBTLValuesAreGreaterThan)
	ctx.Step(`^all annotation keys follow the valid pattern$`, allAnnotationKeysFollowTheValidPattern)
	ctx.Step(`^there are no duplicate annotation keys$`, thereAreNoDuplicateAnnotationKeys)
	ctx.Step(`^I validate the transaction$`, iValidateTheTransaction)
	ctx.Step(`^the validation should succeed$`, theValidationShouldSucceed)
	ctx.Step(`^the validation should fail$`, theValidationShouldFail)
	ctx.Step(`^I have a storage transaction with a create operation$`, iHaveAStorageTransactionWithACreateOperation)
	ctx.Step(`^the create operation has BTL set to (\d+)$`, theCreateOperationHasBTLSetTo)
	ctx.Step(`^the error should mention "([^"]*)"$`, theErrorShouldMention)
	ctx.Step(`^I have a storage transaction with an update operation$`, iHaveAStorageTransactionWithAnUpdateOperation)
	ctx.Step(`^the update operation has BTL set to (\d+)$`, theUpdateOperationHasBTLSetTo)
	ctx.Step(`^I have a storage transaction with an extend operation$`, iHaveAStorageTransactionWithAnExtendOperation)
	ctx.Step(`^the extend operation has NumberOfBlocks set to (\d+)$`, theExtendOperationHasNumberOfBlocksSetTo)
	ctx.Step(`^the create operation has a string annotation with key starting with "([^"]*)"$`, theCreateOperationHasAStringAnnotationWithKeyStartingWith)
	ctx.Step(`^the create operation has duplicate string annotation keys$`, theCreateOperationHasDuplicateStringAnnotationKeys)
	ctx.Step(`^the create operation has duplicate numeric annotation keys$`, theCreateOperationHasDuplicateNumericAnnotationKeys)
	ctx.Step(`^the update operation has duplicate string annotation keys$`, theUpdateOperationHasDuplicateStringAnnotationKeys)
	ctx.Step(`^the update operation has duplicate numeric annotation keys$`, theUpdateOperationHasDuplicateNumericAnnotationKeys)
	ctx.Step(`^the create operation has string annotations with keys "([^"]*)", "([^"]*)", "([^"]*)"$`, theCreateOperationHasStringAnnotationsWithKeys)
	ctx.Step(`^the create operation has numeric annotations with keys "([^"]*)", "([^"]*)"$`, theCreateOperationHasNumericAnnotationsWithKeys)
	ctx.Step(`^the create operation has a string annotation with Unicode key "([^"]*)"$`, theCreateOperationHasAStringAnnotationWithUnicodeKey)
	ctx.Step(`^the create operation has a string annotation with key containing special characters like "([^"]*)" or "([^"]*)"$`, theCreateOperationHasAStringAnnotationWithKeyContainingSpecialCharactersLikeOr)
	ctx.Step(`^the create operation has a string annotation with key starting with a number$`, theCreateOperationHasAStringAnnotationWithKeyStartingWithANumber)
	ctx.Step(`^I have an empty storage transaction$`, iHaveAnEmptyStorageTransaction)
	ctx.Step(`^I have a storage transaction with multiple create operations$`, iHaveAStorageTransactionWithMultipleCreateOperations)
	ctx.Step(`^one create operation has BTL set to (\d+)$`, oneCreateOperationHasBTLSetTo)
	ctx.Step(`^another create operation has valid BTL and annotations$`, anotherCreateOperationHasValidBTLAndAnnotations)
	ctx.Step(`^the error should mention "([^"]*)" and "([^"]*)"$`, theErrorShouldMentionAnd)
	ctx.Step(`^the error should mention the first validation error encountered$`, theErrorShouldMentionTheFirstValidationErrorEncountered)
	ctx.Step(`^I submit a storage transaction with no playload$`, iSubmitAStorageTransactionWithNoPlayload)
	ctx.Step(`^I submit a storage transaction with unparseable data$`, iSubmitAStorageTransactionWithUnparseableData)

}

func iSearchForEntitiesWithTheInvalidQuery(ctx context.Context, query *godog.DocString) error {
	w := testutil.GetWorld(ctx)

	err := w.GethInstance.RPCClient.CallContext(
		ctx,
		nil,
		"golembase_queryEntities",
		query.Content,
	)

	w.LastError = err

	return nil
}

func iShouldSeeAnErrorContaining(ctx context.Context, expectedSubstring string) error {
	w := testutil.GetWorld(ctx)

	if w.LastError == nil {
		return fmt.Errorf("no error occurred")
	}

	if !strings.Contains(w.LastError.Error(), expectedSubstring) {
		return fmt.Errorf("error %w does not contain expected substring: %s", w.LastError, expectedSubstring)
	}

	return nil
}

func iHaveEnoughFundsToPayForTheTransaction(ctx context.Context) error {
	return nil
}

func submitATransactionToCreateAnEntity(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	receipt, err := w.CreateEntity(
		ctx,
		100,
		[]byte("test payload"),
		[]entity.StringAnnotation{
			{
				Key:   "test_key",
				Value: "test_value",
			},
		},
		[]entity.NumericAnnotation{
			{
				Key:   "test_number",
				Value: 42,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	w.CreatedEntityKey = receipt.Logs[0].Topics[1]

	return nil

}

func hashToAddress(hash common.Hash) common.Address {
	return common.Address(hash[12:])
}

func theEntityShouldBeCreated(ctx context.Context) error {

	w := testutil.GetWorld(ctx)
	receipt := w.LastReceipt

	if len(receipt.Logs) == 0 {
		return fmt.Errorf("no logs found in receipt")
	}

	logs := receipt.Logs

	if len(logs) != 2 {
		return fmt.Errorf("expected 2 logs, got %d", len(logs))
	}

	oldCreatedLog := logs[0]

	if oldCreatedLog.Topics[0] != storagetx.GolemBaseStorageEntityCreated {
		return fmt.Errorf("expected GolemBaseStorageEntityCreated log, got %s", oldCreatedLog.Topics[0])
	}

	oldLogData := oldCreatedLog.Data

	if len(oldLogData) != 32 {
		return fmt.Errorf("expected old log data to be 32 bytes, got %d", len(oldLogData))
	}

	expiresAtBlockU256 := uint256.NewInt(0).SetBytes(oldLogData[:32])
	oldExpiresAtBlock := expiresAtBlockU256.Uint64()

	expiresAtBlockExpected := receipt.BlockNumber.Uint64() + 100

	if oldExpiresAtBlock != expiresAtBlockExpected {
		return fmt.Errorf("expected expires at block to be %d, got %d", expiresAtBlockExpected, oldExpiresAtBlock)
	}

	newCreatedLog := logs[1]

	if newCreatedLog.Topics[0] != arkivlogs.ArkivEntityCreated {
		return fmt.Errorf("expected ArkivEntityCreated log, got %s", newCreatedLog.Topics[0])
	}

	if newCreatedLog.Topics[1] != w.CreatedEntityKey {
		return fmt.Errorf("expected arkiv created entity key to be %s, got %s", w.CreatedEntityKey, newCreatedLog.Topics[1])
	}

	newLogData := newCreatedLog.Data

	if len(newLogData) != 64 {
		return fmt.Errorf("expected new log data to be 64 bytes, got %d", len(newLogData))
	}

	newExpiresAtBlockU256 := uint256.NewInt(0).SetBytes(newLogData[:32])
	newExpiresAtBlock := newExpiresAtBlockU256.Uint64()

	if newExpiresAtBlock != expiresAtBlockExpected {
		return fmt.Errorf("expected archiv expires at block to be %d, got %d", expiresAtBlockExpected, newExpiresAtBlock)
	}

	owner := hashToAddress(newCreatedLog.Topics[2])

	if owner != w.FundedAccount.Address {
		return fmt.Errorf("expected owner to be %s, got %s", w.FundedAccount.Address.Hex(), owner.Hex())
	}

	key := receipt.Logs[0].Topics[1]

	var v []byte

	rcpClient := w.GethInstance.RPCClient

	err := rcpClient.CallContext(
		ctx,
		&v,
		"golembase_getStorageValue",
		key.Hex(),
	)
	if err != nil {
		return fmt.Errorf("failed to get storage value: %w", err)
	}

	if string(v) != "test payload" {
		return fmt.Errorf("unexpected storage value: %s", string(v))
	}

	return nil

}

func theExpiryOfTheEntityShouldBeRecorded(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	receipt := w.LastReceipt

	toExpire := []common.Hash{}

	rcpClient := w.GethInstance.RPCClient

	if len(receipt.Logs) == 0 {
		return fmt.Errorf("no logs found in receipt")
	}

	blockNumber256 := uint256.NewInt(0).SetBytes(receipt.Logs[0].Data)

	err := rcpClient.CallContext(
		ctx,
		&toExpire,
		"golembase_getEntitiesToExpireAtBlock",
		blockNumber256.Uint64(),
	)
	if err != nil {
		return fmt.Errorf("failed to get entities to expire: %w", err)
	}

	key := receipt.Logs[0].Topics[1]

	if len(toExpire) != 1 {
		return fmt.Errorf("unexpected number of entities to expire: %d (expected 1)", len(toExpire))
	}

	if toExpire[0] != key {
		return fmt.Errorf("unexpected entity to expire: %s (expected %s)", toExpire[0].Hex(), key.Hex())
	}

	return nil
}

func iShouldBeAbleToRetrieveTheEntityByTheStringAnnotation(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	receipt := w.LastReceipt

	toExpire := []common.Hash{}

	rcpClient := w.GethInstance.RPCClient

	err := rcpClient.CallContext(
		ctx,
		&toExpire,
		"golembase_getEntitiesForStringAnnotationValue",
		"test_key",
		"test_value",
	)
	if err != nil {
		return fmt.Errorf("failed to get entities by string anotation: %w", err)
	}

	key := receipt.Logs[0].Topics[1]

	if len(toExpire) != 1 {
		return fmt.Errorf("unexpected number of entities retrieved: %d (expected 1)", len(toExpire))
	}

	if toExpire[0] != key {
		return fmt.Errorf("unexpected retrieved entity: %s (expected %s)", toExpire[0].Hex(), key.Hex())
	}

	return nil
}

func iShouldBeAbleToRetrieveTheEntityByTheNumericAnnotation(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	receipt := w.LastReceipt

	toExpire := []common.Hash{}

	rcpClient := w.GethInstance.RPCClient

	err := rcpClient.CallContext(
		ctx,
		&toExpire,
		"golembase_getEntitiesForNumericAnnotationValue",
		"test_number",
		42,
	)
	if err != nil {
		return fmt.Errorf("failed to get entities to by numeric annotation: %w", err)
	}

	key := receipt.Logs[0].Topics[1]

	if len(toExpire) != 1 {
		return fmt.Errorf("unexpected number of entities to retrieved: %d (expected 1)", len(toExpire))
	}

	if toExpire[0] != key {
		return fmt.Errorf("unexpected retrieved entity: %s (expected %s)", toExpire[0].Hex(), key.Hex())
	}

	return nil
}

func iStoreAnEntityWithAStringAnnotation(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	_, err := w.CreateEntity(
		ctx,
		100,
		[]byte("test payload"),
		[]entity.StringAnnotation{
			{
				Key:   "test_key",
				Value: "test_value",
			},
		},
		nil,
	)

	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	return nil

}

func iStoreAnEntityWithANumericalAnnotation(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	_, err := w.CreateEntity(
		ctx,
		100,
		[]byte("test payload"),
		[]entity.StringAnnotation{},
		[]entity.NumericAnnotation{
			{
				Key:   "test_number",
				Value: 42,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	return nil

}

func iHaveAnEntityWithStringAnnotations(ctx context.Context, payload string, annotationsTable *godog.Table) error {
	w := testutil.GetWorld(ctx)

	stringAnnotations := []entity.StringAnnotation{}

	for _, row := range annotationsTable.Rows {
		stringAnnotations = append(stringAnnotations, entity.StringAnnotation{
			Key:   row.Cells[0].Value,
			Value: row.Cells[1].Value,
		})
	}

	_, err := w.CreateEntity(
		ctx,
		100,
		[]byte("test payload"),
		stringAnnotations,
		[]entity.NumericAnnotation{},
	)

	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	return nil
}

func iSearchForEntitiesWithTheStringAnnotationEqualTo(ctx context.Context, key, value string) error {
	w := testutil.GetWorld(ctx)

	res := []golemtype.SearchResult{}

	rcpClient := w.GethInstance.RPCClient

	err := rcpClient.CallContext(
		ctx,
		&res,
		"golembase_queryEntities",
		fmt.Sprintf(`%s="%s"`, key, value),
	)
	if err != nil {
		return fmt.Errorf("failed to get entities to by numeric annotation: %w", err)
	}

	w.SearchResult = res

	return nil

}

func iShouldFindEntity(ctx context.Context, count int) error {
	w := testutil.GetWorld(ctx)

	if len(w.SearchResult) != count {
		return fmt.Errorf("unexpected number of entities retrieved: %d (expected %d)", len(w.SearchResult), count)
	}

	return nil
}

func iHaveAnEntityWithNumericAnnotations(ctx context.Context, payload string, annotationsTable *godog.Table) error {
	w := testutil.GetWorld(ctx)

	numericAnnotations := []entity.NumericAnnotation{}

	for _, row := range annotationsTable.Rows {
		val, err := strconv.ParseUint(row.Cells[1].Value, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse numeric value: %w", err)
		}
		numericAnnotations = append(numericAnnotations, entity.NumericAnnotation{
			Key:   row.Cells[0].Value,
			Value: val,
		})
	}

	_, err := w.CreateEntity(
		ctx,
		100,
		[]byte("test payload"),
		[]entity.StringAnnotation{},
		numericAnnotations,
	)

	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	return nil
}

func iSearchForEntitiesWithTheNumericAnnotationEqualTo(ctx context.Context, key string, valueString string) error {
	w := testutil.GetWorld(ctx)

	res := []golemtype.SearchResult{}

	rcpClient := w.GethInstance.RPCClient

	value, err := strconv.ParseUint(valueString, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse numeric value: %w", err)
	}

	err = rcpClient.CallContext(
		ctx,
		&res,
		"golembase_queryEntities",
		fmt.Sprintf(`%s=%d`, key, value),
	)
	if err != nil {
		return fmt.Errorf("failed to get entities to by numeric annotation: %w", err)
	}

	w.SearchResult = res

	return nil

}

func iHaveCreatedAnEntity(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	receipt, err := w.CreateEntity(
		ctx,
		100,
		[]byte("test payload"),
		[]entity.StringAnnotation{
			{
				Key:   "test_key",
				Value: "test_value",
			},
		},
		[]entity.NumericAnnotation{
			{
				Key:   "test_number",
				Value: 42,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	if len(receipt.Logs) == 0 {
		return fmt.Errorf("no logs found in receipt")
	}

	key := receipt.Logs[0].Topics[1]

	w.CreatedEntityKey = key

	return nil
}

func iSubmitATransactionToDeleteTheEntity(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	_, err := w.DeleteEntity(
		ctx,
		w.CreatedEntityKey,
	)

	if err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}

	return nil
}

func theEntityShouldBeDeleted(ctx context.Context) error {

	w := testutil.GetWorld(ctx)
	receipt := w.LastReceipt

	if len(receipt.Logs) == 0 {
		return fmt.Errorf("no logs found in receipt")
	}

	return nil
}

func iSubmitATransactionToUpdateTheEntityChangingThePaylod(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	_, err := w.UpdateEntity(
		ctx,
		w.CreatedEntityKey,
		100,
		[]byte("new payload"),
		[]entity.StringAnnotation{
			{
				Key:   "test_key",
				Value: "test_value",
			},
		},
		[]entity.NumericAnnotation{
			{
				Key:   "test_number",
				Value: 42,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to update entity: %w", err)
	}

	return nil

}

func thePayloadOfTheEntityShouldBeChanged(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	rpcClient := w.GethInstance.RPCClient

	var v []byte

	err := rpcClient.CallContext(
		ctx,
		&v,
		"golembase_getStorageValue",
		w.CreatedEntityKey,
	)
	if err != nil {
		return fmt.Errorf("failed to get storage value: %w", err)
	}

	if string(v) != "new payload" {
		return fmt.Errorf("unexpected storage value: %s", string(v))
	}

	return nil

}

func iSubmitATransactionToUpdateTheEntityChangingTheAnnotations(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	_, err := w.UpdateEntity(
		ctx,
		w.CreatedEntityKey,
		100,
		[]byte("new payload"),
		[]entity.StringAnnotation{
			{
				Key:   "test_key1",
				Value: "test_value1",
			},
		},
		[]entity.NumericAnnotation{
			{
				Key:   "test_number1",
				Value: 43,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to update entity: %w", err)
	}

	return nil

}

func theAnnotationsOfTheEntityShouldBeChanged(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	rpcClient := w.GethInstance.RPCClient

	res := []golemtype.SearchResult{}

	err := rpcClient.CallContext(
		ctx,
		&res,
		"golembase_queryEntities",
		`test_key1="test_value1" && test_number1=43`,
	)
	if err != nil {
		return fmt.Errorf("failed to get entities to by numeric annotation: %w", err)
	}

	if len(res) == 0 {
		return fmt.Errorf("could not find any result when searching by new annotations")
	}

	if res[0].Key != w.CreatedEntityKey {
		return fmt.Errorf("expected entity hash %s but got %s", w.CreatedEntityKey.Hex(), res[0].Key.Hex())
	}

	return nil
}

func iSubmitATransactionToUpdateTheEntityChangingTheBtlOfTheEntity(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	_, err := w.UpdateEntity(
		ctx,
		w.CreatedEntityKey,
		200,
		[]byte("new payload"),
		[]entity.StringAnnotation{
			{
				Key:   "test_key",
				Value: "test_value",
			},
		},
		[]entity.NumericAnnotation{
			{
				Key:   "test_number",
				Value: 42,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to update entity: %w", err)
	}

	return nil
}

func theBtlOfTheEntityShouldBeChanged(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	receipt := w.LastReceipt

	toExpire := []common.Hash{}

	rcpClient := w.GethInstance.RPCClient

	err := rcpClient.CallContext(
		ctx,
		&toExpire,
		"golembase_getEntitiesToExpireAtBlock",
		receipt.BlockNumber.Uint64()+200,
	)
	if err != nil {
		return fmt.Errorf("failed to get entities to expire: %w", err)
	}

	key := receipt.Logs[0].Topics[1]

	if len(toExpire) != 1 {
		return fmt.Errorf("unexpected number of entities to expire: %d (expected 1)", len(toExpire))
	}

	if toExpire[0] != key {
		return fmt.Errorf("unexpected entity to expire: %s (expected %s)", toExpire[0].Hex(), key.Hex())
	}

	return nil
}

func submitATransactionToCreateAnEntityOfK(ctx context.Context, kilobytes int) error {

	w := testutil.GetWorld(ctx)

	payload := make([]byte, 1024*kilobytes)

	_, err := w.CreateEntity(
		ctx,
		200,
		payload,
		[]entity.StringAnnotation{
			{
				Key:   "test_key",
				Value: "test_value",
			},
		},
		[]entity.NumericAnnotation{
			{
				Key:   "test_number",
				Value: 42,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to update entity: %w", err)
	}

	return nil

}

func theEntityCreationShouldNotFail(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	if w.LastReceipt.Status == types.ReceiptStatusFailed {
		return fmt.Errorf("tx has failed")
	}
	return nil
}

func iSearchForEntitiesWithTheQuery(ctx context.Context, queryDoc *godog.DocString) error {
	w := testutil.GetWorld(ctx)

	res := []golemtype.SearchResult{}

	rcpClient := w.GethInstance.RPCClient

	err := rcpClient.CallContext(
		ctx,
		&res,
		"golembase_queryEntities",
		queryDoc.Content,
	)
	if err != nil {
		return fmt.Errorf("failed to get entities to by numeric annotation: %w", err)
	}

	w.SearchResult = res

	return nil
}

func theHousekeepingTransactionShouldBeSubmitted(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	ec := w.GethInstance.ETHClient

	lastBlock, err := ec.BlockByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get last block: %w", err)
	}

	firstTx := lastBlock.Transactions()[0]

	if firstTx.Type() != types.DepositTxType {
		return fmt.Errorf("expected deposit transaction but got %d", firstTx.Type())
	}

	return nil
}

func theHousekeepingTransactionShouldBeSuccessful(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	ec := w.GethInstance.ETHClient

	lastBlock, err := ec.BlockByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get last block: %w", err)
	}

	h := lastBlock.Hash()

	receipts, err := ec.BlockReceipts(ctx, rpc.BlockNumberOrHash{
		BlockHash: &h,
	})
	if err != nil {
		return fmt.Errorf("failed to get receipts: %w", err)
	}

	firstTx := receipts[0]

	if firstTx.Status == types.ReceiptStatusFailed {
		return fmt.Errorf("tx has failed")
	}

	return nil
}

func thereIsANewBlock(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	_, err := w.Transfer(
		ctx,
		big.NewInt(1000000000000000000),
		w.FundedAccount.Address,
	)

	if err != nil {
		return fmt.Errorf("failed to transfer funds: %w", err)
	}

	return nil
}

func thereIsAnEntityThatWillExpireInTheNextBlock(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	receipt, err := w.CreateEntity(
		ctx,
		1,
		[]byte("test payload"),
		[]entity.StringAnnotation{
			{
				Key:   "test_key",
				Value: "test_value",
			},
		},
		[]entity.NumericAnnotation{
			{
				Key:   "test_number",
				Value: 42,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	if len(receipt.Logs) == 0 {
		return fmt.Errorf("no logs found in receipt")
	}

	key := receipt.Logs[0].Topics[1]

	w.CreatedEntityKey = key

	return nil
}

func theExpiredEntityShouldBeDeleted(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	ec := w.GethInstance.ETHClient

	lastBlock, err := ec.BlockByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get last block: %w", err)
	}

	h := lastBlock.Hash()

	receipts, err := ec.BlockReceipts(ctx, rpc.BlockNumberOrHash{
		BlockHash: &h,
	})
	if err != nil {
		return fmt.Errorf("failed to get receipts: %w", err)
	}

	firstTx := receipts[0]

	if firstTx.Status == types.ReceiptStatusFailed {
		return fmt.Errorf("housekeeping tx has failed")
	}

	if len(firstTx.Logs) == 0 {
		return fmt.Errorf("no logs found in housekeeping tx")
	}

	key := firstTx.Logs[0].Topics[1]

	if key != w.CreatedEntityKey {
		return fmt.Errorf("expected entity to be deleted but got %s", key.Hex())
	}

	return nil
}

func theNumberOfEntitiesShouldBe(ctx context.Context, expected int) error {
	w := testutil.GetWorld(ctx)

	var count uint64
	err := w.GethInstance.RPCClient.CallContext(ctx, &count, "golembase_getEntityCount")
	if err != nil {
		return fmt.Errorf("failed to get entity count: %w", err)
	}

	if int(count) != expected {
		return fmt.Errorf("expected %d entities, but got %d", expected, count)
	}

	return nil

}

func theEntityShouldBeInTheListOfAllEntities(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	var entityKeys []common.Hash
	err := w.GethInstance.RPCClient.CallContext(ctx, &entityKeys, "golembase_getAllEntityKeys")
	if err != nil {
		return fmt.Errorf("failed to get all entity keys: %w", err)
	}

	found := false
	for _, key := range entityKeys {
		if key == w.CreatedEntityKey {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("entity with key %s not found in the list of all entities", w.CreatedEntityKey.Hex())
	}

	return nil
}

func theListOfAllEntitiesShouldBeEmpty(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	var entityKeys []common.Hash
	err := w.GethInstance.RPCClient.CallContext(ctx, &entityKeys, "golembase_getAllEntityKeys")
	if err != nil {
		return fmt.Errorf("failed to get all entity keys: %w", err)
	}

	if len(entityKeys) != 0 {
		return fmt.Errorf("expected empty list of entities, but got %d entities", len(entityKeys))
	}

	return nil
}

func theEntityShouldBeInTheListOfEntitiesOfTheOwner(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	var entityKeys []common.Hash
	err := w.GethInstance.RPCClient.CallContext(ctx, &entityKeys, "golembase_getEntitiesOfOwner", w.FundedAccount.Address)
	if err != nil {
		return fmt.Errorf("failed to get entities of owner: %w", err)
	}

	found := false
	for _, key := range entityKeys {
		if key == w.CreatedEntityKey {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("entity with key %s not found in the list of entities of the owner", w.CreatedEntityKey.Hex())
	}

	return nil
}

func theSenderShouldBeTheOwnerOfTheEntity(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	var ap entity.EntityMetaData

	err := w.GethInstance.RPCClient.CallContext(ctx, &ap, "golembase_getEntityMetaData", w.CreatedEntityKey.Hex())
	if err != nil {
		return fmt.Errorf("failed to get entity metadata: %w", err)
	}

	if ap.Owner != w.FundedAccount.Address {
		return fmt.Errorf("expected owner to be %s, but got %s", w.FundedAccount.Address.Hex(), ap.Owner.Hex())
	}

	return nil
}

func theOwnerShouldNotHaveAnyEntities(ctx context.Context) error {

	w := testutil.GetWorld(ctx)

	var entityKeys []common.Hash

	err := w.GethInstance.RPCClient.CallContext(ctx, &entityKeys, "golembase_getEntitiesOfOwner", w.FundedAccount.Address)
	if err != nil {
		return fmt.Errorf("failed to get entity metadata: %w", err)
	}

	if len(entityKeys) != 0 {
		return fmt.Errorf("expected 0 entities, but got %d", len(entityKeys))
	}

	return nil

}

func iSubmitATransactionToExtendBTLOfTheEntityByBlocks(ctx context.Context, blockCount int) error {
	w := testutil.GetWorld(ctx)

	_, err := w.ExtendBTL(
		ctx,
		w.CreatedEntityKey,
		uint64(blockCount),
	)

	if err != nil {
		return fmt.Errorf("failed to extend BTL: %w", err)
	}

	return nil
}

func theEntitysBTLShouldBeExtendedByBlocks(ctx context.Context, numberOfBlocks int) error {
	w := testutil.GetWorld(ctx)

	if w.LastReceipt == nil {
		return fmt.Errorf("no transaction receipt found")
	}

	if len(w.LastReceipt.Logs) == 0 {
		return fmt.Errorf("no logs found in transaction receipt")
	}

	key := w.LastReceipt.Logs[0].Topics[1]

	if key != w.CreatedEntityKey {
		return fmt.Errorf("expected entity key to be %s, but got %s", w.CreatedEntityKey.Hex(), key.Hex())
	}

	oldExpiresAtBlock := new(big.Int).SetBytes(w.LastReceipt.Logs[0].Data[:32])
	newExpiresAtBlock := new(big.Int).SetBytes(w.LastReceipt.Logs[0].Data[32:])

	if oldExpiresAtBlock.Uint64()+uint64(numberOfBlocks) != newExpiresAtBlock.Uint64() {
		return fmt.Errorf("expected entity to expire at block %d, but got %d", oldExpiresAtBlock.Uint64()+uint64(numberOfBlocks), newExpiresAtBlock.Uint64())
	}

	return nil
}

func iSubmitATransactionToDeleteTheEntityByNonowner(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	_, err := w.DeleteEntityFromSecondAccount(
		ctx,
		w.CreatedEntityKey,
	)
	if err != nil {
		return fmt.Errorf("failed to send tx to delete entity: %w", err)
	}

	return nil
}

func theTransactionShouldFail(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	if w.LastReceipt == nil {
		return fmt.Errorf("no transaction receipt found")
	}

	if w.LastReceipt.Status != types.ReceiptStatusFailed {
		return fmt.Errorf("expected transaction to fail, but it succeeded")
	}

	return nil
}

func iSubmitATransactionToUpdateTheEntityByNonowner(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	_, err := w.UpdateEntityBySecondAccount(
		ctx,
		w.CreatedEntityKey,
		100,
		[]byte("new payload"),
		[]entity.StringAnnotation{
			{Key: "test_key", Value: "test_value"},
		},
		[]entity.NumericAnnotation{},
	)
	if err != nil {
		return fmt.Errorf("failed to send tx to update entity: %w", err)
	}

	return nil
}

func thereAreTwoEntitiesThatWillExpireInTheNextBlock(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	receipt1, err := w.CreateEntity(
		ctx,
		2,
		[]byte("test payload"),
		[]entity.StringAnnotation{
			{
				Key:   "test_key",
				Value: "test_value",
			},
		},
		[]entity.NumericAnnotation{
			{
				Key:   "test_number",
				Value: 42,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to create first entity: %w", err)
	}

	key1 := receipt1.Logs[0].Topics[1]
	w.CreatedEntityKey = key1

	receipt2, err := w.CreateEntity(
		ctx,
		1,
		[]byte("test payload2"),
		[]entity.StringAnnotation{
			{
				Key:   "test_key",
				Value: "test_value2",
			},
		},
		[]entity.NumericAnnotation{
			{
				Key:   "test_number",
				Value: 42,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to create first entity: %w", err)
	}

	key2 := receipt2.Logs[0].Topics[1]

	w.SecondCreatedEntityKey = key2

	return nil
}

func theExpiredEntitiesShouldBeDeleted(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	entities := []common.Hash{}

	rcpClient := w.GethInstance.RPCClient

	err := rcpClient.CallContext(
		ctx,
		&entities,
		"golembase_getEntitiesOfOwner",
		w.FundedAccount.Address,
	)
	if err != nil {
		return fmt.Errorf("failed to get entities of owner: %w", err)
	}

	if len(entities) != 0 {
		return fmt.Errorf("expected 0 entities, but got %d", len(entities))
	}

	return nil

}

func iSearchForEntitiesOfAnOwner(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	res := []golemtype.SearchResult{}

	err := w.GethInstance.RPCClient.CallContext(ctx, &res, "golembase_queryEntities", fmt.Sprintf(`$owner="%s"`, w.FundedAccount.Address.Hex()))
	if err != nil {
		return fmt.Errorf("failed to get entities of owner: %w", err)
	}

	w.SearchResult = res

	return nil
}

func aNewGolemBaseInstance(ctx context.Context) error {
	// The Golem Base instance is already set up in the test framework
	return nil
}

func iDeleteTheEntity(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	_, err := w.DeleteEntity(
		ctx,
		w.CreatedEntityKey,
	)

	if err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}

	return nil
}

func iGetTheNumberOfUsedSlots(ctx context.Context) error {
	// This step just triggers the action - actual checking happens in the verification step
	return nil
}

func theNumberOfUsedSlotsShouldBe(ctx context.Context, expected int) error {
	w := testutil.GetWorld(ctx)

	var usedSlots hexutil.Big
	err := w.GethInstance.RPCClient.CallContext(ctx, &usedSlots, "golembase_getNumberOfUsedSlots")
	if err != nil {
		return fmt.Errorf("failed to get used slots: %w", err)
	}

	if int(usedSlots.ToInt().Int64()) != expected {
		return fmt.Errorf("expected %d used slots, but got %d", expected, usedSlots.ToInt().Int64())
	}

	return nil
}

func iUpdateTheEntity(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	_, err := w.UpdateEntity(
		ctx,
		w.CreatedEntityKey,
		100,
		[]byte("updated payload"),
		[]entity.StringAnnotation{
			{
				Key:   "updated_key",
				Value: "updated_value",
			},
		},
		[]entity.NumericAnnotation{
			{
				Key:   "updated_number",
				Value: 99,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to update entity: %w", err)
	}

	return nil
}

func iTraceTheTransactionThatCreatedTheEntity(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	receipt := w.LastReceipt

	txHash := receipt.TxHash

	trace := json.RawMessage{}

	tracerOptions := map[string]interface{}{
		"tracer":       "callTracer",
		"tracerConfig": map[string]interface{}{"withLog": true},
	}

	err := w.GethInstance.RPCClient.CallContext(ctx, &trace, "debug_traceTransaction", txHash.Hex(), tracerOptions)

	if err != nil {
		return fmt.Errorf("failed to trace transaction: %w", err)
	}

	w.LastTrace = trace

	return nil
}

func theTraceShouldBeEmpty(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	type trace struct {
		Calls json.RawMessage `json:"calls"`
	}

	t := trace{}

	fmt.Println(string(w.LastTrace))
	err := json.Unmarshal(w.LastTrace, &t)
	if err != nil {
		return fmt.Errorf("failed to unmarshal trace: %w", err)
	}

	if len(t.Calls) != 0 {
		return fmt.Errorf("expected trace to be empty, but got %s", string(t.Calls))
	}

	return nil
}

// Storage Transaction Validation Step Definitions

func iHaveAStorageTransactionWithCreateUpdateDeleteAndExtendOperations(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	w.CurrentStorageTransaction = &storagetx.StorageTransaction{
		Create: []storagetx.Create{
			{
				BTL:     100,
				Payload: []byte("test payload"),
				StringAnnotations: []entity.StringAnnotation{
					{Key: "type", Value: "test"},
				},
				NumericAnnotations: []entity.NumericAnnotation{
					{Key: "version", Value: 1},
				},
			},
		},
		Update: []storagetx.Update{
			{
				EntityKey: common.HexToHash("0x1234567890"),
				BTL:       200,
				Payload:   []byte("updated payload"),
				StringAnnotations: []entity.StringAnnotation{
					{Key: "status", Value: "updated"},
				},
				NumericAnnotations: []entity.NumericAnnotation{
					{Key: "timestamp", Value: 1678901234},
				},
			},
		},
		Delete: []common.Hash{
			common.HexToHash("0xdeadbeef"),
		},
		Extend: []storagetx.ExtendBTL{
			{
				EntityKey:      common.HexToHash("0xabcdef"),
				NumberOfBlocks: 500,
			},
		},
	}
	return nil
}

func allBTLValuesAreGreaterThan(arg1 int) error {
	// This is already satisfied by the transaction creation above
	return nil
}

func allAnnotationKeysFollowTheValidPattern() error {
	// This is already satisfied by the transaction creation above
	return nil
}

func thereAreNoDuplicateAnnotationKeys() error {
	// This is already satisfied by the transaction creation above
	return nil
}

func iValidateTheTransaction(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil {
		return fmt.Errorf("no current storage transaction set")
	}
	w.ValidationError = w.CurrentStorageTransaction.Validate()
	return nil
}

func theValidationShouldSucceed(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	if w.ValidationError != nil {
		return fmt.Errorf("expected validation to succeed, but got error: %v", w.ValidationError)
	}
	return nil
}

func theValidationShouldFail(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	if w.ValidationError == nil {
		return fmt.Errorf("expected validation to fail, but it succeeded")
	}
	return nil
}

func iHaveAStorageTransactionWithACreateOperation(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	w.CurrentStorageTransaction = &storagetx.StorageTransaction{
		Create: []storagetx.Create{
			{
				BTL:     100,
				Payload: []byte("test payload"),
			},
		},
	}
	return nil
}

func theCreateOperationHasBTLSetTo(ctx context.Context, btl int) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil || len(w.CurrentStorageTransaction.Create) == 0 {
		return fmt.Errorf("no create operation found")
	}
	w.CurrentStorageTransaction.Create[0].BTL = uint64(btl)
	return nil
}

func theErrorShouldMention(ctx context.Context, expectedText string) error {
	w := testutil.GetWorld(ctx)
	if w.ValidationError == nil {
		return fmt.Errorf("no validation error found")
	}
	if !strings.Contains(w.ValidationError.Error(), expectedText) {
		return fmt.Errorf("expected error to contain '%s', but got: %v", expectedText, w.ValidationError)
	}
	return nil
}

func iHaveAStorageTransactionWithAnUpdateOperation(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	w.CurrentStorageTransaction = &storagetx.StorageTransaction{
		Update: []storagetx.Update{
			{
				EntityKey: common.HexToHash("0x1234567890"),
				BTL:       200,
				Payload:   []byte("updated payload"),
			},
		},
	}
	return nil
}

func theUpdateOperationHasBTLSetTo(ctx context.Context, btl int) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil || len(w.CurrentStorageTransaction.Update) == 0 {
		return fmt.Errorf("no update operation found")
	}
	w.CurrentStorageTransaction.Update[0].BTL = uint64(btl)
	return nil
}

func iHaveAStorageTransactionWithAnExtendOperation(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	w.CurrentStorageTransaction = &storagetx.StorageTransaction{
		Extend: []storagetx.ExtendBTL{
			{
				EntityKey:      common.HexToHash("0x1234567890"),
				NumberOfBlocks: 500,
			},
		},
	}
	return nil
}

func theExtendOperationHasNumberOfBlocksSetTo(ctx context.Context, blocks int) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil || len(w.CurrentStorageTransaction.Extend) == 0 {
		return fmt.Errorf("no extend operation found")
	}
	w.CurrentStorageTransaction.Extend[0].NumberOfBlocks = uint64(blocks)
	return nil
}

func theCreateOperationHasAStringAnnotationWithKeyStartingWith(ctx context.Context, keyPrefix string) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil || len(w.CurrentStorageTransaction.Create) == 0 {
		return fmt.Errorf("no create operation found")
	}
	w.CurrentStorageTransaction.Create[0].StringAnnotations = []entity.StringAnnotation{
		{Key: keyPrefix + "invalid", Value: "test"},
	}
	return nil
}

func theCreateOperationHasDuplicateStringAnnotationKeys(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil || len(w.CurrentStorageTransaction.Create) == 0 {
		return fmt.Errorf("no create operation found")
	}
	w.CurrentStorageTransaction.Create[0].StringAnnotations = []entity.StringAnnotation{
		{Key: "type", Value: "test1"},
		{Key: "type", Value: "test2"},
	}
	return nil
}

func theCreateOperationHasDuplicateNumericAnnotationKeys(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil || len(w.CurrentStorageTransaction.Create) == 0 {
		return fmt.Errorf("no create operation found")
	}
	w.CurrentStorageTransaction.Create[0].NumericAnnotations = []entity.NumericAnnotation{
		{Key: "version", Value: 1},
		{Key: "version", Value: 2},
	}
	return nil
}

func theUpdateOperationHasDuplicateStringAnnotationKeys(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil || len(w.CurrentStorageTransaction.Update) == 0 {
		return fmt.Errorf("no update operation found")
	}
	w.CurrentStorageTransaction.Update[0].StringAnnotations = []entity.StringAnnotation{
		{Key: "status", Value: "active"},
		{Key: "status", Value: "inactive"},
	}
	return nil
}

func theUpdateOperationHasDuplicateNumericAnnotationKeys(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil || len(w.CurrentStorageTransaction.Update) == 0 {
		return fmt.Errorf("no update operation found")
	}
	w.CurrentStorageTransaction.Update[0].NumericAnnotations = []entity.NumericAnnotation{
		{Key: "timestamp", Value: 1},
		{Key: "timestamp", Value: 2},
	}
	return nil
}

func theCreateOperationHasStringAnnotationsWithKeys(ctx context.Context, key1, key2, key3 string) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil || len(w.CurrentStorageTransaction.Create) == 0 {
		return fmt.Errorf("no create operation found")
	}
	w.CurrentStorageTransaction.Create[0].StringAnnotations = []entity.StringAnnotation{
		{Key: key1, Value: "value1"},
		{Key: key2, Value: "value2"},
		{Key: key3, Value: "value3"},
	}
	return nil
}

func theCreateOperationHasNumericAnnotationsWithKeys(ctx context.Context, key1, key2 string) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil || len(w.CurrentStorageTransaction.Create) == 0 {
		return fmt.Errorf("no create operation found")
	}
	w.CurrentStorageTransaction.Create[0].NumericAnnotations = []entity.NumericAnnotation{
		{Key: key1, Value: 1},
		{Key: key2, Value: 2},
	}
	return nil
}

func theCreateOperationHasAStringAnnotationWithUnicodeKey(ctx context.Context, key string) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil || len(w.CurrentStorageTransaction.Create) == 0 {
		return fmt.Errorf("no create operation found")
	}
	w.CurrentStorageTransaction.Create[0].StringAnnotations = []entity.StringAnnotation{
		{Key: key, Value: "unicode value"},
	}
	return nil
}

func theCreateOperationHasAStringAnnotationWithKeyContainingSpecialCharactersLikeOr(ctx context.Context, char1, char2 string) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil || len(w.CurrentStorageTransaction.Create) == 0 {
		return fmt.Errorf("no create operation found")
	}
	// Use the first special character
	invalidKey := "invalid" + char1 + "key"
	w.CurrentStorageTransaction.Create[0].StringAnnotations = []entity.StringAnnotation{
		{Key: invalidKey, Value: "test"},
	}
	return nil
}

func theCreateOperationHasAStringAnnotationWithKeyStartingWithANumber(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil || len(w.CurrentStorageTransaction.Create) == 0 {
		return fmt.Errorf("no create operation found")
	}
	w.CurrentStorageTransaction.Create[0].StringAnnotations = []entity.StringAnnotation{
		{Key: "123invalid", Value: "test"},
	}
	return nil
}

func iHaveAnEmptyStorageTransaction(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	w.CurrentStorageTransaction = &storagetx.StorageTransaction{}
	return nil
}

func iHaveAStorageTransactionWithMultipleCreateOperations(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	w.CurrentStorageTransaction = &storagetx.StorageTransaction{
		Create: []storagetx.Create{
			{
				BTL:     100,
				Payload: []byte("valid payload"),
			},
			{
				BTL:     200,
				Payload: []byte("another valid payload"),
			},
		},
	}
	return nil
}

func oneCreateOperationHasBTLSetTo(ctx context.Context, btl int) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil || len(w.CurrentStorageTransaction.Create) == 0 {
		return fmt.Errorf("no create operations found")
	}
	// Set the first operation's BTL to the specified value (likely 0 for error case)
	w.CurrentStorageTransaction.Create[0].BTL = uint64(btl)
	return nil
}

func anotherCreateOperationHasValidBTLAndAnnotations(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	if w.CurrentStorageTransaction == nil || len(w.CurrentStorageTransaction.Create) < 2 {
		return fmt.Errorf("need at least 2 create operations")
	}
	// The second operation should remain valid
	w.CurrentStorageTransaction.Create[1].StringAnnotations = []entity.StringAnnotation{
		{Key: "valid_key", Value: "valid_value"},
	}
	return nil
}

func theErrorShouldMentionAnd(ctx context.Context, text1, text2 string) error {
	w := testutil.GetWorld(ctx)
	if w.ValidationError == nil {
		return fmt.Errorf("no validation error found")
	}
	errorMsg := w.ValidationError.Error()
	if !strings.Contains(errorMsg, text1) {
		return fmt.Errorf("expected error to contain '%s', but got: %v", text1, w.ValidationError)
	}
	if !strings.Contains(errorMsg, text2) {
		return fmt.Errorf("expected error to contain '%s', but got: %v", text2, w.ValidationError)
	}
	return nil
}

func theErrorShouldMentionTheFirstValidationErrorEncountered(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	if w.ValidationError == nil {
		return fmt.Errorf("no validation error found")
	}
	// The first validation error should be about BTL being 0
	if !strings.Contains(w.ValidationError.Error(), "BTL is 0") {
		return fmt.Errorf("expected first error to be about BTL, but got: %v", w.ValidationError)
	}
	return nil
}

func iSubmitAStorageTransactionWithNoPlayload(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	_, err := w.SendTxWithData(
		ctx,
		big.NewInt(1),
		address.GolemBaseStorageProcessorAddress,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to transfer: %w", err)
	}
	return nil
}

func iSubmitAStorageTransactionWithUnparseableData(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	_, err := w.SendTxWithData(
		ctx,
		big.NewInt(1),
		address.GolemBaseStorageProcessorAddress,
		[]byte("unparseable data"),
	)
	if err != nil {
		return fmt.Errorf("failed to transfer: %w", err)
	}
	return nil
}

func theEntityUpdateLogShouldBeRecorded(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	receipt := w.LastReceipt

	if len(receipt.Logs) == 0 {
		return fmt.Errorf("no logs found in receipt")
	}

	logs := receipt.Logs

	if len(logs) != 2 {
		return fmt.Errorf("expected 2 logs, got %d", len(logs))
	}

	oldLog := logs[0]

	if oldLog.Topics[0] != storagetx.GolemBaseStorageEntityUpdated {
		return fmt.Errorf("expected GolemBaseStorageEntityUpdated log, got %s", oldLog.Topics[0])
	}

	oldLogData := oldLog.Data

	if len(oldLogData) != 32 {
		return fmt.Errorf("expected old log data to be 32 bytes, got %d", len(oldLogData))
	}

	oldExpiresAtBlock := uint256.NewInt(0).SetBytes(oldLogData).Uint64()

	expiresAtBlockExpected := receipt.BlockNumber.Uint64() + 100

	if oldExpiresAtBlock != expiresAtBlockExpected {
		return fmt.Errorf("expected old expires at block to be %d, got %d", expiresAtBlockExpected, oldExpiresAtBlock)
	}

	newLog := logs[1]

	if newLog.Topics[0] != arkivlogs.ArkivEntityUpdated {
		return fmt.Errorf("expected ArkivEntityUpdated log, got %s", newLog.Topics[0])
	}

	newLogData := newLog.Data

	if len(newLogData) != 96 {
		return fmt.Errorf("expected new log data to be 64 bytes, got %d", len(newLogData))
	}

	oldEntityExpiresAtBlock := uint256.NewInt(0).SetBytes(newLogData[:32]).Uint64()

	if oldEntityExpiresAtBlock != (expiresAtBlockExpected - 1) {
		return fmt.Errorf("expected old entity expires at block to be %d, got %d", expiresAtBlockExpected-1, oldExpiresAtBlock)
	}

	newExpiresAtBlockU256 := uint256.NewInt(0).SetBytes(newLogData[32:64])
	newExpiresAtBlock := newExpiresAtBlockU256.Uint64()

	if newExpiresAtBlock != expiresAtBlockExpected {
		return fmt.Errorf("expected new expires at block to be %d, got %d", expiresAtBlockExpected, newExpiresAtBlock)
	}

	owner := hashToAddress(newLog.Topics[2])

	if owner != w.FundedAccount.Address {
		return fmt.Errorf("expected owner to be %s, got %s", w.FundedAccount.Address.Hex(), owner.Hex())
	}

	return nil
}

func theEntityDeleteLogShouldBeRecorded(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	receipt := w.LastReceipt

	if len(receipt.Logs) == 0 {
		return fmt.Errorf("no logs found in receipt")
	}

	if len(receipt.Logs) != 2 {
		return fmt.Errorf("expected 2 logs, got %d", len(receipt.Logs))
	}

	oldLog := receipt.Logs[0]

	if oldLog.Topics[0] != storagetx.GolemBaseStorageEntityDeleted {
		return fmt.Errorf("expected GolemBaseStorageEntityDeleted log, got %s", oldLog.Topics[0])
	}

	newLog := receipt.Logs[1]

	if newLog.Topics[0] != arkivlogs.ArkivEntityDeleted {
		return fmt.Errorf("expected ArkivEntityDeleted log, got %s", newLog.Topics[0])
	}

	if len(newLog.Topics) != 3 {
		return fmt.Errorf("expected 3 topics, got %d", len(newLog.Topics))
	}

	if newLog.Topics[1] != w.CreatedEntityKey {
		return fmt.Errorf("expected arkiv entity deleted entity key to be %s, got %s", w.CreatedEntityKey.Hex(), newLog.Topics[1])
	}

	owner := hashToAddress(newLog.Topics[2])

	if owner != w.FundedAccount.Address {
		return fmt.Errorf("expected owner to be %s, got %s", w.FundedAccount.Address.Hex(), owner.Hex())
	}

	return nil
}

func theEntityExtendLogShouldBeRecorded(ctx context.Context) error {
	w := testutil.GetWorld(ctx)
	receipt := w.LastReceipt

	if len(receipt.Logs) == 0 {
		return fmt.Errorf("no logs found in receipt")
	}

	if len(receipt.Logs) != 2 {
		return fmt.Errorf("expected 2 logs, got %d", len(receipt.Logs))
	}

	oldLog := receipt.Logs[0]

	if oldLog.Topics[0] != storagetx.GolemBaseStorageEntityBTLExtended {
		return fmt.Errorf("expected GolemBaseStorageEntityBTLExtended log, got %s", oldLog.Topics[0])
	}

	{
		oldLogData := oldLog.Data
		if len(oldLogData) != 64 {
			return fmt.Errorf("expected old log data to be 64 bytes, got %d", len(oldLogData))
		}
		oldExpiresAtBlock := uint256.NewInt(0).SetBytes(oldLogData[:32]).Uint64()
		if oldExpiresAtBlock != (receipt.BlockNumber.Uint64() + 100 - 1) {
			return fmt.Errorf("expected old entity expires at block to be %d, got %d", receipt.BlockNumber.Uint64()+100-1, oldExpiresAtBlock)
		}
		newExpiresAtBlock := uint256.NewInt(0).SetBytes(oldLogData[32:64]).Uint64()
		if newExpiresAtBlock != (receipt.BlockNumber.Uint64() + 200 - 1) {
			return fmt.Errorf("expected new entity expires at block to be %d, got %d", receipt.BlockNumber.Uint64()+200-1, newExpiresAtBlock)
		}
	}

	newLog := receipt.Logs[1]

	if newLog.Topics[0] != arkivlogs.ArkivEntityBTLExtended {
		return fmt.Errorf("expected ArkivEntityBTLExtended log, got %s", newLog.Topics[0])
	}

	if len(newLog.Topics) != 3 {
		return fmt.Errorf("expected 3 topics, got %d", len(newLog.Topics))
	}

	if newLog.Topics[1] != w.CreatedEntityKey {
		return fmt.Errorf("expected arkiv entity extended entity key to be %s, got %s", w.CreatedEntityKey.Hex(), newLog.Topics[1])
	}

	owner := hashToAddress(newLog.Topics[2])

	if owner != w.FundedAccount.Address {
		return fmt.Errorf("expected owner to be %s, got %s", w.FundedAccount.Address.Hex(), owner.Hex())
	}
	{
		newLogData := newLog.Data
		if len(newLogData) != 96 {
			return fmt.Errorf("expected new log data to be 96 bytes, got %d", len(newLogData))
		}
		oldExpiresAtBlock := uint256.NewInt(0).SetBytes(newLogData[:32]).Uint64()
		if oldExpiresAtBlock != (receipt.BlockNumber.Uint64() + 100 - 1) {
			return fmt.Errorf("expected old entity expires at block to be %d, got %d", receipt.BlockNumber.Uint64()+100-1, oldExpiresAtBlock)
		}
		newExpiresAtBlock := uint256.NewInt(0).SetBytes(newLogData[32:64]).Uint64()
		if newExpiresAtBlock != (receipt.BlockNumber.Uint64() + 200 - 1) {
			return fmt.Errorf("expected new entity expires at block to be %d, got %d", receipt.BlockNumber.Uint64()+200-1, newExpiresAtBlock)
		}
	}

	return nil
}
