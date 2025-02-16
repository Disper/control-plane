package deprovisioning

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/avs"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/broker"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/internal/storage"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var (
	evalIdsHolder      []int64
	parentEvalIdHolder = make(map[int64]int64)
)

const (
	internalEvalId = int64(1234)
	externalEvalId = int64(5678)
	parentEvalId   = int64(91011)
)

func TestAvsEvaluationsRemovalStep_Run(t *testing.T) {
	// given
	logger := logrus.New()
	memoryStorage := storage.NewMemoryStorage()

	deProvisioningOperation := fixDeprovisioningOperation().Operation
	deProvisioningOperation.Avs.AvsEvaluationInternalId = internalEvalId
	deProvisioningOperation.Avs.AVSEvaluationExternalId = externalEvalId
	err := memoryStorage.Operations().InsertOperation(deProvisioningOperation)
	assert.NoError(t, err)
	assert.False(t, deProvisioningOperation.Avs.AVSInternalEvaluationDeleted)
	assert.False(t, deProvisioningOperation.Avs.AVSExternalEvaluationDeleted)

	mockOauthServer := newMockAvsOauthServer()
	defer mockOauthServer.Close()
	mockAvsServer := newMockAvsServer(t)
	defer mockAvsServer.Close()
	avsConfig := avsConfig(mockOauthServer, mockAvsServer)
	avsClient, err := avs.NewClient(context.TODO(), avsConfig, logrus.New())
	assert.NoError(t, err)
	avsDel := avs.NewDelegator(avsClient, avsConfig, memoryStorage.Operations())
	internalEvalAssistant := avs.NewInternalEvalAssistant(avsConfig)
	externalEvalAssistant := avs.NewExternalEvalAssistant(avsConfig)
	step := NewAvsEvaluationsRemovalStep(avsDel, memoryStorage.Operations(), externalEvalAssistant, internalEvalAssistant)

	assert.Equal(t, 0, len(evalIdsHolder))
	assert.Equal(t, 0, len(parentEvalIdHolder))
	// when
	deProvisioningOperation, repeat, err := step.Run(deProvisioningOperation, logger)

	// then
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(0), repeat)

	assert.Equal(t, 2, len(evalIdsHolder))
	assert.Contains(t, evalIdsHolder, internalEvalId)
	assert.Contains(t, evalIdsHolder, externalEvalId)

	assert.Equal(t, 2, len(parentEvalIdHolder))
	assert.Contains(t, parentEvalIdHolder, internalEvalId)
	assert.Contains(t, parentEvalIdHolder, externalEvalId)
	assert.Equal(t, parentEvalIdHolder[internalEvalId], parentEvalId)
	assert.Equal(t, parentEvalIdHolder[externalEvalId], parentEvalId)

	inDB, err := memoryStorage.Operations().GetDeprovisioningOperationByID(deProvisioningOperation.ID)
	assert.NoError(t, err)
	assert.True(t, inDB.Avs.AVSInternalEvaluationDeleted)
	assert.True(t, inDB.Avs.AVSExternalEvaluationDeleted)
	assert.Equal(t, internalEvalId, inDB.Avs.AvsEvaluationInternalId)
	assert.Equal(t, externalEvalId, inDB.Avs.AVSEvaluationExternalId)
}

func TestAvsEvaluationsRemovalWhenAlreadyDeleted_Run(t *testing.T) {
	// given
	logger := logrus.New()
	memoryStorage := storage.NewMemoryStorage()

	deProvisioningOperation := fixDeprovisioningOperation().Operation
	deProvisioningOperation.Avs.AvsEvaluationInternalId = internalEvalId
	deProvisioningOperation.Avs.AVSEvaluationExternalId = externalEvalId
	deProvisioningOperation.Avs.AVSExternalEvaluationDeleted = true
	deProvisioningOperation.Avs.AVSInternalEvaluationDeleted = true
	err := memoryStorage.Operations().InsertOperation(deProvisioningOperation)
	assert.NoError(t, err)
	assert.True(t, deProvisioningOperation.Avs.AVSInternalEvaluationDeleted)
	assert.True(t, deProvisioningOperation.Avs.AVSExternalEvaluationDeleted)

	mockOauthServer := newMockAvsOauthServer()
	defer mockOauthServer.Close()
	mockAvsServer := newMockAvsServer(t)
	defer mockAvsServer.Close()
	avsConfig := avsConfig(mockOauthServer, mockAvsServer)
	avsClient, err := avs.NewClient(context.TODO(), avsConfig, logrus.New())
	assert.NoError(t, err)
	avsDel := avs.NewDelegator(avsClient, avsConfig, memoryStorage.Operations())
	internalEvalAssistant := avs.NewInternalEvalAssistant(avsConfig)
	externalEvalAssistant := avs.NewExternalEvalAssistant(avsConfig)
	step := NewAvsEvaluationsRemovalStep(avsDel, memoryStorage.Operations(), externalEvalAssistant, internalEvalAssistant)

	// when
	deProvisioningOperation, repeat, err := step.Run(deProvisioningOperation, logger)

	// then
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(0), repeat)

	inDB, err := memoryStorage.Operations().GetDeprovisioningOperationByID(deProvisioningOperation.ID)
	assert.NoError(t, err)
	assert.True(t, inDB.Avs.AVSInternalEvaluationDeleted)
	assert.True(t, inDB.Avs.AVSExternalEvaluationDeleted)
	assert.Equal(t, internalEvalId, inDB.Avs.AvsEvaluationInternalId)
	assert.Equal(t, externalEvalId, inDB.Avs.AVSEvaluationExternalId)
}

