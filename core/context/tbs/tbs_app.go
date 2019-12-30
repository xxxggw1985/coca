package tbs

import (
	"github.com/phodal/coca/config"
	"github.com/phodal/coca/core/domain"
	"strings"
)

type TbsApp struct {
}

func NewTbsApp() *TbsApp {
	return &TbsApp{}
}

type TestBadSmell struct {
	FileName    string
	Type        string
	Description string
	Line        int
}

func (a TbsApp) AnalysisPath(deps []domain.JClassNode, identifiersMap map[string]domain.JIdentifier) []TestBadSmell {
	var results []TestBadSmell = nil
	callMethodMap := buildCallMethodMap(deps)
	for _, clz := range deps {
		for _, method := range clz.Methods {
			if !isTest(method) {
				continue
			}

			currentMethodCalls := updateMethodCallsForSelfCall(method, clz, callMethodMap)

			var testType = ""
			for _, annotation := range method.Annotations {
				checkIgnoreTest(clz.Path, annotation, &results, &testType)
				checkEmptyTest(clz.Path, annotation, &results, method, &testType)
			}

			var methodCallMap = make(map[string][]domain.JMethodCall)
			var hasAssert = false
			for index, methodCall := range currentMethodCalls {
				if methodCall.MethodName == "" {
					continue
				}

				methodCallMap[getMethodCallFullPath(methodCall)] = append(methodCallMap[getMethodCallFullPath(methodCall)], methodCall)

				checkRedundantPrintTest(clz.Path, methodCall, &results, &testType)
				checkSleepyTest(clz.Path, methodCall, method, &results, &testType)
				checkRedundantAssertionTest(clz.Path, methodCall, method, &results, &testType)

				if hasAssertion(methodCall.MethodName) {
					hasAssert = true
				}

				if index == len(currentMethodCalls)-1 {
					if !hasAssert {
						checkUnknownTest(clz, method, &results, &testType)
					}
				}
			}

			checkDuplicateAssertTest(clz, &results, methodCallMap, method, &testType)
		}
	}

	return results
}

func buildCallMethodMap(deps []domain.JClassNode) map[string]domain.JMethod {
	var callMethodMap = make(map[string]domain.JMethod)
	for _, clz := range deps {
		for _, method := range clz.Methods {
			callMethodMap[clz.Package+"."+clz.Class+"."+method.Name] = method
		}
	}
	return callMethodMap
}

func updateMethodCallsForSelfCall(method domain.JMethod, clz domain.JClassNode, callMethodMap map[string]domain.JMethod) []domain.JMethodCall {
	currentMethodCalls := method.MethodCalls
	for _, methodCall := range currentMethodCalls {
		if methodCall.Class == clz.Class {
			jMethod := callMethodMap[getMethodCallFullPath(methodCall)]
			if jMethod.Name != "" {
				currentMethodCalls = append(currentMethodCalls, jMethod.MethodCalls...)
			}
		}
	}
	return currentMethodCalls
}

func checkRedundantAssertionTest(path string, call domain.JMethodCall, method domain.JMethod, results *[]TestBadSmell, testType *string) {
	TWO_PARAMETERS := 2
	if len(call.Parameters) == TWO_PARAMETERS {
		if call.Parameters[0] == call.Parameters[1] {
			*testType = "RedundantAssertionTest"
			tbs := *&TestBadSmell{
				FileName:    path,
				Type:        *testType,
				Description: "",
				Line:        method.StartLine,
			}

			*results = append(*results, tbs)
		}
	}
}

func hasAssertion(methodName string) bool {
	methodName = strings.ToLower(methodName)
	for _, assertion := range config.ASSERTION_LIST {
		if strings.Contains(methodName, assertion) {
			return true
		}
	}

	return false
}

func isTest(method domain.JMethod) bool {
	var isTest = false
	for _, annotation := range method.Annotations {
		if annotation.QualifiedName == "Test" || annotation.QualifiedName == "Ignore" {
			isTest = true
		}
	}
	return isTest
}

func checkUnknownTest(clz domain.JClassNode, method domain.JMethod, results *[]TestBadSmell, testType *string) {
	*testType = "UnknownTest"
	tbs := *&TestBadSmell{
		FileName:    clz.Path,
		Type:        *testType,
		Description: "",
		Line:        method.StartLine,
	}

	*results = append(*results, tbs)
}

func checkDuplicateAssertTest(clz domain.JClassNode, results *[]TestBadSmell, methodCallMap map[string][]domain.JMethodCall, method domain.JMethod, testType *string) {
	var isDuplicateAssert = false
	for _, methodCall := range methodCallMap {
		if len(methodCall) >= config.DuplicatedAssertionLimitLength {
			methodName := methodCall[len(methodCall)-1].MethodName
			if hasAssertion(methodName) {
				isDuplicateAssert = true
			}
		}
	}

	if isDuplicateAssert {
		*testType = "DuplicateAssertTest"
		tbs := *&TestBadSmell{
			FileName:    clz.Path,
			Type:        *testType,
			Description: "",
			Line:        method.StartLine,
		}

		*results = append(*results, tbs)
	}
}

func getMethodCallFullPath(methodCall domain.JMethodCall) string {
	return methodCall.Package + "." + methodCall.Class + "." + methodCall.MethodName
}

func checkSleepyTest(path string, method domain.JMethodCall, jMethod domain.JMethod, results *[]TestBadSmell, testType *string) {
	if method.MethodName == "sleep" && method.Class == "Thread" {
		*testType = "SleepyTest"
		tbs := *&TestBadSmell{
			FileName:    path,
			Type:        *testType,
			Description: "",
			Line:        method.StartLine,
		}

		*results = append(*results, tbs)
	}
}

func checkRedundantPrintTest(path string, mCall domain.JMethodCall, results *[]TestBadSmell, testType *string) {
	if mCall.Class == "System.out" && (mCall.MethodName == "println" || mCall.MethodName == "printf" || mCall.MethodName == "print") {
		*testType = "RedundantPrintTest"
		tbs := *&TestBadSmell{
			FileName:    path,
			Type:        *testType,
			Description: "",
			Line:        mCall.StartLine,
		}

		*results = append(*results, tbs)
	}
}

func checkEmptyTest(path string, annotation domain.Annotation, results *[]TestBadSmell, method domain.JMethod, testType *string) {
	if annotation.QualifiedName == "Test" {
		if len(method.MethodCalls) <= 1 {
			*testType = "EmptyTest"
			tbs := *&TestBadSmell{
				FileName:    path,
				Type:        *testType,
				Description: "",
				Line:        method.StartLine,
			}

			*results = append(*results, tbs)
		}
	}
}

func checkIgnoreTest(clzPath string, annotation domain.Annotation, results *[]TestBadSmell, testType *string) {
	if annotation.QualifiedName == "Ignore" {
		*testType = "IgnoreTest"
		tbs := *&TestBadSmell{
			FileName:    clzPath,
			Type:        *testType,
			Description: "",
			Line:        0,
		}

		*results = append(*results, tbs)
	}
}