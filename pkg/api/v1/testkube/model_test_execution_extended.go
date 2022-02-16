package testkube

import (
	"fmt"
	"time"

	"github.com/kubeshop/testkube/pkg/rand"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func NewStartedTestExecution(test Test, request TestExecutionRequest) TestExecution {
	testExecution := TestExecution{
		Id:        primitive.NewObjectID().Hex(),
		StartTime: time.Now(),
		Name:      fmt.Sprintf("%s.%s", test.Name, rand.Name()),
		Status:    TestStatusPending,
		Params:    request.Params,
		Test:      test.GetObjectRef(),
	}

	// add queued execution steps
	steps := append(test.Before, test.Steps...)
	steps = append(steps, test.After...)

	for i := range steps {
		testExecution.StepResults = append(testExecution.StepResults, NewTestStepQueuedResult(&steps[i]))
	}

	return testExecution
}

func (e TestExecution) IsCompleted() bool {
	return *e.Status == *TestStatusError || *e.Status == *TestStatusSuccess
}

func (e *TestExecution) CalculateDuration() time.Duration {

	end := e.EndTime
	start := e.StartTime

	if start.UnixNano() <= 0 && end.UnixNano() <= 0 {
		return time.Duration(0)
	}

	if end.UnixNano() <= 0 {
		end = time.Now()
	}

	return end.Sub(e.StartTime)
}

func (e TestExecution) Table() (header []string, output [][]string) {
	header = []string{"Status", "Step", "ID", "Error"}
	output = make([][]string, 0)

	for _, sr := range e.StepResults {
		status := "no-execution-result"
		if sr.Execution != nil && sr.Execution.ExecutionResult != nil && sr.Execution.ExecutionResult.Status != nil {
			status = string(*sr.Execution.ExecutionResult.Status)
		}

		switch sr.Step.Type() {
		case TestStepTypeExecuteScript:
			var id, errorMessage string
			if sr.Execution != nil && sr.Execution.ExecutionResult != nil {
				errorMessage = sr.Execution.ExecutionResult.ErrorMessage
				id = sr.Execution.Id
			}
			row := []string{status, sr.Step.FullName(), id, errorMessage}
			output = append(output, row)
		case TestStepTypeDelay:
			row := []string{status, sr.Step.FullName(), "", ""}
			output = append(output, row)
		}
	}

	return
}