package testkube

import (
	"fmt"
)

type Tests []Test

// Table implements interface for data rendering in CLI
func (tests Tests) Table() (header []string, output [][]string) {
	header = []string{"Name", "Description", "Steps"}
	for _, e := range tests {
		output = append(output, []string{
			e.Name,
			e.Description,
			fmt.Sprintf("%d", len(e.Steps)),
		})
	}

	return
}
