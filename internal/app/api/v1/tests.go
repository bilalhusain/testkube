package v1

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	testsv2 "github.com/kubeshop/testkube-operator/apis/tests/v2"
	"github.com/kubeshop/testkube/pkg/api/v1/testkube"
	"github.com/kubeshop/testkube/pkg/cronjob"
	testsmapper "github.com/kubeshop/testkube/pkg/mapper/tests"
	"github.com/kubeshop/testkube/pkg/secret"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/kubeshop/testkube/pkg/jobs"
	"k8s.io/apimachinery/pkg/api/errors"
)

// GetTestHandler is method for getting an existing test
func (s TestkubeAPI) GetTestHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		name := c.Params("id")
		crTest, err := s.TestsClient.Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return s.Error(c, http.StatusNotFound, err)
			}

			return s.Error(c, http.StatusBadGateway, err)
		}

		test := testsmapper.MapTestCRToAPI(*crTest)

		return c.JSON(test)
	}
}

// GetTestWithExecutionHandler is method for getting an existing test with execution
func (s TestkubeAPI) GetTestWithExecutionHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		name := c.Params("id")
		crTest, err := s.TestsClient.Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return s.Error(c, http.StatusNotFound, err)
			}

			return s.Error(c, http.StatusBadGateway, err)
		}

		ctx := c.Context()
		execution, err := s.ExecutionResults.GetLatestByTest(ctx, name)
		if err != nil && err != mongo.ErrNoDocuments {
			return s.Error(c, http.StatusInternalServerError, err)
		}

		test := testsmapper.MapTestCRToAPI(*crTest)
		testWithExecution := testkube.TestWithExecution{
			Test: &test,
		}
		if err == nil {
			testWithExecution.LatestExecution = &execution
		}

		return c.JSON(testWithExecution)
	}
}

func (s TestkubeAPI) getFilteredTestList(c *fiber.Ctx) (*testsv2.TestList, error) {
	// TODO filters looks messy need to introduce some common Filter object for Kubernetes query for List like objects
	crTests, err := s.TestsClient.List(c.Query("selector"))
	if err != nil {
		return nil, err
	}

	search := c.Query("textSearch")
	if search != "" {
		// filter items array
		for i := len(crTests.Items) - 1; i >= 0; i-- {
			if !strings.Contains(crTests.Items[i].Name, search) {
				crTests.Items = append(crTests.Items[:i], crTests.Items[i+1:]...)
			}
		}
	}

	testType := c.Query("type")
	if testType != "" {
		// filter items array
		for i := len(crTests.Items) - 1; i >= 0; i-- {
			if !strings.Contains(crTests.Items[i].Spec.Type_, testType) {
				crTests.Items = append(crTests.Items[:i], crTests.Items[i+1:]...)
			}
		}
	}

	return crTests, nil
}

// ListTestsHandler is a method for getting list of all available tests
func (s TestkubeAPI) ListTestsHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		crTests, err := s.getFilteredTestList(c)
		if err != nil {
			return s.Error(c, http.StatusBadGateway, err)
		}

		tests := testsmapper.MapTestListKubeToAPI(*crTests)

		return c.JSON(tests)
	}
}

// ListTestWithExecutionsHandler is a method for getting list of all available test with latest executions
func (s TestkubeAPI) ListTestWithExecutionsHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		crTests, err := s.getFilteredTestList(c)
		if err != nil {
			return s.Error(c, http.StatusBadGateway, err)
		}

		tests := testsmapper.MapTestListKubeToAPI(*crTests)
		ctx := c.Context()
		testWithExecutions := make([]testkube.TestWithExecution, len(tests))
		testNames := make([]string, len(tests))
		for i := range tests {
			testNames[i] = tests[i].Name
		}

		executions, err := s.ExecutionResults.GetLatestByTests(ctx, testNames)
		if err != nil && err != mongo.ErrNoDocuments {
			return s.Error(c, http.StatusInternalServerError, err)
		}

		executionMap := make(map[string]testkube.Execution, len(executions))
		for i := range executions {
			executionMap[executions[i].TestName] = executions[i]
		}

		for i := range tests {
			testWithExecutions[i].Test = &tests[i]
			if execution, ok := executionMap[tests[i].Name]; ok {
				testWithExecutions[i].LatestExecution = &execution
			}
		}

		status := c.Query("status")
		if status != "" {
			statusList, err := testkube.ParseExecutionStatusList(status, ",")
			if err != nil {
				return s.Error(c, http.StatusBadRequest, fmt.Errorf("execution status filter invalid: %w", err))
			}

			statusMap := statusList.ToMap()
			// filter items array
			for i := len(testWithExecutions) - 1; i >= 0; i-- {
				if testWithExecutions[i].LatestExecution != nil && testWithExecutions[i].LatestExecution.ExecutionResult != nil &&
					testWithExecutions[i].LatestExecution.ExecutionResult.Status != nil {
					if _, ok := statusMap[*testWithExecutions[i].LatestExecution.ExecutionResult.Status]; ok {
						continue
					}
				}

				testWithExecutions = append(testWithExecutions[:i], testWithExecutions[i+1:]...)
			}
		}

		return c.JSON(testWithExecutions)
	}
}

