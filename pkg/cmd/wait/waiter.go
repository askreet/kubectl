package wait

import (
	"errors"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/util/jsonpath"
	"k8s.io/kubectl/pkg/cmd/get"
	"strings"
)

// A Waiter defines the behavior of waiting for the desired state, including configuration for the ResourceFinder used
// during the operation.
type Waiter struct {
	// ConditionFunc is called once for each resource provided by ResourceFinder during the wait loop.
	ConditionFn ConditionFunc

	// IgnoreErrorFns specifies which errors are ignored by ResourceFinder prior to starting a wait loop.
	IgnoreErrorFns []resource.ErrMatchFunc

	// If false, AllowNoResources will not immediately fail the wait loop if no resources match the query.
	AllowNoResources bool
}

// ConditionFunc is the interface for providing condition checks
type ConditionFunc func(info *resource.Info, o *WaitOptions) (finalObject runtime.Object, done bool, err error)

func waiterFor(condition string, errOut io.Writer) (*Waiter, error) {
	if strings.ToLower(condition) == "delete" {
		return NewDeletionWaiter(), nil
	}
	if strings.HasPrefix(condition, "condition=") {
		conditionName := condition[len("condition="):]
		conditionValue := "true"
		if equalsIndex := strings.Index(conditionName, "="); equalsIndex != -1 {
			conditionValue = conditionName[equalsIndex+1:]
			conditionName = conditionName[0:equalsIndex]
		}

		return NewConditionalWaiter(conditionName, conditionValue, errOut), nil
	}
	if strings.HasPrefix(condition, "jsonpath=") {
		splitStr := strings.Split(condition, "=")
		if len(splitStr) != 3 {
			return nil, fmt.Errorf("jsonpath wait format must be --for=jsonpath='{.status.readyReplicas}'=3")
		}
		jsonPathExp, jsonPathCond, err := processJSONPathInput(splitStr[1], splitStr[2])
		if err != nil {
			return nil, err
		}
		j, err := newJSONPathParser(jsonPathExp)
		if err != nil {
			return nil, err
		}
		return NewJSONPathWaiter(jsonPathCond, j, errOut), nil
	}

	return nil, fmt.Errorf("unrecognized condition: %q", condition)
}

// newJSONPathParser will create a new JSONPath parser based on the jsonPathExpression
func newJSONPathParser(jsonPathExpression string) (*jsonpath.JSONPath, error) {
	j := jsonpath.New("wait")
	if jsonPathExpression == "" {
		return nil, errors.New("jsonpath expression cannot be empty")
	}
	if err := j.Parse(jsonPathExpression); err != nil {
		return nil, err
	}
	return j, nil
}

// processJSONPathInput will parses the user's JSONPath input and process the string
func processJSONPathInput(jsonPathExpression, jsonPathCond string) (string, string, error) {
	relaxedJSONPathExp, err := get.RelaxedJSONPathExpression(jsonPathExpression)
	if err != nil {
		return "", "", err
	}
	if jsonPathCond == "" {
		return "", "", errors.New("jsonpath wait condition cannot be empty")
	}
	jsonPathCond = strings.Trim(jsonPathCond, `'"`)

	return relaxedJSONPathExp, jsonPathCond, nil
}
