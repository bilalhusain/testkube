/*
 * TestKube API
 *
 * TestKube provides a Kubernetes-native framework for test definition, execution and results
 *
 * API version: 1.0.0
 * Contact: testkube@kubeshop.io
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package testkube

import (
	"time"
)

type Script struct {
	// script name
	Name string `json:"name,omitempty"`
	// script namespace
	Namespace string `json:"namespace,omitempty"`
	// script type
	Type_   string         `json:"type,omitempty"`
	Content *ScriptContent `json:"content,omitempty"`
	Created time.Time      `json:"created,omitempty"`
	// script tags
	Tags []string `json:"tags,omitempty"`
}