func TestExternalAvsEvaluationsRemovalSkipForTrialPlan_Run(t *testing.T) {
	// given
	logger := logrus.New()
	memoryStorage := storage.NewMemoryStorage()

	deProvisioningOperation := fixDeprovisioningOperationWithPlanID(broker.TrialPlanID)
	deProvisioningOperation.Avs.AVSEvaluationExternalId = externalEvalId
	err := memoryStorage.Operations().InsertOperation(deProvisioningOperation)
	assert.NoError(t, err)
	assert.False(t, deProvisioningOperation.Avs.AVSInternalEvaluationDeleted)
	assert.False(t, deProvisioningOperation.Avs.AVSExternalEvaluationDeleted)

	mockOauthServer := newMockAvsOauthServer()
	defer mockOauthServer.Close()
	mockAvsServer := newMockAvsServer(t)
	defer mockAvsServer.Close()
	avsConfig := avsConfig(mockOauthServer, mockAvsServer)
	avsClient, err := avs.NewClient(context.TODO(), avsConfig, logrus.New())
	assert.NoError(t, err)
	avsDel := avs.NewDelegator(avsClient, avsConfig, memoryStorage.Operations())
	internalEvalAssistant := avs.NewInternalEvalAssistant(avsConfig)
	externalEvalAssistant := avs.NewExternalEvalAssistant(avsConfig)
	step := NewAvsEvaluationsRemovalStep(avsDel, memoryStorage.Operations(), externalEvalAssistant, internalEvalAssistant)

	// when
	deProvisioningOperation, repeat, err := step.Run(deProvisioningOperation, logger)

	// then
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(0), repeat)

	inDB, err := memoryStorage.Operations().GetDeprovisioningOperationByID(deProvisioningOperation.ID)
	assert.NoError(t, err)
	assert.False(t, inDB.Avs.AVSExternalEvaluationDeleted)
	assert.Equal(t, externalEvalId, inDB.Avs.AVSEvaluationExternalId)
}

func TestExternalAvsEvaluationsRemovalSkipForFreemiumPlan_Run(t *testing.T) {
	// given
	logger := logrus.New()
	memoryStorage := storage.NewMemoryStorage()

	deProvisioningOperation := fixDeprovisioningOperationWithPlanID(broker.FreemiumPlanID)
	deProvisioningOperation.Avs.AVSEvaluationExternalId = externalEvalId
	err := memoryStorage.Operations().InsertOperation(deProvisioningOperation)
	assert.NoError(t, err)
	assert.False(t, deProvisioningOperation.Avs.AVSInternalEvaluationDeleted)
	assert.False(t, deProvisioningOperation.Avs.AVSExternalEvaluationDeleted)

	mockOauthServer := newMockAvsOauthServer()
	defer mockOauthServer.Close()
	mockAvsServer := newMockAvsServer(t)
	defer mockAvsServer.Close()
	avsConfig := avsConfig(mockOauthServer, mockAvsServer)
	avsClient, err := avs.NewClient(context.TODO(), avsConfig, logrus.New())
	assert.NoError(t, err)
	avsDel := avs.NewDelegator(avsClient, avsConfig, memoryStorage.Operations())
	internalEvalAssistant := avs.NewInternalEvalAssistant(avsConfig)
	externalEvalAssistant := avs.NewExternalEvalAssistant(avsConfig)
	step := NewAvsEvaluationsRemovalStep(avsDel, memoryStorage.Operations(), externalEvalAssistant, internalEvalAssistant)

	// when
	deProvisioningOperation, repeat, err := step.Run(deProvisioningOperation, logger)

	// then
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(0), repeat)

	inDB, err := memoryStorage.Operations().GetDeprovisioningOperationByID(deProvisioningOperation.ID)
	assert.NoError(t, err)
	assert.False(t, inDB.Avs.AVSExternalEvaluationDeleted)
	assert.Equal(t, externalEvalId, inDB.Avs.AVSEvaluationExternalId)
}

func newMockAvsOauthServer() *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"access_token": "90d64460d14870c08c81352a05dedd3465940a7c", "scope": "user", "token_type": "bearer", "expires_in": 86400}`))
		}))
}

func newMockAvsServer(t *testing.T) *httptest.Server {
	router := mux.NewRouter()
	router.HandleFunc("/{evalId}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		evalIdsHolder = append(evalIdsHolder, extractId(vars, "evalId", t))
		w.WriteHeader(http.StatusOK)
	})).Methods(http.MethodDelete)

	router.HandleFunc("/{parentId}/child/{evalId}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		parentEval := extractId(vars, "parentId", t)
		evalId := extractId(vars, "evalId", t)
		parentEvalIdHolder[evalId] = parentEval

		w.WriteHeader(http.StatusOK)
	})).Methods(http.MethodDelete)
	return httptest.NewServer(router)
}

func extractId(vars map[string]string, key string, t *testing.T) int64 {
	evalIdStr := vars[key]
	evalId, err := strconv.ParseInt(evalIdStr, 10, 64)
	assert.NoError(t, err)
	return evalId
}

func avsConfig(mockOauthServer *httptest.Server, mockAvsServer *httptest.Server) avs.Config {
	return avs.Config{
		OauthTokenEndpoint:     mockOauthServer.URL,
		OauthUsername:          "dummy",
		OauthPassword:          "dummy",
		OauthClientId:          "dummy",
		ApiEndpoint:            mockAvsServer.URL,
		DefinitionType:         avs.DefinitionType,
		InternalTesterAccessId: 1234,
		InternalTesterService:  "",
		InternalTesterTags:     []*avs.Tag{},
		ExternalTesterAccessId: 5678,
		ExternalTesterService:  "dummy",
		ExternalTesterTags: []*avs.Tag{
			{
				Content:      "dummy",
				TagClassId:   123,
				TagClassName: "dummy",
			},
		},
		GroupId:  5555,
		ParentId: parentEvalId,
	}
}