// CreateTestHandler creates new test CR based on test content
func (s TestkubeAPI) CreateTestHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {

		var request testkube.TestUpsertRequest
		err := c.BodyParser(&request)
		if err != nil {
			return s.Error(c, http.StatusBadRequest, err)
		}

		s.Log.Infow("creating test", "request", request)

		testSpec := testsmapper.MapToSpec(request)
		testSpec.Namespace = s.Namespace
		test, err := s.TestsClient.Create(testSpec)

		s.Metrics.IncCreateTest(test.Spec.Type_, err)

		if err != nil {
			return s.Error(c, http.StatusBadGateway, err)
		}

		stringData := GetSecretsStringData(request.Content)
		if err = s.SecretClient.Create(secret.GetMetadataName(request.Name), test.Labels, stringData); err != nil {
			return s.Error(c, http.StatusBadGateway, err)
		}

		return c.JSON(test)
	}
}

// UpdateTestHandler updates an existing test CR based on test content
func (s TestkubeAPI) UpdateTestHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {

		var request testkube.TestUpsertRequest
		err := c.BodyParser(&request)
		if err != nil {
			return s.Error(c, http.StatusBadRequest, err)
		}

		s.Log.Infow("updating test", "request", request)

		// we need to get resource first and load its metadata.ResourceVersion
		test, err := s.TestsClient.Get(request.Name)
		if err != nil {
			return s.Error(c, http.StatusBadGateway, err)
		}

		// delete cron job, if schedule is cleaned
		if test.Spec.Schedule != "" {
			cronJob, err := s.CronJobClient.Get(cronjob.GetMetadataName(request.Name, testResourceURI))
			if err != nil && !errors.IsNotFound(err) {
				return s.Error(c, http.StatusBadGateway, err)
			}

			if cronJob != nil {
				if request.Schedule == "" {
					if err = s.CronJobClient.Delete(cronjob.GetMetadataName(request.Name, testResourceURI)); err != nil {
						return s.Error(c, http.StatusBadGateway, err)
					}
				} else {
					if err = s.CronJobClient.UpdateLabels(cronJob, test.Labels, request.Labels); err != nil {
						return s.Error(c, http.StatusBadGateway, err)
					}
				}
			}
		}

		// map test but load spec only to not override metadata.ResourceVersion
		testSpec := testsmapper.MapToSpec(request)
		test.Spec = testSpec.Spec
		test.Labels = request.Labels
		test, err = s.TestsClient.Update(test)

		s.Metrics.IncUpdateTest(test.Spec.Type_, err)

		if err != nil {
			return s.Error(c, http.StatusBadGateway, err)
		}

		// update secrets for scipt
		stringData := GetSecretsStringData(request.Content)
		if err = s.SecretClient.Apply(secret.GetMetadataName(request.Name), test.Labels, stringData); err != nil {
			return s.Error(c, http.StatusBadGateway, err)
		}

		return c.JSON(test)
	}
}

// DeleteTestHandler is a method for deleting a test with id
func (s TestkubeAPI) DeleteTestHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		name := c.Params("id")
		err := s.TestsClient.Delete(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return s.Warn(c, http.StatusNotFound, err)
			}

			return s.Error(c, http.StatusBadGateway, err)
		}

		// delete secrets for test
		if err = s.SecretClient.Delete(secret.GetMetadataName(name)); err != nil {
			if !errors.IsNotFound(err) {
				return s.Error(c, http.StatusBadGateway, err)
			}
		}

		// delete cron job for test
		if err = s.CronJobClient.Delete(cronjob.GetMetadataName(name, testResourceURI)); err != nil {
			if !errors.IsNotFound(err) {
				return s.Error(c, http.StatusBadGateway, err)
			}
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}

// DeleteTestsHandler for deleting all tests
func (s TestkubeAPI) DeleteTestsHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var err error
		selector := c.Query("selector")
		if selector == "" {
			err = s.TestsClient.DeleteAll()
		} else {
			err = s.TestsClient.DeleteByLabels(selector)
		}

		if err != nil {
			if errors.IsNotFound(err) {
				return s.Warn(c, http.StatusNotFound, err)
			}

			return s.Error(c, http.StatusBadGateway, err)
		}

		// delete all secrets for tests
		if err = s.SecretClient.DeleteAll(selector); err != nil {
			if !errors.IsNotFound(err) {
				return s.Error(c, http.StatusBadGateway, err)
			}
		}

		// delete all cron jobs for tests
		if err = s.CronJobClient.DeleteAll(testResourceURI, selector); err != nil {
			if !errors.IsNotFound(err) {
				return s.Error(c, http.StatusBadGateway, err)
			}
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}

func GetSecretsStringData(content *testkube.TestContent) map[string]string {
	// create secrets for test
	stringData := map[string]string{jobs.GitUsernameSecretName: "", jobs.GitTokenSecretName: ""}
	if content != nil && content.Repository != nil {
		stringData[jobs.GitUsernameSecretName] = content.Repository.Username
		stringData[jobs.GitTokenSecretName] = content.Repository.Token
	}

	return stringData
}
