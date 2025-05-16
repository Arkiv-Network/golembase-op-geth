package golembase_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/repr"
	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/golem-base/golemtype"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/golem-base/testutil"
	"github.com/ethereum/go-ethereum/golem-base/wal"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/holiman/uint256"
	"github.com/spf13/pflag" // godog v0.11.0 and later
	"github.com/warpfork/go-wish/difflib"
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
	ctx.Step(`^the write-ahead log for the create should be created$`, theWriteaheadLogForTheCreateShouldBeCreated)
	ctx.Step(`^the write-ahead log for the update should be created$`, theWriteaheadLogForTheUpdateShouldBeCreated)
	ctx.Step(`^the write-ahead log for the delete should be created$`, theWriteaheadLogForTheDeleteShouldBeCreated)
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

func theEntityShouldBeCreated(ctx context.Context) error {

	w := testutil.GetWorld(ctx)
	receipt := w.LastReceipt

	if len(receipt.Logs) == 0 {
		return fmt.Errorf("no logs found in receipt")
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

func theWriteaheadLogForTheCreateShouldBeCreated(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	wl, err := w.ReadWAL(ctx)
	if err != nil {
		return fmt.Errorf("failed to read write-ahead log: %w", err)
	}

	err = checkIfEqual(
		wl,
		[]wal.Operation{
			{
				Create: &wal.Create{
					EntityKey:      w.CreatedEntityKey,
					ExpiresAtBlock: 103,
					Payload:        []byte("test payload"),
					StringAnnotations: []entity.StringAnnotation{
						{Key: "test_key", Value: "test_value"},
					},
					NumericAnnotations: []entity.NumericAnnotation{
						{Key: "test_number", Value: 42},
					},
					Owner: w.FundedAccount.Address,
				},
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to check if write-ahead log is equal: %w", err)
	}

	return nil

}

func checkIfEqual(actual, expected []wal.Operation) error {

	if !reflect.DeepEqual(actual, expected) {

		expectedText := repr.String(expected, repr.Indent("  "))
		actualText := repr.String(actual, repr.Indent("  "))

		diff := difflib.UnifiedDiff{
			A:        difflib.SplitLines(expectedText),
			B:        difflib.SplitLines(actualText),
			FromFile: "Expected",
			ToFile:   "Current",
			Context:  3,
		}

		diffText, _ := difflib.GetUnifiedDiffString(diff)

		return fmt.Errorf("expected\n%s\nbut got\n%s\ndiff:\n%s", expectedText, actualText, diffText)
	}

	return nil
}

func theWriteaheadLogForTheUpdateShouldBeCreated(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	wl, err := w.ReadWAL(ctx)
	if err != nil {
		return fmt.Errorf("failed to read write-ahead log: %w", err)
	}

	err = checkIfEqual(wl[1:],
		[]wal.Operation{
			{
				Update: &wal.Update{
					EntityKey:      w.CreatedEntityKey,
					ExpiresAtBlock: 104,
					Payload:        []byte("new payload"),
					StringAnnotations: []entity.StringAnnotation{
						{Key: "test_key", Value: "test_value"},
					},
					NumericAnnotations: []entity.NumericAnnotation{
						{Key: "test_number", Value: 42},
					},
				},
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to check if write-ahead log is equal: %w", err)
	}

	return nil

}

func theWriteaheadLogForTheDeleteShouldBeCreated(ctx context.Context) error {
	w := testutil.GetWorld(ctx)

	wl, err := w.ReadWAL(ctx)
	if err != nil {
		return fmt.Errorf("failed to read write-ahead log: %w", err)
	}

	err = checkIfEqual(
		wl[1:],
		[]wal.Operation{
			{
				Delete: &w.CreatedEntityKey,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("check deleted: failed to check if write-ahead log is equal: %w", err)
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
