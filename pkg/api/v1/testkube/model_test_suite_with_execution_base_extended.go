package testkube

import (
	"fmt"
)

type TestSuiteWithExecutions []TestSuiteWithExecution

func (testSutes TestSuiteWithExecutions) Table() (header []string, output [][]string) {
	header = []string{"Name", "Description", "Steps", "Labels", "Schedule", "Status", "Execution id"}
	for _, e := range testSutes {
		if e.TestSuite == nil {
			continue
		}

		status := ""
		executionID := ""
		if e.LatestExecution != nil {
			executionID = e.LatestExecution.Id
			if e.LatestExecution.Status != nil {
				status = string(*e.LatestExecution.Status)
			}
		}

		output = append(output, []string{
			e.TestSuite.Name,
			e.TestSuite.Description,
			fmt.Sprintf("%d", len(e.TestSuite.Steps)),
			LabelsToString(e.TestSuite.Labels),
			e.TestSuite.Schedule,
			status,
			executionID,
		})
	}

	return
}

func (t TestSuiteWithExecution) GetObjectRef() *ObjectRef {
	name := ""
	namespace := ""
	if t.TestSuite != nil {
		name = t.TestSuite.Name
		namespace = t.TestSuite.Namespace
	}

	return &ObjectRef{
		Name:      name,
		Namespace: namespace,
	}
}
